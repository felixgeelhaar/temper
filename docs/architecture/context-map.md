# Temper Context Map

This document describes the bounded contexts and their relationships in the Temper system, following Domain-Driven Design principles.

## Bounded Contexts Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Temper System                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐         ┌─────────────────┐         ┌───────────────┐ │
│  │    Pairing      │◄─ACL───►│     Domain      │◄──ACL───│   Learning    │ │
│  │    Context      │         │     Core        │         │   Profile     │ │
│  └────────┬────────┘         └────────┬────────┘         └───────────────┘ │
│           │                           │                                     │
│           │ OHS                       │ PS                                  │
│           │                           │                                     │
│  ┌────────▼────────┐         ┌────────▼────────┐         ┌───────────────┐ │
│  │     Runner      │         │    Exercise     │         │    Session    │ │
│  │    Context      │         │    Context      │         │    Context    │ │
│  └─────────────────┘         └─────────────────┘         └───────────────┘ │
│                                                                             │
│  ┌─────────────────┐         ┌─────────────────┐         ┌───────────────┐ │
│  │      Spec       │         │    Workspace    │         │     Patch     │ │
│  │    Context      │         │    Context      │         │    Context    │ │
│  └─────────────────┘         └─────────────────┘         └───────────────┘ │
│                                                                             │
│  ═══════════════════════════════════════════════════════════════════════   │
│                            Infrastructure Layer                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐ │
│  │     Storage     │  │      Queue      │  │          LLM                │ │
│  │    (Postgres)   │  │   (RabbitMQ)    │  │   (Claude/OpenAI/Ollama)    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────────┘ │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

Legend:
  ACL = Anti-Corruption Layer      PS = Published Language/Shared Kernel
  OHS = Open Host Service          CF = Conformist
  ───► = Upstream/Downstream       ◄──► = Partnership
```

## Bounded Contexts

### 1. Domain Core (`internal/domain`)

**Purpose**: The heart of the system - contains all domain entities, value objects, aggregates, domain events, and domain services.

**Key Components**:
- **Entities**: User, Artifact, Run, Intervention, Exercise
- **Value Objects**: Intent, InterventionLevel, InterventionType, RunStatus, Difficulty
- **Aggregates**: SessionAggregate (root for pairing sessions)
- **Domain Events**: SessionStarted, InterventionDelivered, ExerciseCompleted, SkillLevelChanged
- **Domain Services**: InterventionSelector, SkillEvaluator

**Ubiquitous Language**:
- `Intervention`: AI guidance provided to the learner (not "response" or "message")
- `Intent`: The learner's purpose when requesting help (Hint, Review, Stuck, Next, Explain)
- `Level`: Intervention intensity from L0 (clarify) to L5 (full solution)
- `Run`: Code execution producing structured output
- `Artifact`: User-created work (code, documents)

### 2. Pairing Context (`internal/pairing`)

**Purpose**: Orchestrates the AI pairing experience - intervention selection, prompt generation, and response handling.

**Relationship with Domain Core**: **Anti-Corruption Layer (ACL)**
- The Pairing Context uses the domain InterventionSelector but wraps it with application-layer concerns
- Translates between application DTOs (InterventionContext) and domain types (SelectionContext)

**Key Components**:
- `Selector`: Application adapter for intervention selection (delegates to domain)
- `Prompter`: Builds LLM prompts from context
- `Service`: Orchestrates the pairing loop

### 3. Learning Profile Context (`internal/profile`)

**Purpose**: Tracks and analyzes learner progress, skill levels, and learning patterns.

**Relationship with Domain Core**: **Anti-Corruption Layer (ACL)**
- Maps between storage models (StoredProfile) and domain models (LearningProfile)
- Uses domain SkillEvaluator for cross-cutting analysis

**Key Components**:
- `Service`: Handles profile updates on domain events
- `Store`: Persistence for learning profile data
- `Analytics`: Generates learning insights and trends

### 4. Runner Context (`internal/runner`)

**Purpose**: Executes code in isolated sandboxes, producing structured output (format, build, test results).

**Relationship with Pairing Context**: **Open Host Service (OHS)**
- Provides a clean API for code execution
- Publishes RunOutput that other contexts consume

**Key Components**:
- `Executor`: Docker-based code execution
- `Checker`: Runs format, build, and test checks
- `Risk Detector`: Identifies dangerous code patterns

### 5. Exercise Context (`internal/exercise`)

**Purpose**: Manages exercise definitions, packs, and rubrics.

**Relationship with Domain Core**: **Published Language**
- Shares the Exercise entity definition with domain
- Provides exercise metadata for pairing decisions

**Key Components**:
- `Registry`: Loads and manages exercise packs
- `Loader`: Parses YAML exercise definitions

### 6. Session Context (`internal/session`)

**Purpose**: Manages pairing session lifecycle and state.

**Relationship with Domain Core**: **Published Language**
- Uses SessionAggregate for session state
- Publishes domain events on session transitions

**Key Components**:
- `Service`: Session lifecycle management
- `Store`: Session persistence

### 7. Spec Context (`internal/spec`)

**Purpose**: Handles product specifications for feature guidance sessions.

**Relationship with Domain Core**: **Conformist**
- Uses ProductSpec, Feature, AcceptanceCriterion from domain
- Provides spec parsing and validation

**Key Components**:
- `Store`: Manages .temper/spec.yaml files
- `Validator`: Validates spec completeness
- `SpecLock`: Tracks spec drift with SHA256 hashing

### 8. Workspace Context (`internal/workspace`)

**Purpose**: Manages user artifacts and code files.

**Relationship with Domain Core**: **ACL**
- Maps between file system and domain Artifact

**Key Components**:
- `Service`: File operations and versioning
- `Store`: Artifact persistence

### 9. Patch Context (`internal/patch`)

**Purpose**: Extracts and applies code patches from AI interventions.

**Relationship with Pairing Context**: **Downstream Conformist**
- Consumes intervention output
- Extracts code blocks for potential application

**Key Components**:
- `Extractor`: Parses code blocks from intervention content
- `Applier`: Applies patches to workspace
- `Audit`: Logs patch operations

## Context Relationships

### Partnership Relationships

| Upstream | Downstream | Pattern | Notes |
|----------|------------|---------|-------|
| Domain Core | Pairing | ACL | Pairing adapts domain services for application use |
| Domain Core | Profile | ACL | Profile updates domain models through events |
| Domain Core | Session | PS | Session uses SessionAggregate directly |
| Exercise | Pairing | PS | Exercise metadata used in intervention selection |
| Runner | Pairing | OHS | Runner provides structured output for context |
| Spec | Pairing | CF | Pairing uses spec for feature guidance |
| Pairing | Patch | CF | Patch extracts code from interventions |

### Anti-Corruption Layers

1. **Pairing → Domain**
   - `InterventionContext` (app) → `SelectionContext` (domain)
   - Selector delegates to domain InterventionSelector

2. **Profile → Domain**
   - `StoredProfile` (storage) → `LearningProfile` (domain)
   - Profile service handles mapping

3. **Repository → Domain**
   - Storage models → Domain entities
   - Repository package contains all mapping logic

## Integration Patterns

### Event-Driven Communication

Domain events flow through the system:

```
Session Started → Profile.OnSessionStart()
                → Appreciation.CheckProgress()

Run Completed → Pairing.UpdateContext()
              → Profile.OnRunComplete()

Intervention Delivered → Patch.Extract()
                       → Profile.OnHintDelivered()
                       → Appreciation.CheckEscalationReduction()

Exercise Completed → Profile.OnSessionComplete()
                   → Appreciation.Acknowledge()
```

### Synchronous Dependencies

```
User Request → Daemon API → Session Service
                          → Pairing Service → LLM Provider
                                            → Runner Service
                                            → Profile Service
```

## Shared Kernel

The following types are shared across contexts via the domain package:

- `Intent`, `InterventionLevel`, `InterventionType` (value objects)
- `RunStatus`, `Difficulty` (value objects)
- `LearningPolicy`, `LearningProfile` (domain models)
- Domain events (SessionStarted, etc.)

## Infrastructure Integration

### Storage (Repository Pattern)

```
Domain Entities ◄─── Repository ───► Storage Layer (sqlc)
                        │
                    Mapper.go
```

### LLM Integration

```
Pairing Context ───► LLM Provider (interface)
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
           Claude     OpenAI      Ollama
```

### MCP Server

```
Editor (VS Code/Neovim) ───► MCP Server ───► Daemon API
                                              │
                              ┌───────────────┼───────────────┐
                              ▼               ▼               ▼
                          Session         Pairing         Runner
```

## Evolution Guidelines

### Adding New Contexts

1. Define clear boundaries and ubiquitous language
2. Identify relationship type with existing contexts
3. Use ACL if external integration or legacy concerns
4. Prefer domain events for loose coupling
5. Add mapping tests for ACL transformations

### Refactoring Context Boundaries

1. Move domain logic to Domain Core
2. Keep application orchestration in context services
3. Infrastructure concerns go to dedicated packages (storage, queue, llm)
4. Document relationship changes in this map

---

*Last updated: 2024-01-03*
*DDD Maturity: 7.2/10 → Target: 8.5/10*
