package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
)

// cmdStart starts the daemon in the background
func cmdStart() error {
	// Check if already running
	if isRunning() {
		fmt.Println("✓ Daemon is already running")
		return nil
	}

	// Ensure temper dir exists
	temperDir, err := config.EnsureTemperDir()
	if err != nil {
		return fmt.Errorf("setup temper directory: %w", err)
	}

	// Find temperd binary
	temperdPath, err := findDaemonBinary()
	if err != nil {
		return fmt.Errorf("find daemon binary: %w", err)
	}

	// Start daemon in background
	cmd := exec.Command(temperdPath)
	cmd.Dir = temperDir
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach from parent process (platform-specific)
	configureDaemonProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Wait for daemon to be ready
	fmt.Print("Starting daemon...")
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if isRunning() {
			fmt.Println(" ✓")
			fmt.Printf("Daemon running at %s\n", daemonAddr)
			return nil
		}
		fmt.Print(".")
	}

	fmt.Println(" ✗")
	return fmt.Errorf("daemon failed to start (check logs with 'temper logs')")
}

// cmdStop stops the daemon
func cmdStop() error {
	if !isRunning() {
		fmt.Println("Daemon is not running")
		return nil
	}

	// Read PID file
	temperDir, err := config.TemperDir()
	if err != nil {
		return err
	}

	pidPath := filepath.Join(temperDir, pidFile)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("parse PID: %w", err)
	}

	// Send SIGTERM
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	fmt.Print("Stopping daemon...")
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send signal: %w", err)
	}

	// Wait for daemon to stop
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isRunning() {
			fmt.Println(" ✓")
			return nil
		}
		fmt.Print(".")
	}

	fmt.Println(" ✗")
	return fmt.Errorf("daemon did not stop gracefully")
}

// cmdStatus shows daemon status
func cmdStatus() error {
	if !isRunning() {
		fmt.Println("Status: stopped")
		return nil
	}

	// Get status from daemon
	resp, err := http.Get(daemonAddr + "/v1/status")
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	defer resp.Body.Close()

	var status struct {
		Status       string   `json:"status"`
		Version      string   `json:"version"`
		LLMProviders []string `json:"llm_providers"`
		Runner       string   `json:"runner"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("parse status: %w", err)
	}

	fmt.Printf("Status:    %s\n", status.Status)
	fmt.Printf("Version:   %s\n", status.Version)
	fmt.Printf("Runner:    %s\n", status.Runner)
	fmt.Printf("Providers: %s\n", strings.Join(status.LLMProviders, ", "))
	fmt.Printf("Address:   %s\n", daemonAddr)

	return nil
}

// cmdLogs shows daemon logs
func cmdLogs() error {
	temperDir, err := config.TemperDir()
	if err != nil {
		return err
	}

	logPath := filepath.Join(temperDir, "logs", "temperd.log")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Println("No log file found. Start the daemon first.")
		return nil
	}

	// Open and tail log file
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	// Seek to end and go back ~4KB for recent logs
	info, _ := file.Stat()
	offset := info.Size() - 4096
	if offset < 0 {
		offset = 0
	}
	_, _ = file.Seek(offset, 0)

	// Skip partial first line if we seeked
	if offset > 0 {
		reader := bufio.NewReader(file)
		_, _ = reader.ReadString('\n')
	}

	// Print remaining lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return nil
}

// isRunning checks if the daemon is running by calling the health endpoint
func isRunning() bool {
	resp, err := http.Get(daemonAddr + "/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// findDaemonBinary locates the temperd binary
func findDaemonBinary() (string, error) {
	// Check if temperd is in PATH
	if path, err := exec.LookPath("temperd"); err == nil {
		return path, nil
	}

	// Check relative to this binary
	self, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(self)
		path := filepath.Join(dir, "temperd")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Check common locations
	locations := []string{
		"/usr/local/bin/temperd",
		"./temperd",
		"./cmd/temperd/temperd",
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("temperd binary not found (build with 'go build ./cmd/temperd')")
}
