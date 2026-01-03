# ADR-001: Domain-Driven Design Architecture

## Status
Accepted

## Context
Temper is an adaptive AI pairing tool for learning. The initial implementation prioritized speed-to-market, resulting in:
- Anemic domain models with business logic scattered across services
- Missing aggregate boundaries causing data consistency issues
- No domain events for cross-boundary communication
- Storage layer types leaking into domain logic
- Missing production resilience patterns

A DDD maturity assessment scored the codebase at 4.1/10.

## Decision
We restructured the codebase following DDD tactical patterns:

### 1. Domain Value Objects
Created immutable value objects in `internal/domain/value_objects.go`:
- `InterventionLevel` (L0-L5) with validation and comparison
- `ExerciseID` with format validation
- `RunStatus` with state machine transitions

### 2. Aggregate Boundaries
Defined `PairingSession` as an aggregate root in `internal/domain/session_aggregate.go`:
- Encapsulates session state, policy, and context
- Controls intervention requests through aggregate methods
- Emits domain events for state changes
- Enforces invariants (active session required, level clamping)

### 3. Domain Events
Implemented event infrastructure in `internal/domain/events.go`:
- `DomainEvent` interface with ID, timestamp, aggregate info
- Concrete events: `SessionStartedEvent`, `InterventionRequestedEvent`, `SessionEndedEvent`
- `EventDispatcher` for publishing and subscribing

### 4. Anti-Corruption Layer
Created `internal/repository/` package to isolate domain from storage:
- Repository interfaces defined in `internal/domain/repository.go`
- Mapper functions translate between domain and storage types
- `UnitOfWork` pattern for transactional boundaries
- Domain errors (`ErrUserNotFound`, etc.) instead of SQL errors

### 5. Production Resilience
Integrated `felixgeelhaar/fortify` for fault tolerance:
- **Circuit Breaker**: 3 consecutive failures opens circuit, 60s recovery
- **Retry**: Exponential backoff (2s initial, 60s max) with jitter
- **Rate Limiting**: Token bucket (2 req/s default for LLM)
- **Bulkhead**: Concurrency limiting (5 concurrent LLM calls)
- **HTTP Timeouts**: 120s for LLM responses, 10s dial timeout

## Consequences

### Positive
- Clear boundaries between domain logic and infrastructure
- Business rules enforced in one place (aggregates)
- Testable domain logic without database dependencies
- Resilient LLM integration with graceful degradation
- Type-safe value objects prevent invalid states

### Negative
- More code to maintain (mappers, events, value objects)
- Learning curve for developers unfamiliar with DDD
- Some performance overhead from event dispatching

### Risks Mitigated
- **Cascading LLM failures**: Circuit breaker prevents overload
- **Data inconsistency**: Aggregate boundaries ensure invariants
- **SQL injection via types**: Anti-corruption layer sanitizes data

## References
- Evans, Eric. "Domain-Driven Design: Tackling Complexity in the Heart of Software"
- Vernon, Vaughn. "Implementing Domain-Driven Design"
- Fortify library: github.com/felixgeelhaar/fortify
