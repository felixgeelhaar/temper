package main

import (
	"fmt"
	"os"
	"strings"
)

// Version is set at build time via ldflags
var Version = "dev"

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
		fmt.Printf("temper %s\n", Version)
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
