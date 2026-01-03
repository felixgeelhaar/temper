package runner

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDemuxDockerOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{
			name:  "empty input",
			input: nil,
			want:  "",
		},
		{
			name:    "valid stdout message",
			input:   makeDockerHeader(1, 5, []byte("hello")),
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "valid stderr message",
			input:   makeDockerHeader(2, 5, []byte("error")),
			want:    "error",
			wantErr: false,
		},
		{
			name: "multiple messages",
			input: append(
				makeDockerHeader(1, 6, []byte("hello\n")),
				makeDockerHeader(1, 6, []byte("world\n"))...,
			),
			want:    "hello\nworld\n",
			wantErr: false,
		},
		{
			name: "mixed stdout and stderr",
			input: append(
				makeDockerHeader(1, 4, []byte("out\n")),
				makeDockerHeader(2, 4, []byte("err\n"))...,
			),
			want:    "out\nerr\n",
			wantErr: false,
		},
		{
			name:    "partial header reads remaining",
			input:   []byte("short"), // Less than 8 bytes - can't form valid header
			want:    "",              // ReadFull consumes bytes, remaining is empty
			wantErr: false,           // Code handles gracefully
		},
		{
			name:    "zero size message",
			input:   makeDockerHeader(1, 0, nil),
			want:    "",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.input)
			got, err := demuxDockerOutput(reader)

			if (err != nil) != tc.wantErr {
				t.Errorf("demuxDockerOutput() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if got != tc.want {
				t.Errorf("demuxDockerOutput() = %q, want %q", got, tc.want)
			}
		})
	}
}

// makeDockerHeader creates a Docker log message with the specified stream type and payload
// Stream types: 0=stdin, 1=stdout, 2=stderr
func makeDockerHeader(streamType byte, size int, payload []byte) []byte {
	header := make([]byte, 8)
	header[0] = streamType
	// bytes 1-3 are padding zeros
	binary.BigEndian.PutUint32(header[4:], uint32(size))

	result := append(header, payload...)
	return result
}

func TestDemuxDockerOutput_PartialPayload(t *testing.T) {
	// Create a header that promises more data than provided
	header := make([]byte, 8)
	header[0] = 1 // stdout
	binary.BigEndian.PutUint32(header[4:], 100) // claims 100 bytes

	input := append(header, []byte("partial")...) // but only provides 7
	reader := bytes.NewReader(input)

	got, err := demuxDockerOutput(reader)

	// Should return what it could read, but may return error
	if got == "" && err == nil {
		t.Error("expected either partial content or error")
	}
}

func TestDefaultDockerConfig_Values(t *testing.T) {
	cfg := DefaultDockerConfig()

	if cfg.BaseImage == "" {
		t.Error("BaseImage should not be empty")
	}
	if cfg.MemoryMB == 0 {
		t.Error("MemoryMB should not be zero")
	}
	if cfg.CPULimit == 0 {
		t.Error("CPULimit should not be zero")
	}
	if cfg.Timeout == 0 {
		t.Error("Timeout should not be zero")
	}
	if !cfg.NetworkOff {
		t.Error("NetworkOff should default to true for security")
	}
}

func TestCreateTempCodeDir(t *testing.T) {
	tests := []struct {
		name    string
		code    map[string]string
		wantErr bool
	}{
		{
			name: "simple file",
			code: map[string]string{
				"main.go": "package main\n",
			},
			wantErr: false,
		},
		{
			name: "multiple files",
			code: map[string]string{
				"main.go":   "package main\n",
				"util.go":   "package main\n",
				"config.go": "package main\n",
			},
			wantErr: false,
		},
		{
			name: "nested directories",
			code: map[string]string{
				"main.go":          "package main\n",
				"pkg/util/util.go": "package util\n",
				"internal/app.go":  "package internal\n",
			},
			wantErr: false,
		},
		{
			name:    "empty code map",
			code:    map[string]string{},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := createTempCodeDir(tc.code)
			if (err != nil) != tc.wantErr {
				t.Errorf("createTempCodeDir() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if err != nil {
				return
			}
			defer removeTempDir(tmpDir)

			// Verify directory exists
			if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
				t.Error("temp directory should exist")
			}

			// Verify files were created
			for filename, content := range tc.code {
				filePath := filepath.Join(tmpDir, filename)
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read %s: %v", filename, err)
					continue
				}
				if string(data) != content {
					t.Errorf("file %s content = %q, want %q", filename, string(data), content)
				}
			}
		})
	}
}

func TestRemoveTempDir(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test-remove-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create some files in it
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("nested"), 0644)

	// Remove it
	removeTempDir(tmpDir)

	// Verify it's gone
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Error("temp directory should be removed")
	}
}

func TestNewLocalExecutor_Internal(t *testing.T) {
	workDir := t.TempDir()
	exec := NewLocalExecutor(workDir)

	if exec == nil {
		t.Fatal("NewLocalExecutor should not return nil")
	}
	if exec.workDir != workDir {
		t.Errorf("workDir = %s, want %s", exec.workDir, workDir)
	}
}
