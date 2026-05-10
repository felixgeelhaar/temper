# Temper Pairing Evaluation Harness

A golden-set evaluation framework for the pairing system. Catches regressions
in selector logic and prompt restraint when the model, prompts, or rule
table change.

## What it does

For each case in `eval/cases/*.yaml`:

1. Builds the same prompt the daemon would build (`pairing.Prompter`).
2. Sends it to a configured LLM provider (Claude / OpenAI / Ollama).
3. Validates the response against:
   - `pairing.ClampValidator` (the production output-side rule table — same
     code path as `temperd`).
   - Per-case `must_contain_question`, `required_substrings`,
     `forbidden_substrings`.
4. Aggregates pass-rate and clamp-violation count.

## Running

```bash
# One-shot:
make eval

# With a specific provider:
./bin/eval-harness -dir eval/cases -provider claude

# Lower the threshold while iterating on prompts:
./bin/eval-harness -threshold 0.6
```

The harness reads `~/.temper/secrets.yaml` for API keys, exactly like
`temperd`. Set `claude.enabled: true` and provide an API key in `secrets.yaml`
to run against Claude.

## Case format

```yaml
id: short_unique_id
description: |
  What the case verifies.
intent: hint        # hint|review|stuck|next|explain
level: 1            # expected level (0–5)
exercise_id: go-v1/basics/hello-world
language: go
code:
  main.go: |
    package main
    ...
build_errors: []     # optional
tests_passed: 0
tests_failed: 0
profile:             # optional
  hint_dependency: 0.5
  total_runs: 20
expect:
  clamp_passes: true
  must_contain_question: false
  required_substrings: []
  forbidden_substrings: []
```

## Roadmap

- LLM-as-judge for content quality (currently substring-based heuristics
  only).
- Expand corpus to ~50 cases covering all 6 supported languages.
- CI gate: pass-rate threshold blocking PR merges.
- Retry-budget metrics: average retries per case (clamp validator triggers
  retries on violation).
