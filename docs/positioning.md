# Positioning

Temper sits in no clear category today. Without a category phrase,
prospects benchmark it against Copilot ("slower than Copilot, must be
worse"), Codecademy ("for beginners, I'm past that"), or chat tools
("just another wrapper around Claude"). This document fixes the
phrase and the framing.

## The category

> **Deliberate-practice pairing for working developers.**

Use the phrase verbatim in the README, the landing page, conference
talks, OSS-launch posts, and the first paragraph of the elevator pitch.
Not "AI coding assistant." Not "edtech." Not "AI tutor." Those
categories already exist and Temper does not want to win in them.

## One-liner

**Temper is the AI pairing tool that helps you learn instead of
replacing you.**

This is the line that goes on the OG image. It works because:

- "AI pairing tool" is the genre.
- "helps you learn instead of replacing you" names a defect in every
  competitor and frames Temper's restraint as the cure.

## Three-pillar value prop

| Pillar | What it means | Concrete proof |
|--------|---------------|----------------|
| **Restraint as a feature** | The AI withholds help by policy, not by accident. | Output-side clamp validator (commit 0f77961); restraint SLO < 0.1% violation rate. |
| **Learning, not output** | Progress is measured by independence, not lines per minute. | Per-topic clamp tightens for confident learners; `temper stats trend` shows hint dependency declining. |
| **Local-first, BYOK** | Your code, prompts, and progress stay on your machine. | Single binary, SQLite, secrets chmod 0600, optional Ollama for zero-cloud workflow. |

When a feature does not advance one of these pillars, it is not a
Temper feature.

## Anti-positioning

State what Temper is **not**, in order. Order matters: each line
should make the previous one easier to remember.

- **Not Copilot.** Copilot completes for you; Temper coaches.
- **Not Codecademy.** Codecademy teaches in isolation; Temper guides
  inside your real editor.
- **Not a chatbot.** Chatbots happily over-explain; Temper holds the
  level clamp.
- **Not an autonomous agent.** Agents act on your behalf; Temper keeps
  you the author at all times.

These four lines are the entirety of the comparison-table content.
Temper does not need a feature checklist to win against any of them
because each is in a different category.

## Use of the phrase

Place the category phrase in these locations exactly:

1. README.md first line under the title.
2. Landing-page hero, immediately above the install command.
3. `temper --help` summary line.
4. Top of every blog post about Temper.
5. First sentence of the OSS-launch HN post.
6. The introduction sentence of the conference talk abstract.

Do not paraphrase. The phrase is short enough that consistency is the
cheapest way to make it stick.

## Audience-specific framings

Same category phrase, different opening sentences depending on who
is reading:

| Audience | Opening sentence |
|----------|------------------|
| HN / Lobsters | "We built an AI pairing tool that's deliberately less helpful than Copilot — and that's the feature." |
| Bootcamp instructors | "An AI pairing tool that won't solve the exercise for your students." |
| Senior engineers | "Spec-anchored pairing that surfaces design trade-offs without proposing the implementation." |
| OSS contributors | "Local-first, BYOK Go binary. Works in Neovim, VS Code, and Cursor. MIT-licensed." |

## What changes if the phrase is wrong

If "Deliberate-practice pairing for working developers" doesn't land,
the data will say so within ~four weeks of the OSS launch. Watch:

- Top words in HN/Lobsters comments — does "learning" or "restraint"
  feature, or only "AI assistant"?
- New-user surveys (one-question form on the landing page) — "in one
  word, what is Temper?"
- Conversion: from landing-page visit to `temper init`. A confused
  category produces low conversion despite high traffic.

If the data argues for a different phrase, change it everywhere
at once. The cost of consistency is low; the cost of three-different-
positioning-lines on three pages is high.

## What this doc does not do

This is not the GTM plan. This is the positioning the GTM plan rests
on. The GTM plan (channels, launch sequence, paid-tier hypotheses)
lives in `docs/business-model.md`.
