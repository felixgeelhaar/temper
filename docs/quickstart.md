# Temper Quickstart Guide

Get started with Temper in 5 minutes.

## Prerequisites

- Go 1.21+
- Docker (for isolated code execution)
- An LLM API key (Claude, OpenAI, or local Ollama)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/felixgeelhaar/temper.git
cd temper

# Build the binaries
go build -o temper ./cmd/temper
go build -o temperd ./cmd/temperd

# Move to your PATH (optional)
sudo mv temper temperd /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/felixgeelhaar/temper/cmd/temper@latest
go install github.com/felixgeelhaar/temper/cmd/temperd@latest
```

## First-Time Setup

Run the interactive setup:

```bash
temper init
```

This will:
1. Create `~/.temper/` directory structure
2. Generate default configuration
3. Prompt for LLM API keys
4. Copy exercise packs

### Manual Configuration

If you prefer manual setup:

```bash
# Create directories
mkdir -p ~/.temper/{sessions,exercises,logs,profiles}

# Create config file
cat > ~/.temper/config.yaml << 'EOF'
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

learning_contract:
  default_track: practice
  tracks:
    practice:
      max_level: 3
      cooldown_seconds: 60

runner:
  executor: docker
  docker:
    image: golang:1.23-alpine
    memory_mb: 384
    timeout_seconds: 30
EOF

# Add API key
cat > ~/.temper/secrets.yaml << 'EOF'
providers:
  claude:
    api_key: YOUR_API_KEY_HERE
EOF
chmod 600 ~/.temper/secrets.yaml
```

## Verify Setup

Check that everything is configured correctly:

```bash
temper doctor
```

Expected output:
```
Checking system requirements...

Docker:    ✓
LLM:       ✓ (claude ready)
Exercises: ✓ (3 packs, 41 exercises)

All checks passed!
```

## Start the Daemon

```bash
# Start in background
temper start

# Check status
temper status

# View logs
temper logs
```

## Your First Exercise

### Using the CLI

```bash
# List available exercises
temper exercise list

# Available packs:
#   Go Fundamentals (go-v1) - 14 exercises
#   Python Fundamentals (python-v1) - 13 exercises
#   TypeScript Fundamentals (typescript-v1) - 14 exercises

# Start a session (via daemon API)
curl -X POST http://localhost:7432/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{"exercise_id": "go-v1/basics/hello-world", "track": "practice"}'
```

### Using VS Code

1. Install the extension:
   ```bash
   cd editors/vscode
   npm install
   npm run compile
   # Press F5 to launch Extension Development Host
   ```

2. Open Command Palette (`Ctrl+Shift+P`)
3. Run `Temper: Start Session`
4. Select an exercise pack and exercise
5. Start coding!

### Using Neovim

1. Add the plugin to your config:
   ```lua
   {
     dir = "/path/to/temper/editors/nvim",
     config = function()
       require("temper").setup()
     end,
   }
   ```

2. Open Neovim
3. Run `:TemperStart`
4. Select an exercise
5. Use `:TemperHint` when stuck

### Using Cursor

1. Add to your MCP settings (`.cursor/mcp.json`):
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

2. Start Cursor
3. Use `temper_start` tool to begin a session
4. Use `temper_hint`, `temper_run`, etc.

## Learning Tracks

Choose a track based on your goals:

| Track | Max Level | Cooldown | Best For |
|-------|-----------|----------|----------|
| `practice` | L3 | 60s | Daily practice |
| `interview-prep` | L2 | 120s | Interview preparation |
| `learning` | L4 | 30s | Learning new concepts |

Set your track:
```bash
# In VS Code: Temper: Set Learning Mode
# In Neovim: :TemperMode practice
# Via API:
curl -X PUT http://localhost:7432/v1/sessions/{id}/policy \
  -H "Content-Type: application/json" \
  -d '{"track": "interview-prep"}'
```

## Common Workflows

### Practice Session

1. Start daemon: `temper start`
2. Open your editor
3. Start session: Select exercise
4. Write code
5. Run checks (`Ctrl+Shift+R` in VS Code)
6. Get hints when stuck (`Ctrl+Shift+H`)
7. Complete the exercise
8. End session

### Code Review

1. Write your solution
2. Request review: `Temper: Request Review`
3. Review feedback at your current level
4. Iterate based on feedback

### Getting Unstuck

1. Try solving it yourself first
2. Request a hint (L1-L2)
3. If still stuck, use "I'm Stuck" (adaptive)
4. Cooldown prevents over-reliance on hints

## Troubleshooting

### Daemon Not Starting

```bash
# Check if already running
temper status

# Check logs
temper logs

# Kill and restart
temper stop
temper start
```

### Docker Issues

```bash
# Verify Docker is running
docker info

# Check image exists
docker pull golang:1.23-alpine
```

### LLM Provider Issues

```bash
# List providers
temper provider list

# Verify API key
temper doctor
```

## Track Your Progress

View your learning statistics:

```bash
# Overview of all stats
temper stats

# Skill progression by topic
temper stats skills

# Common error patterns
temper stats errors

# Hint dependency trend over time
temper stats trend
```

Example output:
```
Learning Statistics
==================
Total Sessions:     15
Completed:          12 (80.0%)
Total Exercises:    8
Total Runs:         142
Total Hints:        23
Hint Dependency:    16.2%
Avg Time to Green:  4m30s
```

## Next Steps

- Browse available exercises: `temper exercise list`
- Read the full documentation in `docs/`
- Create custom exercise packs
- Contribute to the project

## Getting Help

- GitHub Issues: https://github.com/felixgeelhaar/temper/issues
- Documentation: `docs/` directory
