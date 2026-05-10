# Model Matrix

| Provider  | Default model       | Streaming | Cache control |
|-----------|---------------------|-----------|---------------|
| Anthropic | `claude-sonnet-4-6` | yes       | yes (ephemeral) |
| OpenAI    | `gpt-4o`            | yes       | n/a (auto-cached server-side for >1k tokens, no per-block control) |
| Ollama    | `llama2` (override per host) | yes | n/a (local, no caching cost) |

## Why claude-sonnet-4-6 by default

- Latest stable Claude 4.x Sonnet release with native prompt-caching
  support and competitive cost on the BYOK path.
- Pricing puts a typical hint round-trip well under $0.01/user/day with
  prompt caching enabled (system prompt + exercise context cached).
- Claude 4.7 Opus is preferred only for L4/L5 escalations once
  level-based model routing lands (tracked in roady).

## Level-based routing

L0/L1 hints don't need flagship reasoning. L4/L5 escalations benefit
from it. Per-level model routing trades cost for capability:

```yaml
llm:
  default_provider: claude
  level_models:
    "0": claude-haiku-4-5     # clarifying questions
    "1": claude-haiku-4-5     # category hints
    "2": claude-sonnet-4-6    # location + concept
    "3": claude-sonnet-4-6    # constrained snippet
    "4": claude-opus-4-7      # partial solution (gated)
    "5": claude-opus-4-7      # full solution (rare)
```

Estimated savings on the highest-frequency hint paths (L0/L1):
roughly 4–5x cheaper input tokens, ~5x cheaper output. Keys "0"–"5"
or "L0"–"L5" are both accepted. A missing key falls back to the
provider's configured default model.

The chosen model is recorded in `Intervention.Rationale` for
transparency: `"…model=claude-haiku-4-5"`.

## Prompt-caching strategy

Anthropic's `system` field accepts an array of content blocks, each
optionally tagged with `cache_control: {type: "ephemeral"}`. Everything
before (and including) a cache breakpoint is cached for ~5 minutes,
shaving ~90% off the per-call input-token cost when the cached prefix
is reused.

Temper sends two stable prefixes:

1. **Level system prompt** — same text per (level, intent), reused
   across every hint/review/stuck request in a session. Marked with
   `cache_control: ephemeral`.
2. **(Future)** Exercise + spec context — stable across multiple hint
   requests on the same exercise. Will become the second cache
   breakpoint when wired (tracked as task-level-based-llm-model-routing
   prerequisite).

User code is **not** cached — it changes every keystroke.

## Internals

`llm.Request.SystemBlocks []SystemContentBlock` carries the structured
form. When non-empty, `claude.go` serializes it as Anthropic's array
shape; when empty, falls back to the legacy `Request.System` string.
OpenAI and Ollama collapse blocks into a single concatenated system
message via `flattenSystemBlocks` (no native cache control).

## Verifying cache hits

After running `temper hint` repeatedly in the same session:

```
$ ANTHROPIC_LOG=debug ./temperd 2>&1 | jq 'select(.usage)'
```

Look for `usage.cache_read_input_tokens > 0` after the first call. The
first call always shows `cache_creation_input_tokens > 0` (priming).
