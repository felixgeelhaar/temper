# Temper Architecture Overview

## System Purpose
Temper is an adaptive AI pairing tool for learning complex crafts through deliberate practice. It enforces a "Learning Contract" where AI adapts intervention levels based on demonstrated skill.

## Architecture Style
**Modular Monolith** with DDD tactical patterns, designed for eventual extraction to microservices.

## Package Structure

```
cmd/
  temper/              # CLI for user interaction
  temperd/             # Daemon server

internal/
  domain/              # Core business logic (DDD)
    ├── value_objects.go    # InterventionLevel, ExerciseID, RunStatus
    ├── session_aggregate.go # PairingSession aggregate root
    ├── events.go           # Domain events infrastructure
    ├── repository.go       # Repository interfaces
    └── errors.go           # Domain errors

  repository/          # Anti-corruption layer
    ├── mapper.go           # Domain ↔ Storage translation
    ├── user_repository.go
    ├── artifact_repository.go
    └── unit_of_work.go     # Transaction management

  llm/                 # LLM provider integration
    ├── provider.go         # Provider interface
    ├── claude.go           # Anthropic Claude
    ├── openai.go           # OpenAI/compatible
    ├── ollama.go           # Local Ollama
    ├── resilient.go        # Fortify wrapper
    └── httpclient.go       # Configured HTTP client

  pairing/             # Pairing engine
  runner/              # Code execution (Docker/local)
  session/             # Session management
  profile/             # Learning profile tracking
  spec/                # Spec parsing and validation
  risk/                # Code risk detection
  daemon/              # HTTP server and handlers
  storage/             # sqlc-generated database access
```

## Key Design Decisions

### 1. Domain-Driven Design
- **Aggregates**: PairingSession is the primary aggregate root
- **Value Objects**: Immutable types with validation (InterventionLevel)
- **Domain Events**: Decouple cross-boundary communication
- **Repository Pattern**: Abstract persistence from domain

### 2. Anti-Corruption Layer
The `repository/` package isolates domain from storage:
```
Domain Model ←→ Repository ←→ Mapper ←→ Storage (sqlc)
```

### 3. Resilience (via Fortify)
LLM providers wrapped with:
- Circuit breaker (fail-fast on provider outage)
- Retry with exponential backoff
- Rate limiting (respect provider quotas)
- Bulkhead (limit concurrent calls)

### 4. Intervention Levels
```
L0: Clarifying question       (least helpful)
L1: Category hint
L2: Location + concept        (default clamp)
L3: Constrained snippet
L4: Partial solution          (gated)
L5: Full solution             (rare, explicit only)
```

## Data Flow

### Pairing Request
```
User → CLI/Editor → Daemon → Session Aggregate → Pairing Engine → LLM Provider
                                    ↓
                            Domain Events → Learning Profile Update
```

### Code Execution
```
User Code → Runner Service → Docker Container → Output Parser → Run Result
                                                      ↓
                                              Pairing Engine (feedback)
```

## External Dependencies

| Dependency | Purpose |
|------------|---------|
| PostgreSQL | Primary data store |
| RabbitMQ | Async job processing |
| Docker | Sandboxed code execution |
| Claude/OpenAI/Ollama | LLM providers |
| Fortify | Resilience patterns |

## Security Considerations

- Password hashing via bcrypt (cost 12)
- Path traversal prevention in workspace
- Docker network isolation for runners
- Rate limiting on API endpoints
- No secrets in code (env vars only)

## Future Considerations

- **Provider Fallback**: Auto-switch providers on failure
- **Distributed Tracing**: OpenTelemetry integration
- **Metrics Export**: Prometheus endpoint
- **Service Extraction**: Split runner/pairing to separate services
