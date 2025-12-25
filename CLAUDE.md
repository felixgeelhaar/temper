# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Temper is an adaptive AI pairing tool for learning complex crafts (coding, product management, strategy). Like tempering steel, it strengthens skills through controlled, measured assistance. It enforces a "Learning Contract" where AI adapts its intervention level based on user skill - leading when stuck, pairing during learning, and stepping back as skill increases.

## Tech Stack

- **Backend**: Go with Postgres (sqlc for type-safe queries), RabbitMQ for async processing
- **Frontend**: Astro + Vue (TypeScript) with Monaco editor for sandbox
- **Editor Clients**: VS Code extension + Neovim Lua plugin
- **Infrastructure**: Docker Compose (dev), Kubernetes-ready (prod)

## Project Structure (Planned)

```
cmd/                    # Service entrypoints
  api/                  # API Gateway
  runner/               # Runner Service
  pairing/              # Pairing Engine Service
  profiles/             # Learning Profile Service
  exercises/            # Exercise Registry Service
internal/
  domain/               # Domain models and business logic
  storage/              # Database access (sqlc-generated)
  queue/                # RabbitMQ producers/consumers
  runner/               # Check execution and sandboxing
  pairing/              # Intervention selection logic
web/                    # Astro + Vue frontend
  src/
    components/         # Vue islands
    pages/              # Astro pages
editors/
  vscode/               # VS Code extension
  nvim/                 # Neovim Lua plugin
sql/
  migrations/           # Database migrations
  queries/              # sqlc query files
exercises/              # Exercise packs (Go v1)
```

## Development Commands

```bash
# Database
make migrate-up          # Run migrations
make migrate-down        # Rollback migration
make sqlc-generate       # Generate Go code from SQL queries

# Backend
go build ./cmd/...       # Build all services
go test ./...            # Run all tests
go test -run TestX ./... # Run specific test
gofmt -w .               # Format code

# Frontend
cd web && npm install    # Install dependencies
cd web && npm run dev    # Start dev server
cd web && npm run build  # Production build

# Docker
docker-compose up        # Start all services locally
docker-compose up -d     # Start in background
```

## Architecture

### Core Services

1. **API Gateway** - Auth, rate limiting, tenant enforcement
2. **Workspace Service** - Artifacts, versions, snapshots
3. **Runner Service** - Executes checks in isolated sandboxes (gofmt, go test -json, govulncheck)
4. **Pairing Engine** - Selects minimum-helpful intervention (L0-L5 levels)
5. **Learning Profile Service** - Tracks skill, dependency, blind spots
6. **Exercise Registry** - Exercise packs with rubrics and check recipes

### Intervention Levels (Learning Contract)

- L0: Clarifying question
- L1: Category hint
- L2: Location + concept (default clamp for practice)
- L3: Constrained snippet/outline (default clamp for practice)
- L4: Partial solution + explanation (gated)
- L5: Full solution (rare, explicit teach mode only)

### Editor Integration Protocol

Shared endpoints for VS Code and Neovim:
- `POST /v1/sessions/start` - Start pairing session
- `POST /v1/context/snapshot` - Send context (files, diagnostics, diff)
- `POST /v1/runs/trigger` - Remote runner execution
- `POST /v1/runs/ingest` - Local run output ingestion
- `POST /v1/pairing/intervene` - Request intervention
- `GET /v1/stream/sessions/{id}` - SSE for real-time updates

### Neovim Commands

`:PairStart`, `:PairHint`, `:PairReview`, `:PairRun`, `:PairNext`, `:PairApply`, `:PairMode observe|guided|teach`

## Domain Terminology

- **Artifact**: User-created content (code workspace, PRD)
- **Exercise**: Structured task with rubric + check recipe
- **Run**: Execution of checks producing structured outputs
- **Intervention**: AI guidance (question/hint/nudge/critique/explain/patch)
- **Learning Contract**: Policy that clamps intervention level
- **Learning Profile**: Evolving model of skill, dependency, progress

## Multi-tenancy

All entities scoped by `org_id` with strict middleware enforcement and SQL WHERE clauses. Privacy defaults to sending diffs only, not full repos. `.pairignore` supported in editor clients.

## Key Principles

1. AI restraint is a feature - understanding beats speed
2. User remains the author at all times
3. Intervention is adaptive, not user-toggled
4. Progression is earned through demonstrated understanding
5. Policy enforcement happens server-side, not in prompts
