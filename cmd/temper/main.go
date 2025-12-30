package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/llm"
	mcpserver "github.com/felixgeelhaar/temper/internal/mcp"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/session"
)

const (
	daemonAddr = "http://127.0.0.1:7432"
	pidFile    = "temperd.pid"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit()
	case "start":
		err = cmdStart()
	case "stop":
		err = cmdStop()
	case "status":
		err = cmdStatus()
	case "logs":
		err = cmdLogs()
	case "doctor":
		err = cmdDoctor()
	case "config":
		err = cmdConfig()
	case "provider":
		err = cmdProvider(os.Args[2:])
	case "exercise":
		err = cmdExercise(os.Args[2:])
	case "spec":
		err = cmdSpec(os.Args[2:])
	case "stats":
		err = cmdStats(os.Args[2:])
	case "mcp":
		err = cmdMCP()
	case "help", "-h", "--help":
		printUsage()
	case "version", "-v", "--version":
		fmt.Println("temper v0.1.0")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Temper - Adaptive AI Pairing for Learning

Usage:
  temper <command> [arguments]

Setup Commands:
  init            Initialize Temper (first-time setup)
  doctor          Check system requirements
  config          Show current configuration
  provider        Manage LLM providers

Daemon Commands:
  start           Start the Temper daemon
  stop            Stop the Temper daemon
  status          Show daemon status
  logs            View daemon logs

Exercise Commands:
  exercise list   List available exercises
  exercise info   Show exercise details

Spec Commands (Specular format):
  spec create     Create a new spec scaffold
  spec list       List specs in workspace
  spec validate   Validate spec completeness
  spec status     Show spec progress
  spec lock       Generate SpecLock for drift detection

Analytics Commands:
  stats           Show learning statistics (overview)
  stats skills    Show skill progression by topic
  stats errors    Show common error patterns
  stats trend     Show hint dependency over time

Integration Commands:
  mcp             Start MCP server (for Cursor integration)

Other:
  help            Show this help message
  version         Show version information

Examples:
  temper start                    # Start daemon
  temper doctor                   # Check Docker, LLM providers
  temper provider set-key claude  # Configure Claude API key
  temper exercise list            # List exercises
  temper mcp                      # Start MCP server for Cursor`)
}

// cmdInit initializes Temper for first-time use
func cmdInit() error {
	fmt.Println("Temper - First-Time Setup")
	fmt.Println("==========================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// 1. Create directory structure
	fmt.Print("Creating ~/.temper directory structure... ")
	temperDir, err := config.EnsureTemperDir()
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	fmt.Println("✓")

	// 2. Create default config if it doesn't exist
	configPath := filepath.Join(temperDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Print("Creating default configuration... ")
		if err := config.SaveLocalConfig(config.DefaultLocalConfig()); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Println("✓")
	} else {
		fmt.Println("Configuration already exists ✓")
	}

	// 3. Copy bundled exercises
	fmt.Print("Setting up exercise packs... ")
	exercisesDest := filepath.Join(temperDir, "exercises")

	// Check if exercises exist in current directory (dev mode)
	if _, err := os.Stat("./exercises"); err == nil {
		// Copy from local
		if err := copyDir("./exercises", exercisesDest); err != nil {
			fmt.Println("⚠ (manual copy required)")
		} else {
			fmt.Println("✓")
		}
	} else {
		// Create placeholder
		goPackDir := filepath.Join(exercisesDest, "go-fundamentals")
		if err := os.MkdirAll(goPackDir, 0755); err == nil {
			fmt.Println("✓ (placeholder created)")
		}
	}

	// 4. Configure LLM provider
	fmt.Println()
	fmt.Println("LLM Provider Setup")
	fmt.Println("------------------")
	fmt.Println("Temper supports: Claude (Anthropic), OpenAI, and Ollama (local)")
	fmt.Println()

	// Load current config to check existing keys
	cfg, _ := config.LoadLocalConfig()

	// Claude
	if cfg != nil && cfg.LLM.Providers["claude"] != nil && cfg.LLM.Providers["claude"].APIKey != "" {
		fmt.Println("Claude API key: already configured ✓")
	} else {
		fmt.Print("Enter Claude API key (or press Enter to skip): ")
		key, _ := reader.ReadString('\n')
		key = strings.TrimSpace(key)
		if key != "" {
			secrets := map[string]string{"claude": key}
			if err := config.SaveSecrets(secrets); err != nil {
				fmt.Printf("  ⚠ Failed to save: %v\n", err)
			} else {
				fmt.Println("  ✓ Saved")
			}
		}
	}

	// 5. Check Docker
	fmt.Println()
	fmt.Print("Checking Docker... ")
	if err := checkDocker(); err != nil {
		fmt.Println("⚠ Not available (local execution will be used)")
	} else {
		fmt.Println("✓")
	}

	// 6. Summary
	fmt.Println()
	fmt.Println("Setup Complete!")
	fmt.Println("===============")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. temper start          # Start the daemon")
	fmt.Println("  2. temper doctor         # Verify configuration")
	fmt.Println("  3. temper exercise list  # See available exercises")
	fmt.Println()
	fmt.Println("For IDE integration:")
	fmt.Println("  - VS Code: Install the Temper extension from editors/vscode/")
	fmt.Println("  - Neovim:  Add the plugin from editors/nvim/")
	fmt.Println("  - Cursor:  Configure MCP with 'temper mcp' command")

	return nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

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

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

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
	file.Seek(offset, 0)

	// Skip partial first line if we seeked
	if offset > 0 {
		reader := bufio.NewReader(file)
		reader.ReadString('\n')
	}

	// Print remaining lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	return nil
}

// cmdDoctor checks system requirements
func cmdDoctor() error {
	fmt.Println("Checking system requirements...")

	allGood := true

	// Check Docker
	fmt.Print("Docker:    ")
	if err := checkDocker(); err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		fmt.Println("✓ available")
	}

	// Check temper directory
	fmt.Print("Directory: ")
	temperDir, err := config.TemperDir()
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else if _, err := os.Stat(temperDir); os.IsNotExist(err) {
		fmt.Println("✗ not created (run 'temper start' to create)")
		allGood = false
	} else {
		fmt.Printf("✓ %s\n", temperDir)
	}

	// Check config
	fmt.Print("Config:    ")
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		fmt.Println("✓ loaded")

		// Check LLM providers
		fmt.Println("\nLLM Providers:")
		for name, provider := range cfg.LLM.Providers {
			if !provider.Enabled {
				continue
			}

			fmt.Printf("  %s: ", name)
			if name == "ollama" {
				// Check Ollama connectivity
				if err := checkOllama(provider.URL); err != nil {
					fmt.Printf("✗ %v\n", err)
				} else {
					fmt.Printf("✓ available (model: %s)\n", provider.Model)
				}
			} else if provider.APIKey != "" {
				fmt.Printf("✓ configured (model: %s)\n", provider.Model)
			} else {
				fmt.Printf("✗ no API key (run 'temper provider set-key %s')\n", name)
			}
		}
	}

	// Check daemon status
	fmt.Print("\nDaemon:    ")
	if isRunning() {
		fmt.Println("✓ running")
	} else {
		fmt.Println("✗ not running (run 'temper start')")
	}

	fmt.Println()
	if allGood {
		fmt.Println("All checks passed! ✓")
	} else {
		fmt.Println("Some checks failed. Please fix the issues above.")
	}

	return nil
}

// cmdConfig shows current configuration
func cmdConfig() error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("Temper Configuration")

	fmt.Println("Daemon:")
	fmt.Printf("  bind: %s:%d\n", cfg.Daemon.Bind, cfg.Daemon.Port)
	fmt.Printf("  log_level: %s\n", cfg.Daemon.LogLevel)

	fmt.Println("\nLLM:")
	fmt.Printf("  default_provider: %s\n", cfg.LLM.DefaultProvider)
	for name, provider := range cfg.LLM.Providers {
		if provider.Enabled {
			hasKey := provider.APIKey != "" || name == "ollama"
			keyStatus := "✗"
			if hasKey {
				keyStatus = "✓"
			}
			fmt.Printf("  %s: enabled=%t model=%s key=%s\n", name, provider.Enabled, provider.Model, keyStatus)
		}
	}

	fmt.Println("\nLearning Contract:")
	fmt.Printf("  default_track: %s\n", cfg.Learning.DefaultTrack)
	for name, track := range cfg.Learning.Tracks {
		fmt.Printf("  %s: max_level=L%d cooldown=%ds\n", name, track.MaxLevel, track.CooldownSeconds)
	}

	fmt.Println("\nRunner:")
	fmt.Printf("  executor: %s\n", cfg.Runner.Executor)
	if cfg.Runner.Executor == "docker" {
		fmt.Printf("  image: %s\n", cfg.Runner.Docker.Image)
		fmt.Printf("  memory: %dMB\n", cfg.Runner.Docker.MemoryMB)
		fmt.Printf("  timeout: %ds\n", cfg.Runner.Docker.TimeoutSeconds)
	}

	temperDir, _ := config.TemperDir()
	fmt.Printf("\nConfig path: %s/config.yaml\n", temperDir)

	return nil
}

// cmdProvider manages LLM provider API keys
func cmdProvider(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Provider management commands:

  temper provider list              List configured providers
  temper provider set-key <name>    Set API key for a provider`)
		return nil
	}

	switch args[0] {
	case "list":
		return cmdProviderList()
	case "set-key":
		if len(args) < 2 {
			return fmt.Errorf("provider name required")
		}
		return cmdProviderSetKey(args[1])
	default:
		return fmt.Errorf("unknown provider command: %s", args[0])
	}
}

func cmdProviderList() error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("Configured LLM Providers:")
	for name, provider := range cfg.LLM.Providers {
		status := "disabled"
		if provider.Enabled {
			if provider.APIKey != "" || name == "ollama" {
				status = "ready"
			} else {
				status = "needs API key"
			}
		}

		isDefault := ""
		if name == cfg.LLM.DefaultProvider {
			isDefault = " (default)"
		}

		fmt.Printf("  %s%s\n", name, isDefault)
		fmt.Printf("    status: %s\n", status)
		fmt.Printf("    model:  %s\n", provider.Model)
		if name == "ollama" && provider.URL != "" {
			fmt.Printf("    url:    %s\n", provider.URL)
		}
		fmt.Println()
	}

	return nil
}

func cmdProviderSetKey(provider string) error {
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if provider exists
	if _, ok := cfg.LLM.Providers[provider]; !ok {
		return fmt.Errorf("unknown provider: %s (valid: claude, openai, ollama)", provider)
	}

	if provider == "ollama" {
		fmt.Println("Ollama doesn't require an API key.")
		return nil
	}

	// Prompt for API key
	fmt.Printf("Enter %s API key: ", provider)
	reader := bufio.NewReader(os.Stdin)
	key, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	key = strings.TrimSpace(key)

	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Load existing secrets and update
	secrets := make(map[string]string)
	secrets[provider] = key

	if err := config.SaveSecrets(secrets); err != nil {
		return fmt.Errorf("save secrets: %w", err)
	}

	fmt.Printf("✓ API key saved for %s\n", provider)
	fmt.Println("Restart the daemon for changes to take effect.")
	return nil
}

// cmdExercise manages exercises
func cmdExercise(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Exercise commands:

  temper exercise list              List all exercise packs
  temper exercise info <pack/slug>  Show exercise details`)
		return nil
	}

	switch args[0] {
	case "list":
		return cmdExerciseList()
	case "info":
		if len(args) < 2 {
			return fmt.Errorf("exercise ID required (e.g., go-v1/hello-world)")
		}
		return cmdExerciseInfo(args[1])
	default:
		return fmt.Errorf("unknown exercise command: %s", args[0])
	}
}

func cmdExerciseList() error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	resp, err := http.Get(daemonAddr + "/v1/exercises")
	if err != nil {
		return fmt.Errorf("get exercises: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Packs []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			Language      string `json:"language"`
			ExerciseCount int    `json:"exercise_count"`
		} `json:"packs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Available Exercise Packs:")
	for _, pack := range result.Packs {
		fmt.Printf("  %s (%s)\n", pack.Name, pack.ID)
		fmt.Printf("    %s\n", pack.Description)
		fmt.Printf("    Language: %s | Exercises: %d\n\n", pack.Language, pack.ExerciseCount)
	}

	fmt.Println("Use 'temper exercise info <pack>/<slug>' for details")
	return nil
}

func cmdExerciseInfo(id string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return fmt.Errorf("exercise ID must be in format: pack/category/slug (e.g., go-v1/basics/hello-world)")
	}

	// Build URL: /v1/exercises/pack/category/slug
	url := fmt.Sprintf("%s/v1/exercises/%s", daemonAddr, id)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get exercise: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("exercise not found: %s", id)
	}

	var exercise struct {
		ID          string   `json:"ID"`
		Title       string   `json:"Title"`
		Description string   `json:"Description"`
		Difficulty  string   `json:"Difficulty"`
		Tags        []string `json:"Tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&exercise); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("Exercise: %s\n\n", exercise.Title)
	fmt.Printf("ID:         %s\n", exercise.ID)
	fmt.Printf("Difficulty: %s\n", exercise.Difficulty)
	fmt.Printf("Tags:       %s\n", strings.Join(exercise.Tags, ", "))
	fmt.Printf("\nDescription:\n%s\n", exercise.Description)

	return nil
}

// cmdSpec manages product specifications (Specular format)
func cmdSpec(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Spec commands (Specular format):

  temper spec create <name>        Create a new spec scaffold
  temper spec list                 List specs in workspace
  temper spec validate <path>      Validate spec completeness
  temper spec status <path>        Show spec progress
  temper spec lock <path>          Generate SpecLock for drift detection
  temper spec drift <path>         Show drift from locked spec

Examples:
  temper spec create "User Authentication"
  temper spec validate .specs/auth.yaml
  temper spec status .specs/auth.yaml`)
		return nil
	}

	switch args[0] {
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("spec name required (e.g., temper spec create \"User Authentication\")")
		}
		return cmdSpecCreate(args[1])
	case "list":
		return cmdSpecList()
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec validate .specs/auth.yaml)")
		}
		return cmdSpecValidate(args[1])
	case "status":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec status .specs/auth.yaml)")
		}
		return cmdSpecStatus(args[1])
	case "lock":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec lock .specs/auth.yaml)")
		}
		return cmdSpecLock(args[1])
	case "drift":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec drift .specs/auth.yaml)")
		}
		return cmdSpecDrift(args[1])
	default:
		return fmt.Errorf("unknown spec command: %s", args[0])
	}
}

func cmdSpecCreate(name string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	body := fmt.Sprintf(`{"name": %q}`, name)
	resp, err := http.Post(daemonAddr+"/v1/specs", "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("create spec failed: %s", errResp.Error)
	}

	var spec struct {
		Name     string `json:"name"`
		FilePath string `json:"file_path"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("✓ Created spec: %s\n", spec.Name)
	fmt.Printf("  File: .specs/%s\n", spec.FilePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the spec file to define your features and acceptance criteria")
	fmt.Println("  2. Run 'temper spec validate .specs/" + spec.FilePath + "' to check completeness")
	fmt.Println("  3. Start a feature guidance session with 'temper session --spec .specs/" + spec.FilePath + "'")

	return nil
}

func cmdSpecList() error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	resp, err := http.Get(daemonAddr + "/v1/specs")
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Specs []struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			FilePath string `json:"file_path"`
			Progress struct {
				Satisfied int     `json:"satisfied"`
				Total     int     `json:"total"`
				Percent   float64 `json:"percent"`
			} `json:"progress"`
		} `json:"specs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if len(result.Specs) == 0 {
		fmt.Println("No specs found in workspace.")
		fmt.Println("Create one with: temper spec create \"Feature Name\"")
		return nil
	}

	fmt.Println("Product Specifications")
	fmt.Println("======================")
	for _, spec := range result.Specs {
		bar := renderProgressBar(spec.Progress.Percent/100, 20)
		fmt.Printf("\n%s (v%s)\n", spec.Name, spec.Version)
		fmt.Printf("  File:     .specs/%s\n", spec.FilePath)
		fmt.Printf("  Progress: %s %d/%d (%.0f%%)\n",
			bar, spec.Progress.Satisfied, spec.Progress.Total, spec.Progress.Percent)
	}

	return nil
}

func cmdSpecValidate(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/validate", daemonAddr, path)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("validate spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	var validation struct {
		Valid    bool     `json:"valid"`
		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&validation); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if validation.Valid {
		fmt.Println("✓ Spec is valid")
	} else {
		fmt.Println("✗ Spec validation failed")
	}

	if len(validation.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range validation.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
	}

	if len(validation.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range validation.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	return nil
}

func cmdSpecStatus(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	// Get spec details
	url := fmt.Sprintf("%s/v1/specs/%s", daemonAddr, path)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	var spec struct {
		Name               string `json:"name"`
		Version            string `json:"version"`
		Goals              []string `json:"goals"`
		AcceptanceCriteria []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Satisfied   bool   `json:"satisfied"`
			Evidence    string `json:"evidence,omitempty"`
		} `json:"acceptance_criteria"`
		Features []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Priority string `json:"priority"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("%s (v%s)\n", spec.Name, spec.Version)
	fmt.Println(strings.Repeat("=", len(spec.Name)+len(spec.Version)+4))

	// Goals
	fmt.Println("\nGoals:")
	for _, goal := range spec.Goals {
		fmt.Printf("  • %s\n", goal)
	}

	// Features
	fmt.Println("\nFeatures:")
	for _, feat := range spec.Features {
		fmt.Printf("  [%s] %s (%s)\n", feat.ID, feat.Title, feat.Priority)
	}

	// Acceptance Criteria
	fmt.Println("\nAcceptance Criteria:")
	satisfied := 0
	for _, ac := range spec.AcceptanceCriteria {
		status := "⏳"
		if ac.Satisfied {
			status = "✓"
			satisfied++
		}
		fmt.Printf("  %s [%s] %s\n", status, ac.ID, ac.Description)
		if ac.Evidence != "" {
			fmt.Printf("      Evidence: %s\n", ac.Evidence)
		}
	}

	// Progress summary
	total := len(spec.AcceptanceCriteria)
	percent := 0.0
	if total > 0 {
		percent = float64(satisfied) / float64(total) * 100
	}
	bar := renderProgressBar(percent/100, 30)
	fmt.Printf("\nProgress: %s %d/%d (%.0f%%)\n", bar, satisfied, total, percent)

	return nil
}

func cmdSpecLock(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/lock", daemonAddr, path)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("lock spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return fmt.Errorf("spec must be valid before locking. Run 'temper spec validate %s' first", path)
	}

	var lock struct {
		Version  string `json:"version"`
		SpecHash string `json:"spec_hash"`
		LockedAt string `json:"locked_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&lock); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("✓ Spec locked successfully")
	fmt.Printf("  Version:  %s\n", lock.Version)
	fmt.Printf("  Hash:     %s\n", lock.SpecHash[:16]+"...")
	fmt.Printf("  Locked:   %s\n", lock.LockedAt)
	fmt.Println()
	fmt.Println("The lock file has been saved to .specs/spec.lock")
	fmt.Println("Use 'temper spec drift' to detect changes from this baseline.")

	return nil
}

func cmdSpecDrift(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/drift", daemonAddr, path)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get drift: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec or lock not found: %s", path)
	}

	var drift struct {
		HasDrift         bool     `json:"has_drift"`
		VersionChanged   bool     `json:"version_changed"`
		OldVersion       string   `json:"old_version,omitempty"`
		NewVersion       string   `json:"new_version,omitempty"`
		AddedFeatures    []string `json:"added_features"`
		RemovedFeatures  []string `json:"removed_features"`
		ModifiedFeatures []string `json:"modified_features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&drift); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if !drift.HasDrift {
		fmt.Println("✓ No drift detected - spec matches lock")
		return nil
	}

	fmt.Println("⚠ Drift detected from locked spec")
	fmt.Println()

	if drift.VersionChanged {
		fmt.Printf("Version: %s → %s\n", drift.OldVersion, drift.NewVersion)
	}

	if len(drift.AddedFeatures) > 0 {
		fmt.Println("\nAdded features:")
		for _, f := range drift.AddedFeatures {
			fmt.Printf("  + %s\n", f)
		}
	}

	if len(drift.RemovedFeatures) > 0 {
		fmt.Println("\nRemoved features:")
		for _, f := range drift.RemovedFeatures {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(drift.ModifiedFeatures) > 0 {
		fmt.Println("\nModified features:")
		for _, f := range drift.ModifiedFeatures {
			fmt.Printf("  ~ %s\n", f)
		}
	}

	fmt.Println()
	fmt.Println("Run 'temper spec lock' to create a new baseline.")

	return nil
}

// cmdStats shows learning statistics
func cmdStats(args []string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	subCmd := "overview"
	if len(args) > 0 {
		subCmd = args[0]
	}

	switch subCmd {
	case "overview", "":
		return cmdStatsOverview()
	case "skills":
		return cmdStatsSkills()
	case "errors":
		return cmdStatsErrors()
	case "trend":
		return cmdStatsTrend()
	default:
		return fmt.Errorf("unknown stats command: %s (valid: overview, skills, errors, trend)", subCmd)
	}
}

func cmdStatsOverview() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/overview")
	if err != nil {
		return fmt.Errorf("get overview: %w", err)
	}
	defer resp.Body.Close()

	var overview struct {
		TotalSessions       int     `json:"total_sessions"`
		CompletedSessions   int     `json:"completed_sessions"`
		TotalRuns           int     `json:"total_runs"`
		TotalHints          int     `json:"total_hints"`
		TotalExercises      int     `json:"total_exercises"`
		HintDependency      float64 `json:"hint_dependency"`
		AvgTimeToGreen      string  `json:"avg_time_to_green"`
		CompletionRate      float64 `json:"completion_rate"`
		MostPracticedTopics []struct {
			Topic    string  `json:"topic"`
			Attempts int     `json:"attempts"`
			Level    float64 `json:"level"`
			Trend    string  `json:"trend"`
		} `json:"most_practiced_topics"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Learning Statistics")
	fmt.Println("==================")
	fmt.Printf("Total Sessions:     %d\n", overview.TotalSessions)
	fmt.Printf("Completed:          %d (%.1f%%)\n", overview.CompletedSessions, overview.CompletionRate*100)
	fmt.Printf("Total Exercises:    %d\n", overview.TotalExercises)
	fmt.Printf("Total Runs:         %d\n", overview.TotalRuns)
	fmt.Printf("Total Hints:        %d\n", overview.TotalHints)
	fmt.Printf("Hint Dependency:    %.1f%%\n", overview.HintDependency*100)
	fmt.Printf("Avg Time to Green:  %s\n", overview.AvgTimeToGreen)

	if len(overview.MostPracticedTopics) > 0 {
		fmt.Println("\nMost Practiced Topics")
		fmt.Println("---------------------")
		for _, topic := range overview.MostPracticedTopics {
			bar := renderProgressBar(topic.Level, 20)
			fmt.Printf("%-20s %s %.0f%% (%d attempts) %s\n",
				topic.Topic, bar, topic.Level*100, topic.Attempts, topic.Trend)
		}
	}

	return nil
}

func cmdStatsSkills() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/skills")
	if err != nil {
		return fmt.Errorf("get skills: %w", err)
	}
	defer resp.Body.Close()

	var breakdown struct {
		Skills map[string]struct {
			Topic      string  `json:"topic"`
			Level      float64 `json:"level"`
			Attempts   int     `json:"attempts"`
			Trend      string  `json:"trend"`
			Confidence float64 `json:"confidence"`
		} `json:"skills"`
		Progression []struct {
			Date         string  `json:"date"`
			AvgSkill     float64 `json:"avg_skill"`
			TopicsActive int     `json:"topics_active"`
		} `json:"progression"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&breakdown); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Skills by Topic")
	fmt.Println("===============")

	if len(breakdown.Skills) == 0 {
		fmt.Println("No skills tracked yet. Start practicing!")
		return nil
	}

	for topic, skill := range breakdown.Skills {
		bar := renderProgressBar(skill.Level, 20)
		fmt.Printf("%-20s %s %.0f%% (%d attempts) %s\n",
			topic, bar, skill.Level*100, skill.Attempts, skill.Trend)
	}

	if len(breakdown.Progression) > 0 {
		fmt.Println("\nProgression (Last 30 days)")
		fmt.Println("--------------------------")
		for _, point := range breakdown.Progression {
			miniBar := renderProgressBar(point.AvgSkill, 10)
			fmt.Printf("%s: %s %.0f%% (%d topics)\n",
				point.Date, miniBar, point.AvgSkill*100, point.TopicsActive)
		}
	}

	return nil
}

func cmdStatsErrors() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/errors")
	if err != nil {
		return fmt.Errorf("get errors: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Patterns []struct {
			Pattern  string `json:"pattern"`
			Count    int    `json:"count"`
			Category string `json:"category"`
		} `json:"patterns"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Common Error Patterns")
	fmt.Println("====================")

	if len(result.Patterns) == 0 {
		fmt.Println("No errors tracked yet. Keep coding!")
		return nil
	}

	for _, pattern := range result.Patterns {
		fmt.Printf("  [%s] %s (%d occurrences)\n",
			pattern.Category, pattern.Pattern, pattern.Count)
	}

	return nil
}

func cmdStatsTrend() error {
	resp, err := http.Get(daemonAddr + "/v1/analytics/trend")
	if err != nil {
		return fmt.Errorf("get trend: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Trend []struct {
			Timestamp  string  `json:"timestamp"`
			Dependency float64 `json:"dependency"`
			RunWindow  int     `json:"run_window"`
		} `json:"trend"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Hint Dependency Trend")
	fmt.Println("====================")

	if len(result.Trend) == 0 {
		fmt.Println("Not enough data yet. Keep practicing!")
		return nil
	}

	// Show last 10 data points
	start := 0
	if len(result.Trend) > 10 {
		start = len(result.Trend) - 10
	}

	for _, point := range result.Trend[start:] {
		bar := renderProgressBar(point.Dependency, 20)
		fmt.Printf("%s: %s %.1f%%\n",
			point.Timestamp[:10], bar, point.Dependency*100)
	}

	// Calculate trend direction
	if len(result.Trend) >= 2 {
		first := result.Trend[0].Dependency
		last := result.Trend[len(result.Trend)-1].Dependency
		if last < first-0.05 {
			fmt.Println("\n↓ Your hint dependency is decreasing - great progress!")
		} else if last > first+0.05 {
			fmt.Println("\n↑ Your hint dependency is increasing - try solving more on your own")
		} else {
			fmt.Println("\n→ Your hint dependency is stable")
		}
	}

	return nil
}

// renderProgressBar creates a visual progress bar
func renderProgressBar(value float64, width int) string {
	filled := int(value * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

// cmdMCP starts the MCP server for Cursor integration
func cmdMCP() error {
	// Load configuration
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize LLM registry
	registry := llm.NewRegistry()

	// Setup LLM providers
	for name, providerCfg := range cfg.LLM.Providers {
		if !providerCfg.Enabled || (providerCfg.APIKey == "" && name != "ollama") {
			continue
		}

		switch name {
		case "claude":
			provider := llm.NewClaudeProvider(llm.ClaudeConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("claude", provider)
		case "openai":
			provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
				APIKey: providerCfg.APIKey,
				Model:  providerCfg.Model,
			})
			registry.Register("openai", provider)
		case "ollama":
			provider := llm.NewOllamaProvider(llm.OllamaConfig{
				BaseURL: providerCfg.URL,
				Model:   providerCfg.Model,
			})
			registry.Register("ollama", provider)
		}
	}

	// Initialize exercise loader
	temperDir, err := config.TemperDir()
	if err != nil {
		return fmt.Errorf("get temper dir: %w", err)
	}
	exercisePath := filepath.Join(temperDir, "exercises")
	loader := exercise.NewLoader(exercisePath)

	// Initialize runner (local for MCP - Docker might not be available)
	executor := runner.NewLocalExecutor("")

	// Initialize session store
	sessionsPath := filepath.Join(temperDir, "sessions")
	sessionStore, err := session.NewStore(sessionsPath)
	if err != nil {
		return fmt.Errorf("create session store: %w", err)
	}

	// Create services
	sessionService := session.NewService(sessionStore, loader, executor)
	pairingService := pairing.NewService(registry, cfg.LLM.DefaultProvider)

	// Create MCP server
	mcpSrv := mcpserver.NewServer(mcpserver.Config{
		SessionService: sessionService,
		PairingService: pairingService,
		ExerciseLoader: loader,
	})

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Serve on stdio
	return mcpSrv.ServeStdio(ctx)
}

// Helper functions

func isRunning() bool {
	resp, err := http.Get(daemonAddr + "/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

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

func checkDocker() error {
	// Check if docker is in PATH
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH")
	}

	// Check if docker daemon is running
	cmd := exec.Command("docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon not running")
	}

	return nil
}

func checkOllama(url string) error {
	if url == "" {
		url = "http://localhost:11434"
	}

	resp, err := http.Get(url + "/api/tags")
	if err != nil {
		return fmt.Errorf("not reachable at %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
