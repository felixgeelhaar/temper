# Temper

Adaptive AI pairing for learning - a CLI-first tool that enforces a Learning Contract to build genuine skill.

Like tempering steel, Temper strengthens skills through controlled, measured assistance. The AI leads when you're stuck, pairs during learning, and steps back as your skill increases.

## Features

- **Learning Contract**: Adaptive intervention levels (L0-L5) that limit AI help based on skill
- **IDE Integration**: Works with VS Code, Neovim, and Cursor (via MCP)
- **Local-First**: Runs as a daemon on your machine with BYOK (Bring Your Own Key)
- **Structured Practice**: Exercise packs with rubrics and check recipes
- **Real Feedback**: Runs `gofmt`, `go build`, and `go test` in Docker isolation

## Quick Start

```bash
# Install
go install github.com/felixgeelhaar/temper/cmd/temper@latest
go install github.com/felixgeelhaar/temper/cmd/temperd@latest

# Initialize (creates ~/.temper/, configures API keys)
temper init

# Check requirements
temper doctor

# Start the daemon
temper start

# List exercises
temper exercise list
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Your Machine                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐    │
│  │ temper   │   │ Neovim   │   │ VS Code  │   │ Cursor   │    │
│  │  CLI     │   │ Plugin   │   │ Extension│   │ (MCP)    │    │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘    │
│       └──────────────┼──────────────┼──────────────┘           │
│                      │ HTTP (localhost:7432)                    │
│                      ▼                                          │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    temperd (Daemon)                          ││
│  │  Sessions │ Exercises │ Runner │ Pairing Engine             ││
│  └─────────────────────────────────────────────────────────────┘│
│                          │                                       │
│  ┌───────────────────────┼───────────────────────────────────┐  │
│  │               ~/.temper/ (Local Storage)                   │  │
│  │  config.yaml │ secrets.yaml │ sessions/ │ exercises/      │  │
│  └────────────────────────────────────────────────────────────┘  │
│                          │                                       │
│  ┌───────────────────────┼───────────────────────────────────┐  │
│  │           LLM Providers (BYOK)                             │  │
│  │  Claude │ OpenAI │ Ollama (local)                         │  │
│  └────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## CLI Commands

### Setup
```bash
temper init              # First-time setup
temper doctor            # Check Docker, LLM providers
temper config            # Show configuration
temper provider set-key claude  # Set Claude API key
```

### Daemon
```bash
temper start             # Start daemon in background
temper stop              # Stop daemon
temper status            # Show daemon status
temper logs              # View daemon logs
```

### Exercises
```bash
temper exercise list     # List available exercises
temper exercise info go-v1/basics/hello-world  # Show exercise details
```

### MCP (for Cursor)
```bash
temper mcp               # Start MCP server (stdio)
```

## IDE Integration

### VS Code

1. Install the extension from `editors/vscode/`
2. Start the daemon: `temper start`
3. Use Command Palette: `Temper: Start Session`

**Keybindings:**
- `Ctrl+Shift+H`: Get hint
- `Ctrl+Shift+R`: Run checks

### Neovim

Add to your config:
```lua
-- Using lazy.nvim
{
  dir = "path/to/temper/editors/nvim",
  config = function()
    require("temper").setup({
      daemon_host = "127.0.0.1",
      daemon_port = 7432,
    })
  end,
}
```

**Commands:**
- `:TemperStart` - Start session
- `:TemperHint` - Get hint
- `:TemperRun` - Run checks
- `:TemperReview` - Code review

### Cursor

Add to your MCP configuration:
```json
{
  "mcpServers": {
    "temper": {
      "command": "temper",
      "args": ["mcp"]
    }
  }
}
```

**Available tools:**
- `temper_start` - Start a session
- `temper_hint` - Get a hint
- `temper_review` - Code review
- `temper_run` - Run checks
- `temper_stuck` - Signal being stuck
- `temper_explain` - Explain concept

## Learning Contract

Temper enforces intervention levels to promote genuine learning:

| Level | Type | Description |
|-------|------|-------------|
| L0 | Question | Clarifying questions only |
| L1 | Category | Direction to explore |
| L2 | Location | Location + concept hint |
| L3 | Snippet | Constrained code snippets |
| L4 | Partial | Partial solutions (gated) |
| L5 | Solution | Full solutions (rare) |

### Tracks

| Track | Max Level | Cooldown | Use Case |
|-------|-----------|----------|----------|
| `practice` | L3 | 60s | Normal practice |
| `interview-prep` | L2 | 120s | Interview preparation |
| `learning` | L4 | 30s | Learning new concepts |

## Configuration

Configuration is stored in `~/.temper/config.yaml`:

```yaml
daemon:
  port: 7432
  bind: "127.0.0.1"
  log_level: info

llm:
  default_provider: claude
  providers:
    claude:
      enabled: true
      model: claude-sonnet-4-20250514
    ollama:
      enabled: true
      url: http://localhost:11434
      model: llama2

learning_contract:
  default_track: practice
  tracks:
    practice:
      max_level: 3
      cooldown_seconds: 60
    interview_prep:
      max_level: 2
      cooldown_seconds: 120

runner:
  executor: docker
  docker:
    image: golang:1.23-alpine
    memory_mb: 384
    timeout_seconds: 30
```

API keys are stored separately in `~/.temper/secrets.yaml` (not committed to version control).

## Development

```bash
# Clone
git clone https://github.com/felixgeelhaar/temper.git
cd temper

# Build
go build ./cmd/temper
go build ./cmd/temperd

# Run tests
go test ./...

# Start in development
./temperd  # Foreground mode with logging
```

## Project Structure

```
temper/
├── cmd/
│   ├── temper/          # CLI tool
│   └── temperd/         # Daemon
├── internal/
│   ├── config/          # Configuration loading
│   ├── daemon/          # HTTP server & handlers
│   ├── domain/          # Domain models
│   ├── exercise/        # Exercise loader
│   ├── llm/             # LLM providers (Claude, OpenAI, Ollama)
│   ├── mcp/             # MCP server for Cursor
│   ├── pairing/         # Intervention selection
│   ├── runner/          # Code execution (Docker/local)
│   └── session/         # Session management
├── editors/
│   ├── nvim/            # Neovim Lua plugin
│   └── vscode/          # VS Code extension
└── exercises/           # Exercise packs
```

## License

MIT
