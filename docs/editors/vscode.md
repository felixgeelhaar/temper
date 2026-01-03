# VS Code Extension

Full-featured VS Code extension for Temper.

## Installation

### From VS Code Marketplace

1. Open VS Code
2. Go to Extensions (`Cmd+Shift+X`)
3. Search for "Temper"
4. Click Install

### From VSIX

Download from [GitHub Releases](https://github.com/felixgeelhaar/temper/releases):

1. Download the `.vsix` file
2. Open Command Palette (`Cmd+Shift+P`)
3. Run "Extensions: Install from VSIX..."
4. Select the downloaded file

## Requirements

- VS Code >= 1.80
- Temper daemon running (`temper start`)

## Features

- Session management
- All intervention commands
- Code execution (format, build, test)
- Spec-driven development
- Progress analytics
- Patch management
- Webview panel with markdown rendering
- Activity bar with exercise browser

## Commands

| Command | Keybinding | Description |
|---------|------------|-------------|
| Temper: Start Session | | Start new session |
| Temper: Get Hint | `Cmd+Shift+H` | Request hint |
| Temper: Run Checks | `Cmd+Shift+R` | Run format/build/test |
| Temper: Toggle Panel | | Open/close panel |

See full list in Command Palette (`Cmd+Shift+P` → "Temper").

## Configuration

```json
{
  "temper.daemon.host": "127.0.0.1",
  "temper.daemon.port": 7432,
  "temper.learningTrack": "practice",
  "temper.autoRunOnSave": false
}
```

## Activity Bar

Click the Temper icon in the activity bar to see:
- **Session**: Current session info
- **Exercises**: Browse and start exercises
