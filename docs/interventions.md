# Interventions

Interventions are AI-generated assistance at varying levels of detail.

## Requesting Help

| Command | Level | Description |
|---------|-------|-------------|
| `temper hint` | L1 | Direction to explore |
| `temper review` | L2 | Code review with specific feedback |
| `temper stuck` | L2-L3 | Help when truly stuck |
| `temper next` | L1-L2 | What to do next |
| `temper explain` | L1-L2 | Concept explanation |
| `temper escalate 4` | L4 | Partial solution (gated) |
| `temper escalate 5` | L5 | Full solution (rare) |

## Intervention Flow

1. You request help
2. Temper analyzes your code and context
3. Temper selects appropriate intervention level
4. Response generated within contract limits
5. Intervention recorded in session history

## Cooldown

After receiving help, there's a cooldown period before requesting more.
Default: 60 seconds (configurable).
