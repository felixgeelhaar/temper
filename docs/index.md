# Temper

**Adaptive AI pairing tool for learning complex crafts through deliberate practice**

> This product is not about doing work faster.
> It is about protecting and scaling human judgment in the age of AI.

## What is Temper?

Temper is an AI pairing tool that helps you learn programming through restrained, adaptive assistance. Like tempering steel, it strengthens your skills through controlled, measured guidance that adapts to your level.

Unlike traditional AI coding assistants that race to complete your code, Temper:

- **Leads when you're stuck** - Provides hints and guidance to unblock you
- **Pairs during learning** - Works alongside you at an appropriate level
- **Steps back as you grow** - Reduces assistance as your skills improve

## Key Features

### Learning Contract

Temper enforces a "Learning Contract" with progressive intervention levels:

| Level | Type | Description |
|-------|------|-------------|
| L0 | Clarifying | Questions only |
| L1 | Hint | Direction to explore |
| L2 | Nudge | Location + concept |
| L3 | Outline | Constrained snippets |
| L4 | Partial | Partial solutions (gated) |
| L5 | Solution | Full solutions (rare) |

### Structured Exercises

41+ exercises across Go, Python, TypeScript, and Rust:

- Beginner to advanced difficulty
- Clear learning objectives
- Progressive hints at each level
- Automated code checking

### Editor Integration

Works with your favorite editor:

- **VS Code** - Full-featured extension
- **Neovim** - Lua plugin with Telescope integration
- **Cursor** - MCP server integration

### Progress Tracking

Track your learning journey:

- Skill progression by topic
- Error pattern analysis
- Hint dependency trends
- Evidence-based appreciation

## Quick Start

```bash
# Install via Homebrew
brew install felixgeelhaar/tap/temper

# Initialize
temper init

# Start the daemon
temper start

# Begin an exercise
temper exercise start go-v1/basics/hello-world
```

## Philosophy

### AI Restraint is a Feature

Understanding beats speed. Temper intentionally limits how much help it provides because struggling with problems is how you learn.

### User Remains the Author

You write the code. Temper provides guidance, hints, and feedback—but the implementation is always yours.

### Progression is Earned

Temper tracks your progress and adapts its assistance level based on demonstrated understanding, not user preference.

## Next Steps

- [Installation Guide](installation.md) - Get Temper installed
- [Quick Start](quickstart.md) - Start your first exercise
- [Learning Contract](learning-contract.md) - Understand the intervention system
- [CLI Reference](cli-reference.md) - Full command documentation
