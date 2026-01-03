# Installation Guide

Multiple ways to install Temper based on your platform and preferences.

## Homebrew (macOS/Linux)

The easiest way on macOS and Linux:

```bash
# Add the tap (first time only)
brew tap felixgeelhaar/tap

# Install Temper
brew install temper

# Verify installation
temper --version
```

Or install directly without tapping:

```bash
brew install felixgeelhaar/tap/temper
```

### Updating

```bash
brew upgrade temper
```

## Download Binary

Download pre-built binaries from [GitHub Releases](https://github.com/felixgeelhaar/temper/releases).

### macOS

```bash
# Apple Silicon (M1/M2/M3)
curl -L https://github.com/felixgeelhaar/temper/releases/latest/download/temper_darwin_arm64.tar.gz | tar xz
sudo mv temper /usr/local/bin/

# Intel
curl -L https://github.com/felixgeelhaar/temper/releases/latest/download/temper_darwin_amd64.tar.gz | tar xz
sudo mv temper /usr/local/bin/
```

### Linux

```bash
# x86_64
curl -L https://github.com/felixgeelhaar/temper/releases/latest/download/temper_linux_amd64.tar.gz | tar xz
sudo mv temper /usr/local/bin/

# ARM64
curl -L https://github.com/felixgeelhaar/temper/releases/latest/download/temper_linux_arm64.tar.gz | tar xz
sudo mv temper /usr/local/bin/
```

### Windows

1. Download `temper_windows_amd64.zip` from [releases](https://github.com/felixgeelhaar/temper/releases)
2. Extract the zip file
3. Add the folder to your PATH or move `temper.exe` to a directory in your PATH

## Build from Source

Requires Go 1.23 or later.

```bash
# Clone the repository
git clone https://github.com/felixgeelhaar/temper.git
cd temper

# Build
go build -o temper ./cmd/temper

# Install to $GOPATH/bin
go install ./cmd/temper

# Or move to a location in your PATH
sudo mv temper /usr/local/bin/
```

## Verify Installation

```bash
# Check version
temper --version

# Run diagnostics
temper doctor

# View help
temper help
```

## Post-Installation Setup

### Initialize Configuration

```bash
temper init
```

This creates `~/.temper/` with default configuration.

### Set Up LLM Provider

Temper needs an LLM provider for AI pairing:

```bash
# Set API key for your provider
temper provider set-key

# Or use environment variable
export ANTHROPIC_API_KEY="your-key"
# or
export OPENAI_API_KEY="your-key"
```

### Start the Daemon

```bash
# Start in foreground
temper start

# Start in background
temper start --daemon
```

## Editor Integration

### VS Code

1. Install from [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=felixgeelhaar.temper)
2. Or download VSIX from [GitHub Releases](https://github.com/felixgeelhaar/temper/releases)

### Neovim

Add to your plugin manager:

```lua
-- lazy.nvim
{ "felixgeelhaar/temper", ft = { "go", "python", "typescript" } }

-- packer.nvim
use "felixgeelhaar/temper"
```

See [Neovim Plugin README](../editors/nvim/README.md) for full setup.

### Cursor

Temper includes an MCP server for Cursor integration:

```bash
# Start the MCP server
temper mcp start
```

See [MCP Integration](../editors/cursor/README.md) for setup.

## Troubleshooting

### Command not found

Ensure the binary is in your PATH:

```bash
# Check PATH
echo $PATH

# Add to PATH (add to ~/.zshrc or ~/.bashrc)
export PATH="$PATH:/usr/local/bin"
```

### Permission denied

Make the binary executable:

```bash
chmod +x /usr/local/bin/temper
```

### Daemon won't start

Check if port 7432 is available:

```bash
lsof -i :7432
```

Run diagnostics:

```bash
temper doctor
```

### Missing exercises

Exercises are bundled with the binary. If they're missing:

```bash
# List available packs
temper exercise list

# If empty, reinstall
brew reinstall temper
```

## Uninstall

### Homebrew

```bash
brew uninstall temper
```

### Manual

```bash
# Remove binary
sudo rm /usr/local/bin/temper

# Remove config (optional)
rm -rf ~/.temper
```
