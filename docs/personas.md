# Personas × Features

The vision and PRD list four personas spanning beginner → senior. The
feature surface skewed toward juniors before this matrix existed; senior
flows under Feature Guidance + spec were under-articulated.

This doc maps each persona to: which session intent they live in, the
features they use weekly, the intervention levels they typically receive,
and what success looks like. Use it when scoping new features ("does
this serve more than one persona?") and when triaging cuts.

## Quick reference

| Persona | Primary intent | Weekly features | Typical levels | Success signal |
|---------|---------------|-----------------|----------------|-----------------|
| Junior  | Training | exercises, hint, explain, escalate | L1–L3 | Completes a pack without escalating to L4/L5 |
| Mid     | Greenfield + Training | hint, review, run, stats | L1–L2 | Hint dependency drops over 30 days |
| Senior  | Feature Guidance | spec create/lock/drift, review, risk, run | L0–L1 | Specs anchor real PRs; clamp violations stay near zero |
| Self-directed learner | Training | exercises, stuck, escalate, stats export | L1–L3 | Reduced stuck-frequency over time |

## 1. Junior developer

**Profile**: First 0–2 years of professional coding. Typically following a
boot-camp curriculum or contributing to OSS for the first time.

**Primary intent**: `Training`.

**Weekly features**:
- `temper exercise list` / `start` — guided, structured practice
- `temper hint` and `temper stuck` — frequent
- `temper explain` for unfamiliar concepts
- `temper escalate 4 "<reason>"` — occasional, after honest attempts
- `temper stats trend` — to see whether dependency is going down

**Typical levels**: L1–L3. Heavy use of L3 outline + placeholder. L4/L5
gated by explicit escalation.

**What success looks like**:
- Completes the `basics` and `intermediate` categories of a pack
  without escalating to L4 most of the time
- Hint dependency below 40% by week 4
- Self-correctly identifies when to escalate vs when to keep struggling

**Risks for this persona**:
- Over-reliance on hints — the cooldown timer is the safeguard
- Skipping exercises because the level clamp feels stingy — the
  per-topic clamp adjusts down only when skill is high (>0.7), so
  early users see the full L3 ceiling

## 2. Mid-level engineer

**Profile**: 2–6 years experience. Comfortable in their primary
language; expanding scope (new framework, new domain, new role).

**Primary intent**: `Greenfield` for new repos; `Training` to fill gaps.

**Weekly features**:
- `temper hint` (rare; usually L1)
- `temper review` against own diff
- `temper run` for fast iteration
- `temper exercise` for targeted skill gaps (e.g. concurrency)
- `temper stats skills` to identify topics under 0.5

**Typical levels**: L1–L2. Selector decay to L0 when all tests pass is
common because they self-resolve quickly.

**What success looks like**:
- Hint dependency under 20% over a 30-day window
- Topic skill scores climb in two new topics per quarter
- Smaller, safer diffs as measured by run output

## 3. Senior / staff engineer

**Profile**: 6+ years. Reviewing more than coding. Concerned with
architecture, judgment, mentoring, scope.

**Primary intent**: `Feature Guidance` — specs anchor PRs that move
through real review.

**Weekly features**:
- `temper spec create` / `validate` / `lock` / `drift`
- `temper review` against spec for design feedback
- `temper run` (rarely; CI is the source of truth)
- Risk notices via the runner's risk detector
- `temper stats export` to share aggregated cohort data

**Typical levels**: L0–L1. Clarifying questions and category hints
dominate. L4/L5 effectively never used; the senior has the answer
already and wants the system to surface trade-offs, not solutions.

**What success looks like**:
- Specs lock cleanly and survive without drift across multi-week
  feature work
- Clamp violation rate near zero on this persona's hint requests
  (model rarely tempted to over-help because the prompt + level
  preserve restraint by design)
- Risk notices catch one or two real issues per quarter

**Gap to close**: today's CLI commands are tilted toward Junior. The
spec authoring + drift workflow needs better surfaces in editors;
tracked under "Editor surfaces for --why rationale" + "Editor
clients send daemon bearer token".

## 4. Self-directed learner

**Profile**: Career-changer, hobbyist, returning developer. Uses
exercise packs as a curriculum rather than as practice.

**Primary intent**: `Training`, occasionally `Greenfield`.

**Weekly features**:
- `temper exercise` flow with offline Ollama default (zero-config)
- `temper stuck` and `temper explain`
- `temper escalate 4` after honest attempts
- `temper stats export` for personal record-keeping

**Typical levels**: L1–L3. Same as Junior, but without the external
scaffolding of a boot camp curriculum.

**What success looks like**:
- Completes a full pack in 4–8 weeks
- Stuck-frequency declines visibly between week 1 and week 4
- Earns the higher-level packs (advanced category) without
  short-circuiting via L5 escalations

## How to read the matrix

When scoping a new feature, this is the first artifact to consult:

1. List the personas the feature serves.
2. If only one — is it the persona we are still under-serving (Senior)?
   If yes, prioritize. If no (yet another Junior feature), ask why.
3. Check the typical levels. A feature that only matters at L4/L5
   is rare and should be justified explicitly; restraint-as-a-feature
   means most value is delivered at L0–L3.

When triaging cuts: features that no persona uses weekly are
candidates for removal, not feature-flagging. The product is small;
keep it that way.
