# CLI Reference

Complete reference for all Temper commands.

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file |
| `--verbose` | Enable verbose output |
| `--json` | Output in JSON format |
| `--help` | Show help |

## Commands

### Core

#### `temper init`
Initialize Temper configuration.

```bash
temper init [--force]
```

#### `temper start`
Start the Temper daemon.

```bash
temper start [--daemon] [--port PORT]
```

#### `temper stop`
Stop the running daemon.

```bash
temper stop
```

#### `temper status`
Show daemon and session status.

```bash
temper status [--json]
```

#### `temper doctor`
Run diagnostic checks.

```bash
temper doctor
```

### Sessions

#### `temper exercise list`
List available exercises.

```bash
temper exercise list [--pack PACK]
```

#### `temper exercise start`
Start an exercise session.

```bash
temper exercise start PACK/EXERCISE [--max-level LEVEL]
```

#### `temper exercise info`
Show current exercise details.

```bash
temper exercise info
```

### Pairing

#### `temper hint`
Request a hint (L1).

```bash
temper hint
```

#### `temper review`
Request code review (L2).

```bash
temper review
```

#### `temper stuck`
Signal you're stuck (L2-L3).

```bash
temper stuck
```

#### `temper next`
Ask what to do next.

```bash
temper next
```

#### `temper explain`
Request explanation of a concept.

```bash
temper explain [TOPIC]
```

#### `temper escalate`
Request higher intervention (L4/L5).

```bash
temper escalate LEVEL "JUSTIFICATION"
```

### Code Execution

#### `temper run`
Run format, build, and test checks.

```bash
temper run [--format] [--build] [--test]
```

#### `temper format`
Format current file.

```bash
temper format [FILE]
```

### Specifications

#### `temper spec create`
Create a new spec.

```bash
temper spec create NAME
```

#### `temper spec list`
List specs in workspace.

```bash
temper spec list
```

#### `temper spec validate`
Validate spec completeness.

```bash
temper spec validate [PATH]
```

#### `temper spec status`
Show spec progress.

```bash
temper spec status [PATH]
```

#### `temper spec lock`
Generate SpecLock.

```bash
temper spec lock [PATH]
```

#### `temper spec drift`
Check for spec drift.

```bash
temper spec drift [PATH]
```

### Patches

#### `temper patch preview`
Preview pending patch.

```bash
temper patch preview
```

#### `temper patch apply`
Apply pending patch.

```bash
temper patch apply
```

#### `temper patch reject`
Reject pending patch.

```bash
temper patch reject
```

#### `temper patch list`
List session patches.

```bash
temper patch list
```

#### `temper patch log`
View patch audit log.

```bash
temper patch log [--limit N]
```

### Analytics

#### `temper stats`
Show learning statistics.

```bash
temper stats [overview|skills|errors|trend]
```

### Configuration

#### `temper config show`
Show current configuration.

```bash
temper config show
```

#### `temper provider set-key`
Set LLM provider API key.

```bash
temper provider set-key [PROVIDER]
```
