# Product Requirements Document (PRD)

**Product:** Temper
**Version:** v1
**Status:** Approved (post-brainstorming)
**Owner:** Product & Engineering
**Audience:** Founders, Engineering, Design

---

## 1. Product Vision

### Vision Statement

**Temper helps developers learn by shipping real work — without surrendering authorship to AI.**

Temper is a learning-first pairing system that integrates directly into a developer's editor, guiding them through real-world coding tasks using structured specs, deliberate feedback, and restrained AI assistance. Progress is measured by independence, clarity, and judgment — not speed or output volume.

---

## 2. Problem Statement

### The core problem

Modern AI coding tools optimize for producing code, not forming developers.

As a result:
- Beginners become dependent on solutions they don't understand
- Experienced developers lose opportunities to sharpen judgment
- Learning happens outside real projects (tutorials, katas), not within them
- Tools lack restraint, context, and respect for authorship

### Why this matters

Developers don't just need answers — they need:
- guidance
- feedback
- reflection
- confidence that they are actually improving

No mainstream tool enforces this today.

---

## 3. Target Users & Personas

### Primary persona: The Practicing Developer

- Uses Neovim, VS Code, or Cursor
- Works in real repositories (open source or company code)
- Wants to learn while building, not in isolation
- Values autonomy, quality, and long-term skill growth

This includes:
- junior developers learning fundamentals
- mid-level engineers expanding scope
- senior/staff engineers refining judgment and architecture skills

### Secondary persona: The Self-Directed Learner

- Uses exercises or greenfield projects
- Wants deliberate practice without "AI doing it for me"
- Values progress visibility and earned confidence

---

## 4. Product Principles (Non-negotiable)

These principles govern all decisions.

| Principle | Description |
|-----------|-------------|
| **User remains the author** | AI never silently writes code |
| **Learning over output** | Faster code is not success; deeper understanding is |
| **Restraint is enforced, not optional** | Guardrails are policy-based, not prompt-based |
| **Spec-driven work** | Clear intent precedes implementation |
| **Language agnostic by design** | The system reasons about work, not syntax |
| **Local-first and private by default** | No mandatory cloud, no silent data extraction |

---

## 5. Core User Experience (v1)

### High-level experience

Temper runs as a local daemon (`temperd`) and integrates into IDEs.
Users interact entirely from within their editor.

There is:
- no required web UI
- no mandatory SaaS account
- no chat-first experience

---

## 6. Session Model

### Session Intent (explicit & visible)

Every session operates under one of these intents:

| Intent | Description |
|--------|-------------|
| **Training** | Structured exercises and deliberate practice |
| **Greenfield** | Starting new projects or components |
| **Feature Guidance** | Extending an existing codebase |

Intent:
- is inferred automatically
- is always visible
- can be changed at any time

Intent determines:
- guidance style
- intervention limits
- progress interpretation

---

## 7. Spec-Driven Workflow

### Specs are mandatory for feature work

Temper follows Specular's spec format as best practice.

A spec:
- defines intent and acceptance criteria
- constrains scope
- anchors feedback and review
- lives in the repository

Temper:
- helps create and validate specs
- never replaces the spec with chat instructions

---

## 8. Active Pairing Loop (Core Value)

During a session, users work in a loop:

```
1. Request the next step
2. Write code themselves
3. Run checks locally
4. Receive targeted feedback
5. Adjust and continue
```

The system provides:
- hints
- questions
- reviews
- risk notices

It does **not**:
- jump ahead
- generate full solutions by default
- bypass tests or specs

---

## 9. Patch Policy

### Default behavior

- No automatic code changes
- No background edits

### Explicit escalation

If the user explicitly requests help:
1. Temper evaluates policy
2. Only small, scoped patches may be proposed
3. Patches must be previewed and explicitly applied

All patch interactions are logged locally.

---

## 10. Progress & Appreciation

Temper includes progress recognition, not gamification.

### Progress signals

- Reduced dependence on hints
- Faster convergence without escalation
- Improved test discipline
- Smaller, safer diffs
- Better spec alignment

### Appreciation

- Shown sparingly
- Always evidence-based
- Calm, professional tone

Example:
> "You resolved this without escalating beyond hints. That shows growing confidence."

**No:**
- streaks
- points
- leaderboards
- pressure loops

---

## 11. Platform & Integrations (v1)

### IDEs

| Editor | Status |
|--------|--------|
| VS Code | ✅ Extension available |
| Cursor | ✅ MCP server available |
| Neovim | ⏳ Planned |

All IDEs offer a comparable learning experience.

### CLI

- Used only for install, updates, and diagnostics
- Not part of daily workflow

### External tools

Jira, GitHub, CI, etc. are explicitly **out of scope** for v1.

---

## 12. Success Metrics

### Primary success indicators

- Users complete work with fewer escalations over time
- Users request fewer direct solutions
- Users continue using Temper in real projects

### Secondary indicators

- Session length stability (no "rage quitting")
- Positive qualitative feedback on learning confidence
- Adoption by experienced developers (trust signal)

---

## 13. Non-Goals (Explicit)

Temper v1 will **not**:
- act as an autonomous coding agent
- optimize for speed or volume
- replace code reviews or human mentorship
- provide cloud-hosted sandboxes
- integrate with ticketing systems

---

## 14. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Users bypass learning via other AI tools | Temper focuses on being the best learning experience, not enforcement |
| Perceived friction vs Copilot-style tools | IDE-native UX, explicit value framing, strong progress visibility |
| Overengineering early | Local-first, minimal infra, capability-based extension model |

---

## 15. Roadmap Snapshot

### v1 (Current)

- ✅ CLI + temperd
- ✅ IDE integrations (VS Code, Cursor MCP)
- 🔨 Spec-driven feature guidance
- ✅ Language-agnostic core
- ✅ Progress & appreciation (temper stats)

### v2 (Future)

- Sandboxes
- Team policies
- External context providers
- Web-based progress views
