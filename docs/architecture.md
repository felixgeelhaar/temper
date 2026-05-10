# Temper Architecture Overview

## System Purpose
Temper is an adaptive AI pairing tool for learning complex crafts through deliberate practice. It enforces a "Learning Contract" where AI adapts intervention levels based on demonstrated skill.

## Architecture Style
**Local-first modular monolith** with DDD tactical patterns. A single `temperd` binary on `127.0.0.1:7432` serves the CLI and editor integrations.

## Runtime Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                          Your Machine                            │
├─────────────────────────────────────────────────────────────────┤
│  temper CLI │ Neovim │ VS Code │ Cursor (MCP)                   │
│       │         │         │           │                          │
│       └─────────┴────┬────┴───────────┘                          │
│                      │ HTTP + Bearer token (localhost:7432)      │
│                      ▼                                            │
│              ┌──────────────┐                                     │
│              │   temperd    │                                     │
│              │  (modular    │                                     │
│              │   monolith)  │                                     │
│              └──────┬───────┘                                     │
│                     │                                              │
│  ┌──────────────────┼─────────────────────┐                       │
│  │           ~/.temper/                    │                       │
│  │  config.yaml │ secrets.yaml │ temper.db │                       │
│  └─────────────────────────────────────────┘                       │
│                     │                                              │
│  ┌──────────────────┼─────────────────────┐                       │
│  │   Docker (per-run sandboxes)            │                       │
│  │   LLM provider (Claude / OpenAI / Ollama)│                      │
│  └─────────────────────────────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

There is no remote SaaS, no Postgres, no message broker. v1 is single-binary,
single-machine. Future hosted tier (v2+) is a separate decision tracked in
`docs/roadmap.md`.

## Package Structure

```
cmd/
  temper/             # CLI: init, doctor, start/stop, exercise, spec, stats, mcp
  temperd/            # Daemon entrypoint
  eval-harness/       # Golden-set pairing evaluator (BYOK)

internal/
  domain/             # Aggregates, value objects, domain events (pure DDD core)
  pairing/            # Selector, Prompter, ClampValidator, fence, Service
  llm/                # Provider interface, Claude, OpenAI, Ollama, ResilientProvider
  runner/             # Code execution: DockerExecutor, LocalExecutor, parsers
  sandbox/            # Persistent Docker sandboxes for sessions
  session/            # Session lifecycle, intent inference
  profile/            # Learning profile, topics, error patterns, analytics
  spec/               # Specular spec parser, validator, lock, drift
  risk/               # Risk pattern detector
  patch/              # Patch policy + audit log
  appreciation/       # Evidence-based progress recognition
  exercise/           # Exercise pack loader and registry
  docindex/           # Document indexing for spec authoring context
  daemon/             # HTTP server, middleware (auth, host guard, CORS), handlers
  mcp/                # MCP server for Cursor
  config/             # Config + secrets loading, auth-token generation
  storage/
    local/            # JSON file storage backend
    sqlite/           # SQLite storage backend (default)
    migrations/       # SQLite schema migrations (embed)
  workspace/          # Artifact + version model
  eval/               # Pairing evaluation harness (case loader, scorer, runner)
```

Every internal package is consumed only via its public interface. The
domain package imports nothing from other internal packages.

## Key Design Decisions

### 1. Local-first, single-process
The whole system runs as one Go binary against the local filesystem.
No service-to-service network calls, no shared database, no queue.
This trades horizontal scalability for zero-deps install and BYOK
privacy. v1 success criteria do not require scale beyond one user
per machine.

### 2. Domain-Driven Design
- Aggregates: `PairingSession`, `Run`, `LearningProfile`.
- Value objects: `InterventionLevel`, `ExerciseID`, `SessionIntent` —
  immutable, validated, no behavior beyond invariants.
- Domain events: significant transitions (session created, run completed,
  hint delivered) flow through `domain/events.go`.

### 3. Trust boundaries
Three layers protect the "AI restraint as feature" promise:
1. **Selector** picks the level deterministically from policy + signals.
2. **System prompt** instructs the LLM to honor the level.
3. **ClampValidator** verifies output and retries with a tightening
   directive on violation. The clamp is the only one that catches a
   misbehaving model.

### 4. Prompt-injection mitigation
Every user-, exercise-author-, or spec-author-controlled string is
wrapped in nonce-fenced delimiters before insertion into the LLM
prompt. Per-request 16-byte nonces defeat closing-delimiter forgery.
See `internal/pairing/injection.go`.

### 5. Daemon API authentication
Every `/v1` route except `/v1/health` requires a Bearer token loaded
from `~/.temper/secrets.yaml`. Token comparison is constant-time. A
Host-header guard rejects DNS-rebinding attempts. The daemon refuses
to start on a non-loopback bind without a configured token.

### 6. Resilience (via Fortify)
LLM providers wrap with: circuit breaker, exponential backoff retry,
bulkhead concurrency limit, rate limiter. Stream calls skip retry and
bulkhead since they're stateful and long-running.

### 7. Intervention Levels
```
L0: Clarifying question       (least helpful)
L1: Category hint
L2: Location + concept        (default clamp for stricter tracks)
L3: Constrained snippet       (default clamp for practice)
L4: Partial solution          (gated behind explicit escalation)
L5: Full solution             (rare, escalation with justification)
```

## Data Flow

### Hint request
```
User → CLI/Editor → daemon (/v1/sessions/{id}/hint)
  → pairing.Selector picks level → pairing.Prompter builds prompt
  → llm.Provider generates → pairing.ClampValidator checks
  → (retry if violated) → response → editor
```

### Code execution
```
User → daemon (/v1/sessions/{id}/runs)
  → runner.DockerExecutor (per-run container, network-off)
  → output parsed into RunOutput → run persisted → response
```

### Sandbox session
```
User → daemon (/v1/sessions/{id}/sandbox)
  → sandbox.Manager creates persistent container (10 max, 30 min idle)
  → AttachCode + Execute via cached container → response
  → Background cleanup loop reaps expired containers
```

## External Dependencies

| Dependency        | Purpose                                              |
|-------------------|------------------------------------------------------|
| Docker            | Per-run sandbox + persistent session sandbox         |
| Claude / OpenAI   | LLM providers (BYOK)                                 |
| Ollama            | Optional local LLM                                   |
| SQLite            | Default storage backend (`~/.temper/temper.db`)      |
| Fortify           | Circuit breaker, retry, bulkhead, rate limit         |

## Security Considerations

- Bearer-token auth on every `/v1` route except `/v1/health`.
- Host-header allowlist defeats DNS-rebinding.
- CORS allowlist restricted to localhost origins.
- Secrets stored in `~/.temper/secrets.yaml` chmod 0600.
- Docker network isolation for runners (`network_off: true`).
- Sandbox command allowlist (see `internal/sandbox/`).
- Prompt-injection mitigation via nonce-fenced delimiters around all
  untrusted strings.
- Output-side clamp validator ensures the LLM cannot exceed policy
  even if the system prompt is overridden.

## Future Considerations

- Provider fallback / model routing per intervention level.
- Prometheus metrics endpoint for SLO tracking.
- OpenAPI spec + generated editor client types.
- Optional hosted tier (would reintroduce Postgres + queue at that
  point, behind a deliberate v2 decision).
