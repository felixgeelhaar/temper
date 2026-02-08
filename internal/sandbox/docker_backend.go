package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// DockerBackend manages Docker container operations for sandboxes.
type DockerBackend struct {
	client *client.Client
}

// NewDockerBackend creates a new Docker backend.
func NewDockerBackend() (*DockerBackend, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	// Verify Docker is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		cli.Close()
		return nil, fmt.Errorf("docker not reachable: %w", err)
	}

	return &DockerBackend{client: cli}, nil
}

// CreateContainer creates a long-lived container for a sandbox.
func (b *DockerBackend) CreateContainer(ctx context.Context, cfg Config) (string, error) {
	// Ensure image is available
	if err := b.ensureImage(ctx, cfg.Image); err != nil {
		return "", fmt.Errorf("ensure image: %w", err)
	}

	// Create container that stays alive (sleep loop)
	containerCfg := &container.Config{
		Image:           cfg.Image,
		Cmd:             []string{"sh", "-c", "while true; do sleep 3600; done"},
		WorkingDir:      "/workspace",
		NetworkDisabled: cfg.NetworkOff,
		Tty:             false,
		Labels: map[string]string{
			"temper.sandbox": "true",
			"temper.lang":    cfg.Language,
		},
	}

	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:   int64(cfg.MemoryMB) * 1024 * 1024,
			NanoCPUs: int64(cfg.CPULimit * 1e9),
		},
	}

	resp, err := b.client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if err := b.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up on failure
		_ = b.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("start container: %w", err)
	}

	return resp.ID, nil
}

// CopyFiles copies code files into a running container.
func (b *DockerBackend) CopyFiles(ctx context.Context, containerID string, files map[string]string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return fmt.Errorf("write tar content: %w", err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}

	return b.client.CopyToContainer(ctx, containerID, "/workspace", &buf, container.CopyToContainerOptions{})
}

// Exec executes a command inside a running container.
func (b *DockerBackend) Exec(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (*ExecResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execCfg := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   "/workspace",
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := b.client.ContainerExecCreate(execCtx, containerID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("create exec: %w", err)
	}

	start := time.Now()

	attachResp, err := b.client.ContainerExecAttach(execCtx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("attach exec: %w", err)
	}
	defer attachResp.Close()

	// Read combined output
	var outBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, attachResp.Reader)

	duration := time.Since(start)

	// Get exit code
	inspectResp, err := b.client.ContainerExecInspect(execCtx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("inspect exec: %w", err)
	}

	// Demux the Docker stream output
	raw := outBuf.Bytes()
	stdout, stderr := demuxOutput(raw)

	return &ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
	}, nil
}

// PauseContainer pauses a running container.
func (b *DockerBackend) PauseContainer(ctx context.Context, containerID string) error {
	return b.client.ContainerPause(ctx, containerID)
}

// ResumeContainer unpauses a paused container.
func (b *DockerBackend) ResumeContainer(ctx context.Context, containerID string) error {
	return b.client.ContainerUnpause(ctx, containerID)
}

// DestroyContainer stops and removes a container.
func (b *DockerBackend) DestroyContainer(ctx context.Context, containerID string) error {
	timeout := 10
	_ = b.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	return b.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// IsContainerRunning checks if a container is running.
func (b *DockerBackend) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	info, err := b.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}

// Close closes the Docker client.
func (b *DockerBackend) Close() error {
	return b.client.Close()
}

func (b *DockerBackend) ensureImage(ctx context.Context, img string) error {
	_, err := b.client.ImageInspect(ctx, img)
	if err == nil {
		return nil // Already present
	}

	reader, err := b.client.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", img, err)
	}
	defer reader.Close()
	// Drain the reader to complete the pull
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// demuxOutput separates Docker multiplexed stdout/stderr streams.
// Docker stream protocol uses 8-byte headers: [type][0][0][0][size1][size2][size3][size4]
// type: 1=stdout, 2=stderr
func demuxOutput(data []byte) (stdout, stderr string) {
	var outBuf, errBuf strings.Builder

	for len(data) >= 8 {
		streamType := data[0]
		size := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		data = data[8:]

		if size > len(data) {
			size = len(data)
		}

		chunk := string(data[:size])
		data = data[size:]

		switch streamType {
		case 1:
			outBuf.WriteString(chunk)
		case 2:
			errBuf.WriteString(chunk)
		}
	}

	// If no headers were found, treat entire output as stdout
	if outBuf.Len() == 0 && errBuf.Len() == 0 && len(data) > 0 {
		return string(data), ""
	}

	return outBuf.String(), errBuf.String()
}
