# Dependency Security

Dependency security on this repo is owned by **Nox** (`nox scan`),
not Dependabot. Dependabot version-updates are explicitly disabled in
`.github/dependabot.yml`; the security-alert path inside GitHub may
still surface advisories but no auto-PR is created.

## Why Nox

Three reasons:

1. **One source of truth.** Nox runs the same scan locally
   (`make security` / `nox scan`) and in CI
   (`.github/workflows/security.yml`). Dependabot only ran in the
   cloud; local devs had no parity.

2. **One config.** The baseline (`.nox/baseline.json`) and the
   ignore file (`.noxignore`) are committed to the repo. Nothing
   important lives in repo settings UI.

3. **Multi-domain rules.** Nox covers dependency CVEs (VULN-001),
   typosquatting (VULN-002), AI-specific risks (AI-006/AI-049),
   IaC misconfig (IAC-351), and secrets (SEC-* rules from gitleaks).
   Dependabot covered only dependency CVEs.

## Workflow

### Local

```bash
nox scan            # Full repo scan
nox list-findings   # Walk findings (severity / rule / file filters)
nox fix-plan        # Read-only plan for VULN-001 upgrades
nox fix             # Apply the plan
```

### CI

`.github/workflows/security.yml` runs `nox scan` on every push,
every PR, and weekly on Sunday 03:00 UTC. The job exits non-zero on
any new critical/high finding outside the baseline. Reviewers see
the PR-annotation comment with details.

## Suppressing false positives

Nox baseline (`.nox/baseline.json`) records fingerprints we have
inspected and consciously chosen to ignore. Each entry carries a
human-readable `reason` so future reviewers know why.

```bash
# After investigating finding, add it:
nox baseline-add <fingerprint> --reason "<short justification>"
```

For path-level exclusions (e.g. lockfiles where every secret regex
hits sha512 integrity hashes), use `.noxignore`. Format is
gitignore-style. Document each entry inline.

## Real findings handled in this commit

When Nox first ran on the repo it produced 3940 findings. Most were
lockfile false positives. After triage:

| Action | Count | Notes |
|--------|-------|-------|
| Removed (real fix) | dead `internal/workspace/` | Postgres scaffolding, never wired. |
| Removed (real fix) | dead `internal/config/config.go` | Multi-service config struct, never wired. |
| Rewrote `.env.example` | 1 SEC-073 | Removed DATABASE_URL/RABBITMQ_URL stale credentials. |
| `.noxignore` (lockfiles + demo + .env.example) | ~3000 | sha512 integrity hashes false-match every secret rule. |
| Baseline (false positives) | ~10 | Fortify Execute pattern, sandbox-by-design, test fixtures, model name strings. |

After triage: 0 critical, 0 high, ~880 medium remaining (mostly
AI-006/AI-008/AI-026 false positives on model name strings in test
fixtures and CLI print statements; tracked as a follow-up triage
pass).

## Plugin

`mcp__nox__plugin_install` installs registered plugins. Adopting a
plugin is a fresh-code-on-disk operation; require explicit
`confirmed: true` on the call. Document the chosen plugin name and
version here when one is adopted.

## Replacing a closed Dependabot PR

The five conflicting Dependabot PRs (#3, #11, #18, #20, #24) were
closed when Nox took over. To pick up the same dependency bump:

```bash
go get -u <module>@<version>      # Go bumps
cd web && npm update <pkg>         # npm bumps
cd editors/vscode && npm update <pkg>
```

Then run `nox scan` to confirm no new VULN-001 findings, commit,
PR, and let CI re-validate.
