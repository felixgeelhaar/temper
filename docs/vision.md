# Product Vision Document

**Product:** Temper — Learning-First AI Pairing

---

## 1. The Problem (From First Principles)

High-quality software work is learned through practice, feedback, and judgment.
Yet modern tools force a damaging tradeoff:

**Traditional learning** (docs, courses, tutorials)
→ slow feedback, poor context, happens outside real work

**Generative AI** (copilots, agents, one-shot generators)
→ fast output, but collapses the learning loop by doing the work for you

As a result:
- Beginners become dependent on solutions they don't understand
- Experienced developers lose opportunities to sharpen judgment
- Learning happens outside real projects, not within them
- Tools lack restraint, context, and respect for authorship

The industry is optimizing for output while silently eroding craft.

---

## 2. The Vision

### Vision Statement

**Temper helps developers learn by shipping real work — without surrendering authorship to AI.**

We envision a world where AI:
- Protects learning instead of bypassing it
- Guides through real-world tasks, not isolated exercises
- Uses structured specs, deliberate feedback, and restrained assistance
- Measures progress by independence, clarity, and judgment — not speed

---

## 3. Who This Is For

### Primary: The Practicing Developer

- Uses Neovim, VS Code, or Cursor
- Works in real repositories (open source or company code)
- Wants to learn while building, not in isolation
- Values autonomy, quality, and long-term skill growth

This includes:
- Junior developers learning fundamentals
- Mid-level engineers expanding scope
- Senior/staff engineers refining judgment and architecture skills

### Secondary: The Self-Directed Learner

- Uses exercises or greenfield projects
- Wants deliberate practice without "AI doing it for me"
- Values progress visibility and earned confidence

---

## 4. The Learning Contract (Core Belief)

This product enforces a learning contract:

| User State | AI Behavior |
|------------|-------------|
| No mental model | May lead with questions and hints |
| Learning | Must pair — guide, not generate |
| Capable | Must step back — observe, not intervene |

**The human remains the author at all times.**

This contract is policy-enforced, not prompt-based.

---

## 5. Product Principles (Non-negotiable)

| Principle | Description |
|-----------|-------------|
| **User remains the author** | AI never silently writes code |
| **Learning over output** | Faster code is not success; deeper understanding is |
| **Restraint is enforced** | Guardrails are policy-based, not optional |
| **Spec-driven work** | Clear intent precedes implementation |
| **Language agnostic** | The system reasons about work, not syntax |
| **Local-first and private** | No mandatory cloud, no silent data extraction |

---

## 6. North Star Outcome

> "I understand why this works — and can apply it elsewhere."

### Success signals:
- Reduced dependency on hints over time
- Faster convergence without escalation
- Ability to solve novel problems independently
- Confidence that is earned, not given

---

## 7. Why We Win

Incumbents are structurally constrained:
- Productivity tools must optimize for speed
- Content platforms optimize for completion
- Generators optimize for plausibility

**None can withhold help by design.**

Our differentiation is:
- **Behavioral** — AI restraint as a feature
- **Philosophical** — Learning over output
- **Systemic** — Policy-enforced, not prompt-based

This makes it hard to copy without breaking existing products.

---

## 8. What We're NOT

- ❌ An autonomous coding agent
- ❌ A speed/volume optimizer
- ❌ A replacement for code reviews or human mentorship
- ❌ A chat-first experience
- ❌ A gamified learning platform (no streaks, points, leaderboards)

---

## 9. The Experience

Temper runs as a local daemon and integrates into your editor.

There is:
- No required web UI
- No mandatory SaaS account
- No chat-first experience

You work in your editor. Temper guides from within.
