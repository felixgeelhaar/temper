
## CLI Tooling

The temper CLI handles installation, setup, diagnostics, provider management, exercise browsing, spec management, and analytics display. Split into focused command files (main, setup, daemon, exercise, spec, stats, mcp). Commands include temper init, doctor, config, provider, exercise, spec, stats, and mcp.

---

## Daemon (temperd)

Local HTTP server managing sessions, exercises, pairing, specs, analytics, and workspace state. Runs as a background process with PID file management, health checks, graceful shutdown on SIGTERM, and structured JSON logging. Exposes REST API on localhost:7432.

---

## Learning Profile

Evolving model of user skill, dependency, blind spots, and progress. Tracks skill levels per topic with confidence scoring, records error patterns with category and frequency, calculates hint dependency ratio over sliding windows, provides trend analysis, and persists profiles locally in ~/.temper/.

---

## Exercise System

Structured exercises with rubrics and check recipes across Go, Python, and TypeScript. 41 exercises organized by language, category, and difficulty with metadata for skill mapping. Loaded from ~/.temper/exercises/ with API endpoints for listing packs and retrieving details.

---

## VS Code Extension

VS Code extension providing inline pairing experience with session management, hint requests, run triggers, and progress display. Communicates with temperd via REST API. Shows session summary on stop with motivational feedback. Status bar integration showing session state.

---

## Cursor MCP Integration

MCP (Model Context Protocol) server enabling Temper as a tool provider for Cursor. Serves on stdio with session management, pairing, and exercise tools. Local runner execution with Docker optional. Signal handling for graceful shutdown.

---

## Neovim Lua Plugin

Neovim plugin with full feature parity to VS Code extension. Provides PairStart, PairHint, PairReview, PairRun, PairNext, PairApply commands and PairMode observe/guided/teach for mode switching. Floating window display for interventions.

---

## Session Model

Every session operates under an explicit intent (Training, Greenfield, Feature Guidance) that determines guidance style, intervention limits, and progress interpretation. Intent is inferred automatically from context, always visible, and changeable. Session persistence with start/stop lifecycle.

---

## Active Pairing Loop

Core value loop: request next step, write code, run checks, receive targeted feedback, adjust. L0-L5 intervention levels from clarifying question to full solution. Default clamp at L2-L3 for practice. L4/L5 gated behind explicit request. Interventions selected by minimum-helpful policy, feedback anchored to check results and spec criteria.

---

## LLM Provider System

Multi-provider LLM integration supporting Claude (Anthropic), OpenAI, and Ollama (local). Provider registry with configurable default. Resilient wrapper with circuit breaker, retry with exponential backoff, bulkhead concurrency limiting, and rate limiting via fortify library. Streaming support for real-time responses.

---

## Spec-Driven Workflow

Specular format specs define intent, acceptance criteria, and scope. Specs anchor all feedback and review. Includes spec creation with scaffold generation, validation checking completeness, progress tracking with satisfied/total acceptance criteria, SpecLock generation with SHA256 feature hashing, and drift detection comparing current spec against locked baseline. Specs live in repository under .specs/ directory.

---

## Patch Policy

No automatic code changes. No background edits. Patches only via explicit escalation, scoped to small changes, previewed before application, and logged locally in JSONL audit trail. Policy enforcement is server-side, not prompt-based.

---

## Risk Analysis

Warn about risky patterns in code changes. Surface risk notices during pairing sessions based on diff analysis and code patterns. Integrates with session feedback loop.

---

## Progress and Appreciation

Evidence-based progress recognition without gamification. Tracks reduced hint dependency, faster convergence, improved test discipline, and spec alignment. Appreciation shown sparingly with calm, professional tone. Session summary with motivational feedback on stop. No streaks, points, or leaderboards.

---

## Runner System

Execute checks in isolated sandboxes. Docker executor with configurable image, memory, and timeout. Local executor fallback when Docker is unavailable. Structured JSON output from check execution. Supports language-specific check recipes (gofmt, go test -json, govulncheck). Configurable via runner section in config.yaml.

---

## Configuration System

YAML-based configuration in ~/.temper/config.yaml covering daemon settings, LLM providers, learning contract tracks, and runner configuration. Separate secrets storage for API keys. Default configuration generation on init. Learning contract tracks with max_level and cooldown. Multi-tenancy scoped by org_id with strict middleware enforcement.

---
