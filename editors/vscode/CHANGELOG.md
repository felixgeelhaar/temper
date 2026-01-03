# Change Log

All notable changes to the "Temper" VS Code extension will be documented in this file.

## [0.1.0] - 2026-01-03

### Added

- **Session Management**: Start, stop, and monitor pairing sessions
- **Interventions**: Request hints (L1), reviews (L2), stuck signals (L2-L3), and more
- **Escalation**: Explicit L4/L5 intervention requests with justification
- **Code Execution**: Run format, build, and test checks directly from the editor
- **Specification Support**: Create, validate, lock specs and track progress
- **Analytics**: View learning statistics, skill progression, error patterns, and trends
- **Patch Management**: Preview, apply, and reject AI-suggested code patches
- **Audit Trail**: Local patch audit logging for accountability
- **Webview Panel**: Rich UI for interventions with markdown rendering
- **Activity Bar**: Browse exercises and monitor session status
- **Authentication**: Automatic token-based auth via `~/.temper/auth.token`
- **Keybindings**: `Cmd+Shift+H` for hints, `Cmd+Shift+R` for running checks

### Keybindings

| Key | Command |
|-----|---------|
| `Cmd+Shift+H` / `Ctrl+Shift+H` | Get Hint |
| `Cmd+Shift+R` / `Ctrl+Shift+R` | Run Checks |

### Commands

- Session: Start, Stop, Status, Toggle Panel
- Interventions: Hint, Review, Stuck, Next, Explain, Escalate
- Execution: Run Checks, Format Code
- Specs: Create, List, Validate, Progress, Lock, Check Drift
- Analytics: Overview, Skills, Errors, Trend
- Patches: Preview, Apply, Reject, List, Audit Log, Statistics
