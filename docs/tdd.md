Technical Design Document
Product: Temper — Adaptive AI Pair for Craft Learning
Scope: Coding → PM/Strategy → broader crafts
Approach: Sandbox first → Editor integrations → org adoption
Core Stack: Go, Postgres, sqlc, RabbitMQ
Web: Astro + Vue (TypeScript)
Editor clients: VS Code + Neovim (nvim)
⸻

1. Goals and non-goals
   Goals
   • Deliver a learning-first pairing experience enforcing the Learning Contract:
   • AI may lead when you’re stuck (gated)
   • AI pairs via hints/questions during learning
   • AI steps back into review as skill increases
   • User remains the author
   • Provide a Sandbox for deliberate practice with controlled execution and measurable learning loops.
   • Provide editor integrations (VS Code + Neovim) using a shared protocol and server-side policy enforcement.
   • Provide fast feedback via build/test/lint/basic security checks and feed results into the Pairing Engine.
   • Persist learning profiles and adapt intervention levels automatically.
   Non-goals (v1)
   • Default autonomous agent writing production code end-to-end.
   • Content-first course platform.
   • Perfect skill measurement (v1 uses pragmatic signals + self-report).
   ⸻
2. Product primitives and domain language
   • Artifact: user-created content (code workspace; later docs like PRDs)
   • Exercise: structured task with rubric + check recipe
   • Run: execution of checks producing structured outputs
   • Intervention: AI guidance action (question/hint/nudge/critique/explain/patch proposal)
   • Learning Contract: policy that clamps intervention level and patch permissions
   • Learning Profile: evolving model of skill, dependency, preferences, progress
   ⸻
3. Architecture overview
   Logical components
4. API Gateway (Go)
   Auth/session, rate limits, tenancy enforcement
5. Workspace Service (Go)
   Artifacts, versions, exercise binding, snapshots
6. Runner Service (Go)
   Executes checks in isolated sandboxes (remote) OR ingests local run outputs
7. Pairing Engine Service (Go)
   Minimum-helpful intervention selection + LLM guidance generation
8. Learning Profile Service (Go)
   Signals → profile updates → recommendations → policy caps
9. Exercise Registry Service (Go)
   Exercise packs, rubrics, check recipes
10. Telemetry & Evaluation
    Events, experiments, offline eval, policy compliance tests
    Infrastructure
    • Postgres (system of record)
    • RabbitMQ (async runs, aggregation, telemetry)
    • Object Storage (S3-compatible) for logs/snapshots/patch payloads
    • Redis (optional) for sessions/cache/rate limit
    ⸻
11. Multi-tenancy and security model
    Tenancy
    • org_id on all relevant entities; strict scoping middleware + SQL WHERE clauses.
    Auth
    • Web auth (cookie-based)
    • Editor tokens (token-based)
    • Optional org policies (disable patching, disable content retention, etc.)
    Privacy defaults
    • Default: send diff + relevant chunks, not full repo.
    • .pairignore supported in editor clients.
    • Configurable retention: none / diffs / snapshots.
    ⸻
12. Sandbox-first experience (Horizon 1) — Go-first
    Why sandbox first
    • Reliable enforcement of learning behavior and policies.
    • Controlled execution ensures consistent checks and comparable learning signals.
    • Faster iteration for v1.
    v1 sandbox language: Go
    Rationale
    • Tooling is stable, fast, and “batteries included” (compilation, testing, formatting).
    • Great for deliberate practice (interfaces, concurrency, testing discipline).
    • Easier to package for remote runners.
    Sandbox types
    • Code sandbox (v1): Go exercises + harness
    • Doc sandbox (vNext/H2): PRD/vision rubrics in structured editor
    Execution isolation (remote)
    • Ephemeral container per run
    • CPU/mem/time quotas
    • No outbound network by default (allowlist if needed)
    • Store logs + structured diagnostics; snapshots optional
    ⸻
13. Editor integrations (VS Code + Neovim)
    6.1 Shared integration contract (editor-agnostic)
    Endpoints
    • POST /v1/sessions/start
    • POST /v1/context/snapshot
    • POST /v1/runs/trigger (remote runner)
    • POST /v1/runs/ingest (local outputs)
    • POST /v1/pairing/intervene
    • GET /v1/stream/sessions/{session_id} (SSE)
    Context payload
    • open files (chunks or hashes)
    • selection (file/range)
    • diagnostics (LSP, compiler)
    • diff (git or buffer)
    • intent: hint | review | teach | stuck | next
    • mode: observe | guided | limited_autonomy
    Server remains source of truth for:
    • intervention clamps
    • patch permissioning
    • audit trails
    ⸻
    6.2 VS Code extension (vNext)
    Observe → Guided edits → Limited autonomy (opt-in + audited)
    ⸻
    6.3 Neovim integration (required)
    Lua plugin (thin client) with commands:
    • :PairStart
    • :PairHint
    • :PairReview
    • :PairRun (local or remote)
    • :PairNext
    • :PairApply
    • :PairMode observe|guided|teach
    Rendering:
    • floating windows for interventions
    • virtual text for nudges
    • quickfix list integration
    • optional telescope picker for exercises
    Execution models:
    • Model A (default): local go test, remote guidance
    • plugin runs go test ./... (async)
    • sends outputs + diagnostics via /runs/ingest
    • Model B: remote runner
    • send diff/chunks; remote runner executes
    Patch flow:
    • patch preview in split buffer
    • explicit apply via :PairApply
    • interview-prep mode can disable patching
    ⸻
14. Runner Service (checks & feedback) — Go-first pipeline
    7.1 Check pipeline
    Defined by Check Recipe (declarative):
    • Format
    • Lint (optional in v1)
    • Build/Compile
    • Unit tests
    • Basic security checks (optional; policy-driven)
    7.2 Go pack (v1)
    Default commands
    • gofmt (or gofumpt later if desired)
    • go test ./... with -json output for structured results
    • go test -run TestX for targeted feedback (optional)
    • Optional:
    • govulncheck ./...
    • staticcheck (if you choose to bundle it)
    • golangci-lint (later; heavier)
    Output normalization
    Runner emits structured:
    • diagnostics (file/range/severity/message)
    • tests (name, package, pass/fail, duration)
    • lint issues (optional)
    • security findings (optional)
    • logs_pointer
    7.3 Future packs (post-v1)
    • Rust (cargo check/test/clippy/fmt)
    • TS (tsc/eslint/vitest)
    • Python (pytest/ruff/mypy)
    ⸻
15. Pairing Engine (core)
    8.1 Inputs
    • intent (explicit + inferred)
    • diff/context snapshot
    • runner results
    • exercise rubric
    • learning profile
    • learning contract policy
    8.2 Outputs
    • intervention plan (type/level/targets/rationale)
    • user-facing content (questions/hints/critiques)
    • optional patch pointer (when allowed)
    8.3 Intervention levels
    • L0 clarify
    • L1 category hint
    • L2 location + concept
    • L3 constrained snippet/outline
    • L4 partial solution + explanation (gated)
    • L5 full solution (rare; explicit teach)
    Default clamp in deliberate practice: L2–L3
    8.4 Learning Contract enforcement (hard guardrails)
    Deterministic policy clamps:
    • max level per track/persona
    • cooldown on direct code
    • disable patching for interview-prep tracks
    • audit logs for any patch proposal/accept
    ⸻
16. Learning Profile Service
    9.1 Signals
    • attempts per exercise
    • time-to-green
    • common error categories (compile/test failures)
    • hint frequency and escalation
    • patch request rate
    • dependency proxy
    • comprehension self-report (“I understand why”)
    9.2 Outputs
    • stage per domain topic (e.g., Go interfaces, concurrency, testing)
    • next exercise recommendation
    • suggested default mode + policy caps
    ⸻
17. Exercise Registry and domain plugins — Go-first packs
    10.1 Exercise packs v1 (Go)
    Initial packs (examples):
    • Go basics (types, errors, slices/maps)
    • Testing fundamentals (table tests, mocks minimalism)
    • Interfaces & design (composition, small interfaces)
    • Concurrency (channels, context cancellation)
    • Clean code exercises (naming, structure, refactor with tests)
    • “Debugging under pressure” pack (intentional failing tests)
    Each exercise includes:
    • starter code
    • tests
    • rubric criteria (readability, correctness, idioms)
    • check recipe config
    10.2 Future (H2) doc craft packs
    • PRD critique + rewrite (rubric-based)
    • Vision doc structure (Cagan/Perri aligned)
    • Messaging and GTM narrative drills
    ⸻
18. Data model (Postgres) + sqlc
    11.1 Tables (minimal complete)
    • users, orgs, memberships
    • workspaces, workspace_versions
    • exercises, rubrics, check_recipes
    • runs, run_outputs
    • interventions
    • learning_profiles
    • events
    • api_tokens
    11.2 sqlc patterns
    • Queries scoped per service boundary
    • Strong tenant filters in SQL
    • jsonb for metrics/preferences with typed columns for key fields
    ⸻
19. Async processing (RabbitMQ)
    Queues:
    • runs.execute (remote runner)
    • runs.normalize (parse outputs)
    • profiles.aggregate
    • telemetry.ingest
    • eval.offline
    Consumers are idempotent, schema-versioned, and dedupe by ids.
    ⸻
20. API design (public)
    Key endpoints:
    • Auth: /v1/auth/login, /v1/auth/token
    • Sessions: /v1/sessions/start, /v1/sessions/end
    • Workspaces: /v1/workspaces, /v1/workspaces/{id}/snapshot
    • Runs: /v1/runs/trigger, /v1/runs/ingest, /v1/runs/{id}
    • Pairing: /v1/pairing/intervene, /v1/stream/sessions/{session_id}
    • Learning: /v1/profiles/me, /v1/recommendations/next
    ⸻
21. Web app (Astro + Vue)
    • Astro for marketing/docs
    • Vue islands for interactive app:
    • sandbox editor (Monaco optional)
    • run output panel
    • pairing feed
    • exercise browser
    • profile dashboard
    ⸻
22. Observability, evaluation, safety
    • JSON structured logs, OpenTelemetry traces
    • Metrics: run latency, queue depth, intervention distribution, patch usage
    • Offline eval suites per pack
    • Safety: sandbox hardening, secret redaction, org policy controls
    ⸻
23. Deployment
    • Docker Compose for local dev
    • Kubernetes-ready
    • Runner pools isolated
    • Migration tooling + sqlc generation enforced in CI
    ⸻
24. Milestones (updated: Go-first)
    M0 — Go sandbox MVP
    • Go exercise registry v1 (basics + testing)
    • Remote runner: gofmt + go test -json
    • Pairing engine L0–L3 + policy clamp
    • Basic web UI (Astro + Vue) for run + interventions
    M1 — Profiles + recommendations
    • learning profile v1
    • progression signals + next exercise recommender
    • telemetry pipeline
    M2 — Interview-prep track + constraints
    • interview-prep policies (disable patching, stricter clamps)
    • “reasoning under constraints” packs
    M3 — Editor integrations (observe-only)
    • VS Code extension observe/hint/review
    • Neovim plugin v0.1 observe/hint/review with local go test ingest
    M4 — Guided edits (explicit apply)
    • patch preview/apply for VS Code + nvim
    • audit trail hardened
    M5 — Doc craft pilot (PM)
    • PRD/vision rubrics + critique mode in web sandbox
    • shared learning contract across domains
    ⸻
25. Key decisions (locked)
    • Go + Postgres + sqlc + RabbitMQ
    • Sandbox-first Go deliberate practice v1
    • Shared editor protocol; VS Code + Neovim
    • Astro + Vue for web UX
    • Policy-enforced learning contract outside prompts (hard guardrails)
