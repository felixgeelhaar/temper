# Architecture

Temper follows a local-first, daemon-based architecture.

## Components

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  VS Code    │     │   Neovim    │     │   Cursor    │
│  Extension  │     │   Plugin    │     │     MCP     │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                    ┌──────▼──────┐
                    │   temperd   │
                    │   (daemon)  │
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼───────┐  ┌───────▼───────┐  ┌───────▼───────┐
│    Runner     │  │    Pairing    │  │   Learning    │
│   (Local/     │  │    Engine     │  │    Profile    │
│    Docker)    │  │               │  │               │
└───────────────┘  └───────────────┘  └───────────────┘
```

## Daemon (`temperd`)

The daemon runs locally and provides:
- HTTP API for editor clients
- SSE streaming for real-time updates
- Session management
- Exercise registry
- Learning profile storage

## Runner

Executes code checks locally or in Docker:
- Format (gofmt, black, prettier)
- Build (go build, tsc)
- Test (go test, pytest, vitest)

## Pairing Engine

Selects the minimum helpful intervention:
1. Analyzes context (code, errors, history)
2. Determines appropriate intervention level
3. Generates response via LLM
4. Enforces learning contract limits

## Learning Profile

Tracks user progress:
- Skill levels by topic
- Error patterns
- Hint dependency
- Session history
