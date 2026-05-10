# Service Level Objectives

Temper's "AI restraint as a feature" promise is only credible if it is
measurable. The objectives below are the public contract: if any are
breached for two consecutive 30-day windows, the team treats it as a
priority-zero incident and considers the affected feature path broken.

All SLOs are tracked from the metrics emitted at `GET /v1/metrics`.

## Restraint SLOs

| SLO | Target | Window | Measurement source |
|-----|--------|--------|--------------------|
| Clamp violation rate | < 0.1% | rolling 30 days | `clamp_violations_total` ÷ `hint_requests_total` |
| Clamp retry-success rate | ≥ 95% | rolling 7 days | (retries succeeding without sanitization) ÷ (total clamp violations) — instrumentation pending |
| Output sanitization rate | < 0.01% | rolling 30 days | (sanitize fallback path executions) ÷ `hint_requests_total` — instrumentation pending |

The clamp validator (commit 0f77961) returns a violation, retries once,
then sanitizes if the retry also violates. Restraint is held when:

  1. Most calls never trigger the validator at all (< 0.1%).
  2. When they do, a single retry recovers (≥ 95%).
  3. The sanitization fallback is essentially never the user's
     experience (< 0.01%).

## Latency SLOs

| SLO | Target | Window | Source |
|-----|--------|--------|--------|
| Hint request p99 latency | < 3.0s | rolling 7 days | `llm_request_duration_ms_bucket` (instrumentation pending) |
| Daemon time-to-ready (cold start) | < 5s | rolling 7 days | `daemon_startup_duration_ms` (instrumentation pending) |
| Sandbox exec p95 latency | < 30s | rolling 7 days | `sandbox_exec_duration_ms_bucket` (instrumentation pending) |

## Availability SLOs

| SLO | Target | Window | Source |
|-----|--------|--------|--------|
| Daemon uptime per active session | > 99.0% | rolling 30 days | external monitor against `/v1/health` |
| Runner success rate (warm Docker) | > 99.0% | rolling 30 days | (runs returning result) ÷ (runs created) — `runner_runs_total{result="ok|fail"}` (instrumentation pending) |
| Provider fallback rate | < 5% | rolling 7 days | `offline_fallback_total` ÷ `hint_requests_total` (instrumentation pending) |

User-initiated daemon restarts (via `temper stop` / SIGINT / SIGTERM)
do **not** count against the uptime SLO. Failures from missing API
keys or disabled providers count toward `provider_fallback_rate`, not
toward uptime.

## Cost SLOs (BYOK)

| SLO | Target | Window | Source |
|-----|--------|--------|--------|
| Per-session p95 input tokens (with caching) | < 30k | rolling 7 days | `llm_input_tokens_total{provider}` aggregated by session (instrumentation pending) |
| Per-session p95 output tokens | < 4k | rolling 7 days | `llm_output_tokens_total{provider}` |
| Cache hit rate (Anthropic) | ≥ 70% on intra-session repeat hints | rolling 7 days | `cache_read_tokens_total` ÷ (`cache_creation_tokens_total` + `cache_read_tokens_total`) — instrumentation pending |

Cache hit rate below 70% means level-based routing or the system-prompt
breakpoint regressed; cost on the BYOK path doubles.

## Quality SLOs

| SLO | Target | Window | Source |
|-----|--------|--------|--------|
| Eval-harness pass rate | ≥ 90% | every PR | `bin/eval-harness -threshold 0.9` exit code |
| Adversarial corpus pass rate | 100% | every PR | clamp + injection tests in CI |

The eval harness lives at `eval/cases/`. The adversarial corpus is
defined in `internal/pairing/clamp_test.go` and `internal/pairing/
injection_test.go`. Both are CI-gated.

## What's instrumented today

Metrics already emitted (commit bf4d9aa):

  - `hint_requests_total{intent}` — counter
  - `clamp_violations_total` — counter

Pending instrumentation (tracked as follow-up tasks):

  - LLM token usage per provider + cache hit rate
  - LLM request duration histogram
  - Runner result counters
  - Sandbox active gauge + exec latency histogram
  - Offline fallback counter

## How operators use this

```bash
# Tail the metrics endpoint and compute violation rate.
curl -sH "Authorization: Bearer $(yq '.daemon.auth_token' ~/.temper/secrets.yaml)" \
     http://127.0.0.1:7432/v1/metrics \
  | awk '/^hint_requests_total/  { req += $2 }
         /^clamp_violations/     { v   += $2 }
         END { printf "violations: %d / requests: %d = %.3f%%\n", v, req, (v/req)*100 }'
```

Use Grafana, Datadog, or a similar tool for long-window aggregation.
