
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

## Output-side clamp validator

CRITICAL. Post-process every LLM intervention output to verify it matches the selected intervention level (L0-L5). At L0-L2, reject responses containing code blocks; at L0-L1, reject responses naming specific functions/methods. On violation, retry once with tightened system prompt. Increment clamp_violation_total counter and surface in temper stats. Without this, the core "AI restraint as feature" promise is enforced only by hope. Acceptance: clamp_validator package with rule table per level; integration in pairing.Service.Intervene; unit tests with adversarial samples; metrics counter wired.

---

## Daemon authentication and DNS-rebind guard

CRITICAL. Add bearer-token middleware to all /v1 routes. Token generated at temper init, stored ~/.temper/secrets.yaml chmod 600, included by CLI/editor clients. Verify Host header is in {127.0.0.1, localhost} to defeat DNS-rebinding attacks via malicious browser tabs. Refuse to start daemon if bind != 127.0.0.1 and no token configured. Lock CORS allowlist to known editor origins. Without this, any browser tab can call sandbox/exec endpoints while daemon runs locally.

---

## Prompt-injection mitigation with nonce-fenced delimiters

CRITICAL. User code is concatenated verbatim into LLM prompts using markdown headers, allowing adversarial code (especially from community exercise packs) to escape the user-content boundary and rewrite system instructions. Wrap all user-controlled strings (code, exercise descriptions, spec content) in fenced delimiters with random per-request nonces (e.g., &lt;user_code nonce="a3f9..."&gt;...&lt;/user_code nonce="a3f9..."&gt;). Strip the nonce token from the user content before insertion. Add an adversarial test corpus to pairing/prompter_test.go covering instruction-override attempts.

---

## Selector and prompt evaluation harness

CRITICAL. Build a CI-gated golden-set evaluation harness for the pairing system. Curate ~50 (intent, exercise, code state, profile) → expected_level pairs. Use promptfoo (or custom Go harness) running an LLM-as-judge to score each generated intervention against level constraints. Block PR merges on regressions in: clamp adherence rate, level-match accuracy, content quality. Without this, prompt or model changes silently degrade the restraint guarantee. First milestone: measure baseline clamp violation rate across all 41 existing exercises.

---

## Selector property tests and invariants

HIGH. Replace anecdotal selector tests with property-based invariants using pgregory.net/rapid. Properties: (1) result level is always within [L0, policy.MaxLevel]; (2) result is monotonic in profile.HintDependency (higher dependency never reduces level); (3) Select(Select(x))==Select(x) when context is unchanged; (4) no underflow when adjustForContext decrements at L0. Move underflow guard into InterventionLevel as a smart constructor. Document the deliberate ordering of adjustForContext→adjustForProfile→adjustForRunOutput→adjustForSpec.

---

## Delete dead infrastructure and reconcile architecture docs

HIGH. docs/architecture.md describes Postgres + RabbitMQ + sqlc as runtime dependencies but the daemon imports none of them. internal/queue/ (RabbitMQ) is unused; sqlc-generated files in internal/storage/{models.go,querier.go,*.sql.go} are unused; only internal/storage/sqlite/ and internal/storage/local/ are wired. Delete the unused packages and sqlc.yaml, OR commit to wiring them and document the migration path. Then update docs/architecture.md to match the shipped reality. New contributor onboarding currently fails on this drift.

---

## Update Claude default model and add prompt caching

HIGH. internal/llm/claude.go pins the default model to claude-sonnet-4-20250514 (outdated; latest is claude-sonnet-4-6). Update default and document a model matrix. Add Anthropic prompt caching by emitting cache_control breakpoints on the stable parts of the prompt: system prompt + exercise description + spec context. The user code section remains uncached. Expected impact for BYOK users: 60-80% input-token cost reduction on hint/review/stuck flows where the same context is reused across iterations.

---

## Level-based LLM model routing

HIGH. Route intervention generation by level: L0-L1 → Haiku (clarifying questions, category hints don't need flagship reasoning), L2-L3 → Sonnet, L4-L5 → Opus. Estimated 4-5x cost reduction on the highest-frequency hint paths. Add per-level model config in config.yaml with sane defaults; surface chosen model in Intervention.Rationale for transparency. Acceptance: docs/architecture/model-routing.md, llm.Registry.ForLevel(level), regression tests asserting routing matches config.

---

## Parameterize language in pairing system prompts

HIGH. internal/pairing/prompter.go hardcodes "tutor helping a learner practice Go" in the base system prompt, despite the project shipping exercise packs for Python, TypeScript, Java, Rust, and C. Inject {{language}} from exercise.Language into the system prompt, and adapt example phrases per language. Acceptance: SystemPrompt(level, language) signature; per-language hint/snippet examples in fixtures; tests covering all 6 supported languages. Removes the false "language agnostic" claim until corrected.

---

## OpenAPI spec and generated editor client types

HIGH. Three editor clients (VS Code TS, Neovim Lua, MCP Go) consume the daemon REST API with hand-written request shapes, with no contract enforcement. Author OpenAPI 3.1 spec at docs/api/openapi.yaml as the single source of truth for ~50 routes. Add CI step that regenerates handler request/response stubs (or at minimum JSON Schemas) and asserts no diff. Generate Lua and TypeScript client types from the spec. Eliminates silent breaking-change risk across editor clients when daemon payloads evolve.

---

## Split daemon server.go into handler groups

HIGH. internal/daemon/server.go is 2782 lines with 75 functions and a single Server struct holding 14 fields (service locator anti-pattern). Refactor: server.go retains only lifecycle (NewServer, Start, Shutdown, route registration) ~400 lines; split handlers into handlers_session.go, handlers_pairing.go, handlers_spec.go, handlers_run.go, handlers_sandbox.go, handlers_analytics.go, handlers_authoring.go. Move setupLLMProviders and setupRoutes into a wiring.go composition root taking explicit deps. Reduces review surface and decouples test fixtures.

---

## HTTP client timeouts and connection pooling for LLM providers

MEDIUM. internal/llm/claude.go (and likely openai.go, ollama.go) constructs http.Client{} with no Timeout and no Transport tuning. Long-tail Claude latencies park goroutines and leak file descriptors. Configure: Timeout 120s for non-streaming; Transport with MaxIdleConns 10, MaxIdleConnsPerHost 5, IdleConnTimeout 90s. For streams, use the request context with a derived deadline rather than client.Timeout (which would kill mid-stream). Audit all three providers for the same gap.

---

## Daemon Prometheus or JSON metrics endpoint

MEDIUM. No /v1/metrics endpoint exists; observability is structured logging only. Add minimal counters and histograms: hint_requests_total{intent,level}, clamp_violation_total{level}, llm_latency_ms{provider,level}, runner_duration_ms{language,result}, sandbox_active_count, llm_input_tokens_total{provider}, llm_output_tokens_total{provider}. Expose Prometheus format at /v1/metrics and a JSON variant feeding into temper stats so BYOK users can see per-session token cost. Foundation for SLO tracking.

---

## SLO definitions and clamp violation tracking

MEDIUM. Define explicit SLOs for the restraint-as-feature product: hint p99 latency &lt; 3s; clamp_violation_rate &lt; 0.1% over 30-day window; daemon uptime per active session &gt; 99% excluding user-initiated restarts; runner success rate &gt; 99% on warm Docker. Document SLOs in docs/architecture/slos.md with rationale and measurement source. Wire violation counters from the metrics endpoint feature. Establishes objective fitness for the core promise.

---

## Architecture fitness functions in CI

MEDIUM. No automated architectural governance. Add go-arch-lint (or equivalent) rules enforced in CI: (1) internal/domain imports nothing under internal/ (clean architecture dependency rule); (2) internal/pairing does not import internal/daemon; (3) handler files capped at 100 lines per function (size fitness); (4) cyclomatic complexity threshold on selector.go and spec parser. Block PRs on violation. Treats architecture as a property to maintain, not a phase.

---

## Graceful LLM degradation with YAML hints fallback

MEDIUM. When the LLM circuit breaker opens or the user has no API key, pairing currently returns an error. Exercises already define static hints in YAML (hints.L0, hints.L1, hints.L2, hints.L3). Implement a fallback path that serves a level-appropriate hint from the exercise YAML when the LLM is unavailable, marked clearly as offline-mode. Allows a useful first-run experience without an API key and survives provider outages.

---

## Idempotency-Key support on run creation

MEDIUM. POST /v1/sessions/{id}/runs has no idempotency. Editor retries on a network blip create duplicate Docker exec runs, double-billing the user (CPU time and LLM follow-up). Accept Idempotency-Key request header; cache last-result per (session_id, key) for 5 minutes. Document the behavior in OpenAPI spec. Standard practice for any non-idempotent POST that triggers expensive side effects.

---

## Payload size cap on run requests

MEDIUM. handleCreateRun accepts arbitrary code map[string]string from clients with no size limit. Malicious or buggy client can send unbounded payloads → OOM in daemon and runner. Enforce: max total payload 1 MiB, max file count 50, max single-file size 256 KiB. Return 413 Payload Too Large with structured error code. Same caps apply to sandbox.AttachCode.

---

## Structured error taxonomy with stable error codes

MEDIUM. Daemon errors return {message, error} only. Editor clients distinguish failure modes by parsing error message strings — fragile and breaks when wording changes. Add error_code field with a stable enum (EXERCISE_NOT_FOUND, SPEC_INVALID, CLAMP_VIOLATION, RATE_LIMITED, LLM_UNAVAILABLE, PAYLOAD_TOO_LARGE, SANDBOX_LIMIT_REACHED, UNAUTHORIZED, etc.). Document in OpenAPI spec. Editors switch on code, not message.

---

## Per-topic intervention level clamp from learning profile

MEDIUM. LearningPolicy.MaxLevel is a global ceiling per session, but profile.TopicSkills already tracks per-topic skill (e.g., strong in slices, weak in concurrency). Clamp dynamically: if profile.TopicSkills[topic].Level &gt; 0.7, max_level for that topic drops by 1 (more restrained); if &lt; 0.3, allow current ceiling. Otherwise the Learning Profile is decoration, not policy input. Surface chosen clamp + reason in Intervention.Rationale and the --why output.

---

## Reconcile dual runner executor language coverage

MEDIUM. internal/runner/executor.go LocalExecutor.RunFormat only handles .go files but the project ships exercise packs for Python, TypeScript, Java, Rust, and C. Docker executor has language-specific implementations (executor_python.go, executor_typescript.go, etc.). Either: (a) delete LocalExecutor and force Docker (clearer story), or (b) implement language-aware dispatch matching Docker. Today’s silent semantic divergence means non-Go exercises behave differently when AllowLocalFallback is on. Pick one and document.

---

## Tag integration tests with build constraint

MEDIUM. The runner test suite takes 123s because Docker-based integration tests run on every go test ./... invocation. Tag them with //go:build integration so default unit-test runs stay under 10s. Add make targets: make test (unit only), make test-integration (Docker required), make test-all. Update CI to run unit tests on every push and integration on PRs to main. Improves dev loop without sacrificing coverage.

---

## Mutation testing on selector and spec parser

MEDIUM. Test coverage % is high but mutation coverage is unmeasured. Add go-mutesting (or equivalent) on internal/pairing/selector.go and internal/spec/parser.go in CI. Surfaces untested branches in critical decision logic. Threshold: 80% mutation score on these files; PR fails if score drops. Lower priority than property tests but complements them.

---

## End-to-end correlation ID propagation through LLM call

MEDIUM. middleware.go injects correlation_id into request context, but it is not propagated to LLM provider HTTP calls. When debugging "user got wrong intervention," there is no way to correlate a daemon request with an Anthropic API call. Pass correlation_id through pairing.Service.Intervene → llm.Request → provider HTTP headers (X-Request-ID for OpenAI/Ollama; embed in Anthropic metadata.user_id or trace header). Log correlation_id in resilient.go state-change events.

---

## Stats export JSONL for opt-in research cohort

MEDIUM. PRD lists "fewer escalations over time" and "earned confidence" as success metrics, but the project has no telemetry pipeline. Privacy-respecting alternative: temper stats export --since=2026-01-01 emits JSONL of aggregate session metrics (anonymized, no code, no LLM content) that users can voluntarily share for research. Local-only by default. Establishes a measurement loop without compromising the local-first promise.

---

## Reconcile PRD, roadmap, and README

MEDIUM. Doc drift: PRD §11 lists Neovim as Planned but roadmap and README list it Complete. README claims 41 exercises across 3 languages but exercises/ ships 6 packs (go, python, typescript, java, rust, c). Do a content reconciliation pass: single source of truth for status table, accurate exercise counts, label experimental packs as such. Run a docs-vs-code lint in CI to detect future drift (e.g., script that counts exercises in YAML and asserts README number matches).

---

## Persona-by-feature matrix documentation

LOW. Vision and PRD list senior engineers as primary persona, but the feature surface (exercises, escalate L4/L5) skews juniors. Senior workflow under Feature Guidance + spec is underspecified. Author docs/personas.md mapping each persona (Junior, Mid, Senior, Self-Directed Learner) to: which intent (Training/Greenfield/Feature Guidance), which features they use weekly, which intervention levels are typical, what success looks like. Forces explicit articulation and surfaces gaps for senior-engineer workflow.

---

## Editor status-line component for session state

MEDIUM. VS Code, Neovim, and Cursor lack a persistent status indicator showing: current intent, active level clamp, hint cooldown countdown, current track, current exercise. Today users discover state only by running commands. Add a status-line/status-bar component to all three editor integrations querying GET /v1/sessions/{id}/state on a 5s interval. Increases trust and reduces cognitive load.

---

## Why flag and rationale surfacing on every intervention

MEDIUM. domain.Intervention already carries a Rationale field, but selector outputs a generic string ("Selected L%d based on intent=%s, profile signals") and the field is never shown to the user. Build a rich rationale: "L2 chosen because intent=stuck, build errors detected (3), profile.HintDependency=18%, topic=concurrency (skill 0.4), policy clamp=L3." Add temper hint --why and equivalent editor commands. Builds trust by making restraint policy legible.

---

## First-run zero-config wizard with Ollama default

LOW-MEDIUM. Onboarding requires temper init → API key entry → temper doctor → Docker check before the first hint — about 5 friction points. Detect Ollama at localhost:11434 during temper init; if present, default to it and skip API key entry, allowing a 30-second "free first hint" experience. If absent, prompt for BYOK as today. Reduces time-to-first-value and lowers cost barrier.

---

## Web dashboard empty states and loading skeletons

LOW. web/src/pages/ ships dashboard placeholders that fetch /v1/analytics/overview but show "Loading..." indefinitely if the daemon is not running. Add: skeleton loaders for stat cards and charts, empty-state copy when no sessions exist ("Run your first exercise to populate this dashboard"), error state when daemon is unreachable. Run axe-core for WCAG 2.2 contrast and focus-visible states. Either ship the polished version or hide the dashboard route until v1.1.

---

## Category phrase and positioning document

LOW. Temper sits in no clear category — it is not "AI coding," not "edtech," not "katas." Without a category phrase, prospects benchmark it against the wrong alternatives (Copilot speed). Author docs/positioning.md naming the category (proposal: "Deliberate-practice pairing for working developers") with a one-liner, three-pillar value prop, and explicit anti-positioning ("Not Copilot. Not Codecademy. Not a chatbot."). Use the phrase consistently in README, landing page, conference talks, and OSS launch posts.

---

## Anti-Copilot side-by-side demo asset

LOW. Strongest GTM frame is "the first AI pairing tool that helps you learn instead of replacing you," but no demo asset proves it today. Record a 90-second video showing the same exercise solved with Copilot (instant solution, no learning) versus Temper (questions → hints → user writes code → tests pass). Embed in README, landing page, HN/Lobsters launch posts. Single most leveraged GTM asset for the OSS launch.

---

## Monetization v2 plan

LOW. MIT + BYOK has no revenue path, which is fine for v1 OSS launch but needs an explicit v2 plan to avoid sustainability risk. Document the chosen path in docs/business-model.md from these options: (a) hosted exercise registry + curated paid packs, (b) team learning policies + admin dashboard as paid tier, (c) corp upskilling/enterprise plan, (d) pure OSS forever with sponsorship. Pick one direction now; design v2 features (sandboxes, team policies, web progress) consistent with the choice.

---

## Editor clients send daemon bearer token

FOLLOWUP to daemon auth feature. Three editor integrations now break without token: (1) VS Code extension at editors/vscode/src — read ~/.temper/secrets.yaml.daemon.auth_token at activation, attach Authorization: Bearer header to all daemon fetches, surface clear error when token is missing/wrong; (2) Neovim Lua plugin at editors/nvim/lua — same behavior via vim.fn.readfile + plenary.curl; (3) MCP server at internal/mcp — load token from config and attach to its outbound HTTP requests; (4) web dashboard at web/src — read token via daemon-served /v1/dashboard-token endpoint that requires same-origin OR via injected meta tag. Acceptance: smoke test for each editor confirming a hint request succeeds when daemon auth is enabled.

---
