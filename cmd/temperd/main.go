package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/daemon"
)

const (
	pidFileName = "temperd.pid"
)

func main() {
	if err := run(); err != nil {
		slog.Error("daemon error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Ensure ~/.temper directory exists
	temperDir, err := config.EnsureTemperDir()
	if err != nil {
		return fmt.Errorf("ensure temper dir: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Setup logging
	logLevel := parseLogLevel(cfg.Daemon.LogLevel)
	logFile, err := setupLogging(temperDir, logLevel)
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	// Write PID file
	pidPath := filepath.Join(temperDir, pidFileName)
	if err := writePIDFile(pidPath); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer os.Remove(pidPath)

	// Use exercises directory (try current dir first, then ~/.temper)
	exercisePath := "./exercises"
	if _, err := os.Stat(exercisePath); os.IsNotExist(err) {
		exercisePath = filepath.Join(temperDir, "exercises")
	}

	// Create server
	ctx := context.Background()
	server, err := daemon.NewServer(ctx, daemon.ServerConfig{
		Config:       cfg,
		ExercisePath: exercisePath,
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	// Graceful shutdown
	done := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh

		slog.Info("received signal, shutting down", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "error", err)
		}
		close(done)
	}()

	// Start server
	if err := server.Start(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	<-done
	slog.Info("daemon stopped")
	return nil
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setupLogging(temperDir string, level slog.Level) (*os.File, error) {
	logPath := filepath.Join(temperDir, "logs", "temperd.log")

	// Create log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	// Create handler that writes to both stdout and file
	handler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: level,
	})

	// Also log to stderr for foreground mode
	multiHandler := &multiHandler{
		handlers: []slog.Handler{
			handler,
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}),
		},
	}

	slog.SetDefault(slog.New(multiHandler))

	return logFile, nil
}

func writePIDFile(path string) error {
	pid := os.Getpid()
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

// multiHandler logs to multiple handlers
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
