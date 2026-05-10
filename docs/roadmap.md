# Temper Roadmap

**Product:** Temper
**Status:** v1 core complete

---

## Implementation Status

✅ = Complete | 🔨 = In Progress | ⏳ = Planned

---

## v1 — Learning-First Pairing

**Goal:** Prove that restrained AI pairing improves developer learning in real work.

### Outcomes
- Users feel supported, not robbed of learning
- Progress is visible and evidence-based
- AI restraint is trusted, not resented

### Core Infrastructure ✅

| Component | Status | Notes |
|-----------|--------|-------|
| CLI (`temper`) | ✅ | Install, updates, diagnostics |
| Daemon (`temperd`) | ✅ | Local server, session management |
| Learning Profile | ✅ | Skill tracking, error patterns, hint dependency |
| Exercise System | ✅ | 85 exercises across 6 language packs (Go, Python, TypeScript, Java, Rust, C) |
| Progress CLI | ✅ | `temper stats` with overview, skills, errors, trends |

### IDE Integrations

| Editor | Status | Notes |
|--------|--------|-------|
| VS Code | ✅ | Extension available |
| Cursor | ✅ | MCP server integration |
| Neovim | ✅ | Lua plugin with full feature parity |

### Session Model

| Feature | Status | Notes |
|---------|--------|-------|
| Training intent | ✅ | Structured exercises |
| Greenfield intent | ✅ | New project guidance |
| Feature Guidance intent | ✅ | Spec-driven feature work |
| Intent inference | ✅ | Auto-detect from context |

### Pairing Loop

| Feature | Status | Notes |
|---------|--------|-------|
| Hints & questions | ✅ | L0-L5 intervention levels |
| Run checks | ✅ | Local execution via Docker |
| Targeted feedback | ✅ | Based on check results |
| Risk notices | ✅ | Warn about risky patterns |

### Spec-Driven Workflow

| Feature | Status | Notes |
|---------|--------|-------|
| Spec format (Specular) | ✅ | Define intent & acceptance |
| Spec validation | ✅ | Check completeness |
| Spec-anchored feedback | ✅ | Review against spec |
| SpecLock drift detection | ✅ | SHA256 hashing |
| CLI commands | ✅ | `temper spec create/list/validate/status/lock/drift` |

### Patch Policy

| Feature | Status | Notes |
|---------|--------|-------|
| No automatic changes | ✅ | Policy enforced |
| Explicit escalation | ✅ | User must request L4/L5 with justification |
| Patch preview | ✅ | Show before apply |
| Patch logging | ✅ | Local audit trail with JSONL format |

### Progress & Appreciation

| Feature | Status | Notes |
|---------|--------|-------|
| Hint dependency tracking | ✅ | `temper stats` |
| Escalation reduction | ✅ | Tracked over time |
| Evidence-based appreciation | ✅ | Calm, professional tone |

---

## v2 — Team & Scale (Future)

**Goal:** Extend Temper to teams and broader contexts.

### Planned Features

| Feature | Description |
|---------|-------------|
| Sandboxes | Isolated environments for exercises |
| Team policies | Shared learning contracts |
| External context providers | Pull context from docs, repos |
| Web-based progress views | Dashboard for progress review |
| Multi-language expansion | Rust, Java, etc. |

---

## What We're NOT Building (v1)

- ❌ Autonomous coding agent
- ❌ Speed/volume optimization
- ❌ Cloud-hosted sandboxes
- ❌ Ticketing system integration (Jira, GitHub Issues)
- ❌ Gamification (streaks, points, leaderboards)

---

## Current Focus

**Priority for v1 completion:**

1. ~~**Spec-driven workflow**~~ ✅ — Specular format with validation, locking, drift detection
2. ~~**Intent inference**~~ ✅ — Auto-detect Training/Greenfield/Feature from context
3. ~~**Neovim plugin**~~ ✅ — Full feature parity with VS Code extension
4. ~~**Evidence-based appreciation**~~ ✅ — Progress recognition without gamification

**v1 Core Features Complete!**

---

## Philosophy

> This product is not about doing work faster.
> It is about protecting and scaling human judgment in the age of AI.
> That is a rare, meaningful, and defensible mission.
