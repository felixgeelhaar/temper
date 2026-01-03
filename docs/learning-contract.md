# Learning Contract

The Learning Contract is Temper's core philosophy: AI assistance should be calibrated to maximize learning, not minimize effort.

## Intervention Levels

Temper uses a 6-level intervention system (L0-L5), where higher levels provide more direct assistance:

| Level | Name | Description | Example |
|-------|------|-------------|---------|
| L0 | Clarifying | Questions only | "What should happen when the input is empty?" |
| L1 | Hint | Direction to explore | "Look at the fmt package for string formatting" |
| L2 | Nudge | Location + concept | "Use fmt.Sprintf with the %s placeholder" |
| L3 | Outline | Constrained snippets | Code skeleton without full solution |
| L4 | Partial | Partial solution | Key function with explanation |
| L5 | Solution | Full solution | Complete working code |

## Default Limits

By default, Temper clamps assistance at L3 for practice mode:

- **Practice track**: L0-L3 (default)
- **Interview-prep track**: L0-L2 (stricter)

## Earning Higher Levels

L4 and L5 interventions are gated behind explicit escalation:

```bash
# Request L4/L5 with justification
temper escalate 4 "I've been stuck for 30 minutes and don't understand the error interface"
```

Requirements:
- Minimum 20-character justification
- Recorded in session history
- Shown in progress analytics

## Why This Matters

### The Problem with Unrestricted AI

When AI provides immediate solutions:
- You skip the struggle that builds understanding
- You become dependent on AI assistance
- You don't develop problem-solving skills

### The Temper Approach

By graduating assistance:
1. You attempt solutions first
2. You get minimal help when stuck
3. You build understanding through guided discovery
4. Full solutions are rare, intentional teaching moments

## Customizing Limits

### Per-Session

```bash
temper start --max-level 2
```

### Per-Pack

Exercise packs define default policies:

```yaml
default_policy:
  max_level: 3
  patching_enabled: false
  cooldown_seconds: 60
```

### Global Configuration

```yaml
# ~/.temper/config.yaml
learning:
  default_track: practice
  tracks:
    practice:
      max_level: 3
      cooldown_seconds: 60
    interview-prep:
      max_level: 2
      cooldown_seconds: 120
```

## Patch Policy

Code patches (actual file modifications) follow strict rules:

1. **No automatic changes** - Temper never modifies your code without permission
2. **Preview first** - See exactly what will change
3. **Explicit apply** - You must confirm each patch
4. **Audit trail** - All patches logged for review

```bash
# Preview pending patch
temper patch preview

# Apply if you agree
temper patch apply

# Reject if not
temper patch reject
```

## Progress Recognition

Temper tracks your learning and provides evidence-based appreciation:

- "Hint dependency reduced by 40% this week"
- "3 exercises completed without L3 assistance"
- "Error rate for 'nil pointer' down from 5 to 1 per session"

No streaks. No points. No leaderboards. Just honest feedback on your progress.
