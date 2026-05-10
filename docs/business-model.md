# Business Model

MIT + BYOK with no revenue path is fine for the v1 OSS launch but
becomes a sustainability problem within 12 months. This doc commits
to a v2 monetization direction so future feature work (sandboxes,
team policies, web progress views) is designed coherently rather
than retrofitted later.

## The chosen direction: open-core team plan

> **OSS forever.** Single-user, single-machine Temper stays free,
> MIT, BYOK. No feature is gated behind a subscription on the
> single-user path.
>
> **Paid: team learning policies + admin dashboard + hosted
> exercise registry.** Organizations pay per-seat for shared
> tracks, cohort analytics, and curated/custom exercise packs.

Why this option, ranked against the alternatives:

| Option | Revenue ceiling | Risk to OSS promise | Fits product DNA |
|--------|-----------------|---------------------|------------------|
| **Open-core team plan (chosen)** | High (B2B SaaS pricing) | Low — OSS path stays free | High — team policies are an obvious extension of single-user contracts |
| Hosted runner / cloud sandboxes | Medium | Medium — pushes against local-first | Low — undermines the privacy story |
| Paid exercise marketplace | Low–Medium | Low | Medium — adjacent but not core |
| Pure OSS + sponsorship | Low | Zero | Indifferent |
| Corp upskilling / enterprise consulting | High but lumpy | Low | Medium — drags focus to services |

Open-core team plan wins on revenue ceiling without compromising
the local-first, BYOK promise that defines the product. Single-user
contributors and learners pay nothing, ever.

## Paid tier: what's in scope

The paid offering is **team-shaped**, not feature-shaped. Individual
features must continue to ship to OSS. Paid value comes from
multi-user coordination:

1. **Team learning contracts.** A team admin defines tracks
   (`practice`, `interview-prep`, custom) once and applies them to
   the cohort. Members inherit. Today everyone reconfigures
   `~/.temper/config.yaml` independently.

2. **Cohort analytics dashboard.** Aggregated stats across the
   team: who is escalating most, which topics show team-wide gaps,
   which exercises consistently fail. Source data is the same
   `temper stats export` JSONL, aggregated server-side. Individual
   stats stay local; only aggregates leave each machine, on opt-in.

3. **Curated/custom exercise packs.** Hosted catalog with
   versioning, sharing, and access control. OSS users build packs
   in YAML and host themselves; paid teams get a managed registry
   with per-team visibility.

4. **Spec governance.** Org-level spec templates, drift alerts
   delivered to Slack/email, lock-file review across PRs. Single
   user uses `temper spec lock` locally; teams need rollups.

5. **Single sign-on, audit log, role-based access.** Standard
   B2B-SaaS table stakes for compliance.

## What stays free

Forever:

- Daemon, CLI, all editor integrations.
- All 6 language packs and their 85 exercises (current).
- All intervention levels (L0–L5), per-topic clamp, profile
  tracking.
- The eval harness, all SLO instrumentation, the metrics endpoint.
- BYOK Claude / OpenAI / Ollama access.
- `temper stats export` JSONL — users can roll their own dashboards.

Anything an individual learner or solo developer needs today, they
get tomorrow at zero cost.

## Pricing hypothesis (testable)

Initial guess for testing:

- **Team plan**: $9/seat/month, 5-seat minimum.
- **Annual**: $90/seat/year (savings = ~17% vs monthly).
- **Education / OSS-team discount**: 50% off, no minimum.

These numbers are placeholders. The first 10 design partners will
calibrate them. Anchors:

- Below GitHub Copilot Business ($21/seat/month): we are not faster
  but we are differentiated on learning outcome.
- Above DataDog DevSec teamlets ($5/seat): we offer engineering-
  outcome data, not log routing.

## Implementation prerequisites

The features below must exist (and be shipped to OSS) before the
paid tier can sell:

- [x] Output-side clamp validator + SLO (commit 0f77961).
- [x] Stats export JSONL (commit 1518652).
- [x] Bearer-token auth + DNS-rebind guard (commit d5a85da).
- [ ] OpenAPI spec (HIGH-priority follow-up task) — required before
      a hosted control plane can integrate.
- [ ] Multi-user data model in domain layer — currently uuid.Nil.
- [ ] Tenant-aware storage layer — currently single-tenant SQLite.

A Postgres + queue stack can return as a deliberate v2 decision at
this point — the dead-infrastructure cleanup (commit 8aa265d)
removed it because it was unused, not because it was wrong. Restore
from git history when the team plan is ready to sell.

## Decision points and triggers

- **When to start building the paid tier**: when ≥ 50 GitHub stars,
  ≥ 5 design partners requesting team features unprompted.
  Building before that is premature.
- **When to incorporate**: when first paying customer signs.
  Not before.
- **When to hire**: when MRR covers a single FTE comfortably for
  18 months at conservative growth.

These are decision triggers, not promises. Adjust as data lands.

## What this doc is NOT

- Not a financial projection. Numbers above are hypotheses, not
  forecasts.
- Not a commitment to specific features. The five paid features
  listed are the most-promising candidates today; design-partner
  feedback may reorder them.
- Not an exit plan. The OSS-forever single-user promise rules out
  a category of acquirers; that's deliberate.
