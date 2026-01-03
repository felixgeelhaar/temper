# Sessions

A session represents your work on a specific exercise or task.

## Creating a Session

```bash
temper exercise start go-v1/basics/hello-world
```

## Session State

Each session tracks:
- Current exercise
- Code snapshots
- Run history
- Intervention history
- Time spent

## Viewing Session Status

```bash
temper status
```

## Ending a Session

```bash
temper stop
```

Sessions are persisted locally in `~/.temper/sessions/`.
