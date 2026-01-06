package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// Executor defines the interface for code execution
type Executor interface {
	// RunFormat runs gofmt and returns diff
	RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error)

	// RunFormatFix runs gofmt and returns formatted code
	RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error)

	// RunBuild runs go build
	RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error)

	// RunTests runs go test with JSON output
	RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error)
}

// FormatResult contains the result of gofmt
type FormatResult struct {
	OK   bool
	Diff string
}

// BuildResult contains the result of go build
type BuildResult struct {
	OK     bool
	Output string
}

// TestResult contains the result of go test
type TestResult struct {
	OK       bool
	Output   string
	Duration time.Duration
}

// LocalExecutor executes code locally (for development)
type LocalExecutor struct {
	workDir string
}

// NewLocalExecutor creates a new local executor
func NewLocalExecutor(workDir string) *LocalExecutor {
	return &LocalExecutor{workDir: workDir}
}

func (e *LocalExecutor) RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Run gofmt -d on all .go files
	var allDiffs strings.Builder
	for filename := range code {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		cmd := exec.CommandContext(ctx, "gofmt", "-d", filepath.Join(tmpDir, filename))
		output, _ := cmd.CombinedOutput()
		if len(output) > 0 {
			allDiffs.Write(output)
		}
	}

	diff := allDiffs.String()
	return &FormatResult{
		OK:   diff == "",
		Diff: diff,
	}, nil
}

func (e *LocalExecutor) RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	// Create a copy of the code map for the result
	result := make(map[string]string)
	for filename, content := range code {
		result[filename] = content
	}

	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Run gofmt on each .go file and read the formatted output
	for filename := range code {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		filePath := filepath.Join(tmpDir, filename)

		// Run gofmt and get formatted output
		cmd := exec.CommandContext(ctx, "gofmt", filePath)
		output, err := cmd.Output()
		if err != nil {
			// If gofmt fails (syntax error), keep original
			continue
		}
		result[filename] = string(output)
	}

	return result, nil
}

func (e *LocalExecutor) RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Initialize go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := "module exercise\n\ngo 1.22\n"
		_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

	// Run go build
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &BuildResult{
			OK:     false,
			Output: string(output),
		}, nil
	}

	return &BuildResult{
		OK:     true,
		Output: string(output),
	}, nil
}

func (e *LocalExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	// Create temp directory for code
	tmpDir, err := createTempCodeDir(code)
	if err != nil {
		return nil, err
	}
	defer removeTempDir(tmpDir)

	// Initialize go.mod if not present
	if _, ok := code["go.mod"]; !ok {
		modContent := "module exercise\n\ngo 1.22\n"
		_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644)
	}

	// Run go test with JSON output
	start := time.Now()
	args := append([]string{"test", "-json", "./..."}, flags...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	duration := time.Since(start)

	return &TestResult{
		OK:       cmd.ProcessState.ExitCode() == 0,
		Output:   string(output),
		Duration: duration,
	}, nil
}

// Helper functions
func createTempCodeDir(code map[string]string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "temper-run-*")
	if err != nil {
		return "", err
	}

	for filename, content := range code {
		cleaned, err := sanitizeRelativePath(filename)
		if err != nil {
			removeTempDir(tmpDir)
			return "", fmt.Errorf("invalid filename %q: %w", filename, err)
		}
		filePath := filepath.Join(tmpDir, cleaned)
		// Create parent directories if needed
		if dir := filepath.Dir(filePath); dir != tmpDir {
			_ = os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			removeTempDir(tmpDir)
			return "", err
		}
	}

	return tmpDir, nil
}

func removeTempDir(dir string) {
	os.RemoveAll(dir)
}

func sanitizeRelativePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("path must be a file path")
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	sep := string(filepath.Separator)
	if strings.HasPrefix(cleaned, ".."+sep) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return cleaned, nil
}

// DockerExecutor executes code in Docker containers
type DockerExecutor struct {
	client     *client.Client
	baseImage  string
	memoryMB   int64
	cpuLimit   float64
	networkOff bool
	timeout    time.Duration
}

// DockerConfig holds Docker executor configuration
type DockerConfig struct {
	BaseImage  string
	MemoryMB   int64
	CPULimit   float64
	NetworkOff bool
	Timeout    time.Duration
}

// DefaultDockerConfig returns sensible defaults for Docker execution
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		BaseImage:  "golang:1.23-alpine",
		MemoryMB:   256,
		CPULimit:   0.5,
		NetworkOff: true,
		Timeout:    120 * time.Second,
	}
}

// NewDockerExecutor creates a new Docker executor
func NewDockerExecutor(cfg DockerConfig) (*DockerExecutor, error) {
	if cfg.BaseImage == "" {
		cfg.BaseImage = "golang:1.23-alpine"
	}
	if cfg.MemoryMB == 0 {
		cfg.MemoryMB = 256
	}
	if cfg.CPULimit == 0 {
		cfg.CPULimit = 0.5
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	// Try to create client with environment settings first
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Try Docker Desktop socket paths if the default doesn't work
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		// Try common Docker Desktop socket locations
		socketPaths := []string{
			os.Getenv("HOME") + "/.docker/run/docker.sock",
			"/var/run/docker.sock",
		}

		for _, socketPath := range socketPaths {
			if _, statErr := os.Stat(socketPath); statErr == nil {
				cli.Close()
				cli, err = client.NewClientWithOpts(
					client.WithHost("unix://"+socketPath),
					client.WithAPIVersionNegotiation(),
				)
				if err != nil {
					continue
				}
				ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
				_, pingErr := cli.Ping(ctx2)
				cancel2()
				if pingErr == nil {
					slog.Info("connected to Docker daemon", "socket", socketPath)
					break
				}
			}
		}
	}

	return &DockerExecutor{
		client:     cli,
		baseImage:  cfg.BaseImage,
		memoryMB:   cfg.MemoryMB,
		cpuLimit:   cfg.CPULimit,
		networkOff: cfg.NetworkOff,
		timeout:    cfg.Timeout,
	}, nil
}

// Close closes the Docker client
func (e *DockerExecutor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// EnsureImage pulls the base image if not present
func (e *DockerExecutor) EnsureImage(ctx context.Context) error {
	// Check if image exists locally
	_, err := e.client.ImageInspect(ctx, e.baseImage)
	if err == nil {
		return nil // Image exists
	}

	slog.Info("pulling Docker image", "image", e.baseImage)
	reader, err := e.client.ImagePull(ctx, e.baseImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", e.baseImage, err)
	}
	defer reader.Close()

	// Wait for pull to complete
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (e *DockerExecutor) RunFormat(ctx context.Context, code map[string]string) (*FormatResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Build the command to format all Go files
	var goFiles []string
	for filename := range code {
		if strings.HasSuffix(filename, ".go") {
			goFiles = append(goFiles, "/workspace/"+filename)
		}
	}

	if len(goFiles) == 0 {
		return &FormatResult{OK: true, Diff: ""}, nil
	}

	// Run gofmt -d on all files
	cmd := append([]string{"gofmt", "-d"}, goFiles...)
	output, exitCode, err := e.runInContainer(execCtx, code, cmd)
	if err != nil {
		return nil, err
	}

	return &FormatResult{
		OK:   exitCode == 0 && output == "",
		Diff: output,
	}, nil
}

func (e *DockerExecutor) RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	// Create a copy of the code map for the result
	result := make(map[string]string)
	for filename, content := range code {
		result[filename] = content
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Format each Go file individually to get the formatted output
	for filename := range code {
		if !strings.HasSuffix(filename, ".go") {
			continue
		}

		// Run gofmt on the file and capture output
		cmd := []string{"gofmt", "/workspace/" + filename}
		output, exitCode, err := e.runInContainer(execCtx, code, cmd)
		if err != nil || exitCode != 0 {
			// If gofmt fails (syntax error), keep original
			continue
		}
		result[filename] = output
	}

	return result, nil
}

func (e *DockerExecutor) RunBuild(ctx context.Context, code map[string]string) (*BuildResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Add go.mod if not present
	codeWithMod := make(map[string]string)
	for k, v := range code {
		codeWithMod[k] = v
	}
	if _, ok := codeWithMod["go.mod"]; !ok {
		codeWithMod["go.mod"] = "module exercise\n\ngo 1.22\n"
	}

	// Run go build
	cmd := []string{"go", "build", "./..."}
	output, exitCode, err := e.runInContainer(execCtx, codeWithMod, cmd)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		OK:     exitCode == 0,
		Output: output,
	}, nil
}

func (e *DockerExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*TestResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Add go.mod if not present
	codeWithMod := make(map[string]string)
	for k, v := range code {
		codeWithMod[k] = v
	}
	if _, ok := codeWithMod["go.mod"]; !ok {
		codeWithMod["go.mod"] = "module exercise\n\ngo 1.22\n"
	}

	// Run go test with JSON output
	start := time.Now()
	cmd := append([]string{"go", "test", "-json", "./..."}, flags...)
	output, exitCode, err := e.runInContainer(execCtx, codeWithMod, cmd)
	duration := time.Since(start)

	if err != nil {
		return nil, err
	}

	return &TestResult{
		OK:       exitCode == 0,
		Output:   output,
		Duration: duration,
	}, nil
}

// runInContainer executes a command in a Docker container with the given code
func (e *DockerExecutor) runInContainer(ctx context.Context, code map[string]string, cmd []string) (string, int, error) {
	// Ensure image is available
	if err := e.EnsureImage(ctx); err != nil {
		return "", -1, err
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image:           e.baseImage,
		Cmd:             cmd,
		WorkingDir:      "/workspace",
		NetworkDisabled: e.networkOff,
		Tty:             false,
	}

	// Host configuration with resource limits
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   e.memoryMB * 1024 * 1024,
			NanoCPUs: int64(e.cpuLimit * 1e9),
		},
		AutoRemove: false, // We'll remove it manually after getting output
	}

	// Create container
	resp, err := e.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", -1, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	// Ensure container is removed when done
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = e.client.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true})
	}()

	// Copy code files to container
	if err := e.copyFilesToContainer(ctx, containerID, code); err != nil {
		return "", -1, fmt.Errorf("failed to copy files to container: %w", err)
	}

	// Start container
	if err := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", -1, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int
	select {
	case err := <-errCh:
		if err != nil {
			return "", -1, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	case <-ctx.Done():
		// Timeout - kill the container
		killCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.client.ContainerKill(killCtx, containerID, "KILL")
		return "", -1, ctx.Err()
	}

	// Get container logs (stdout + stderr)
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}
	logs, err := e.client.ContainerLogs(ctx, containerID, logOptions)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logs.Close()

	// Read logs and strip Docker multiplexing headers
	output, err := demuxDockerOutput(logs)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to read container output: %w", err)
	}

	return output, exitCode, nil
}

// copyFilesToContainer copies code files to the container
func (e *DockerExecutor) copyFilesToContainer(ctx context.Context, containerID string, code map[string]string) error {
	// Create a tar archive of the files
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for filename, content := range code {
		header := &tar.Header{
			Name: filename,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	// Copy tar archive to container
	return e.client.CopyToContainer(ctx, containerID, "/workspace", &buf, container.CopyToContainerOptions{})
}

// demuxDockerOutput removes Docker multiplexing headers from log output
func demuxDockerOutput(reader io.Reader) (string, error) {
	var result bytes.Buffer
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(reader, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			// If we can't read a full header, just read the rest directly
			remaining, _ := io.ReadAll(reader)
			result.Write(remaining)
			break
		}

		// Docker multiplexes stdout/stderr with 8-byte headers:
		// [stream type (1 byte)] [0 0 0 (3 bytes)] [size (4 bytes big-endian)]
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])

		if size > 0 {
			data := make([]byte, size)
			_, err := io.ReadFull(reader, data)
			if err != nil {
				return result.String(), err
			}
			result.Write(data)
		}
	}

	return result.String(), nil
}
