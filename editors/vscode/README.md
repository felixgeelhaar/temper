# Temper VS Code Extension

Adaptive AI pairing for learning - VS Code integration.

## Requirements

- VS Code >= 1.80
- `temperd` daemon running locally

## Installation

### From Source

1. Clone the repository
2. Navigate to `editors/vscode`
3. Run `npm install`
4. Run `npm run compile`
5. Press F5 to launch a new VS Code window with the extension

### From VSIX

```bash
code --install-extension temper-0.1.0.vsix
```

## Configuration

Open Settings (Ctrl+,) and search for "Temper":

| Setting | Default | Description |
|---------|---------|-------------|
| `temper.daemon.host` | `127.0.0.1` | Daemon host address |
| `temper.daemon.port` | `7432` | Daemon port |
| `temper.learningTrack` | `practice` | Learning track (`practice` or `interview-prep`) |
| `temper.autoRunOnSave` | `false` | Automatically run checks on file save |

## Commands

Access commands via Command Palette (Ctrl+Shift+P):

| Command | Description | Keybinding |
|---------|-------------|------------|
| `Temper: Start Session` | Start a session with an exercise | |
| `Temper: Stop Session` | End current session | |
| `Temper: Show Status` | Show session status | |
| `Temper: Get Hint` | Request a hint | Ctrl+Shift+H |
| `Temper: Request Review` | Request code review | |
| `Temper: I'm Stuck` | Signal that you're stuck | |
| `Temper: What's Next` | Ask what to do next | |
| `Temper: Explain` | Request an explanation | |
| `Temper: Run Checks` | Run format/build/test | Ctrl+Shift+R |
| `Temper: Format Code` | Format current file | |
| `Temper: List Exercises` | List available exercises | |
| `Temper: Set Learning Mode` | Set track (practice/interview-prep) | |
| `Temper: Check Daemon Health` | Check daemon status | |

## Workflow

1. Start the daemon: `temper start`
2. Open VS Code in your project
3. Command Palette → "Temper: Start Session"
4. Select an exercise pack and enter exercise ID
5. Work on the exercise
6. Use Ctrl+Shift+R to run checks
7. Use Ctrl+Shift+H for hints when needed
8. Command Palette → "Temper: Stop Session" when done

## Features

- **Activity Bar Integration**: Temper icon in the activity bar
- **Status Bar**: Shows current session info
- **Output Channel**: Detailed intervention content and run results
- **Progress Indicators**: Loading feedback during operations
- **Auto-run on Save**: Optional automatic check execution

## Learning Contract

Temper enforces a Learning Contract that limits intervention depth:

| Level | Description |
|-------|-------------|
| L0 | Clarifying questions only |
| L1 | Category hints (direction to explore) |
| L2 | Location + concept (default max for practice) |
| L3 | Constrained snippets/outlines |
| L4 | Partial solutions (gated) |
| L5 | Full solutions (rare) |

Different tracks have different max levels:
- **Practice**: L0-L3
- **Interview Prep**: L0-L2 (stricter)

## Development

```bash
cd editors/vscode
npm install
npm run watch  # Compile in watch mode
# Press F5 to launch Extension Development Host
```

## License

MIT
