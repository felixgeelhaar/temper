# Temper Neovim Plugin

Adaptive AI pairing for learning - Neovim integration.

## Requirements

- Neovim >= 0.8
- `curl` command available
- `temperd` daemon running locally

## Installation

### lazy.nvim

```lua
{
  "felixgeelhaar/temper",
  config = function()
    require("temper").setup({
      -- Configuration options
    })
  end,
  ft = "go", -- Load for Go files
}
```

### packer.nvim

```lua
use {
  "felixgeelhaar/temper",
  config = function()
    require("temper").setup()
  end,
}
```

### Manual

Clone the repository and add to your runtimepath:

```lua
vim.opt.runtimepath:append("~/path/to/temper/editors/nvim")
require("temper").setup()
```

## Configuration

```lua
require("temper").setup({
  -- Daemon connection
  host = "127.0.0.1",
  port = 7432,

  -- UI settings
  panel_width = 60,
  panel_position = "right", -- "left" or "right"

  -- Auto behaviors
  auto_run_on_save = false,

  -- Keymaps (set to false to disable all, or individual keys)
  keymaps = {
    hint = "<leader>th",     -- Request hint
    review = "<leader>tr",   -- Request code review
    stuck = "<leader>ts",    -- Signal stuck
    next = "<leader>tn",     -- What to do next
    explain = "<leader>te",  -- Request explanation
    run = "<leader>tR",      -- Run checks
    toggle = "<leader>tt",   -- Toggle panel
  },
})
```

## Commands

### Session Management

| Command | Description |
|---------|-------------|
| `:TemperStart <pack/exercise>` | Start a session with an exercise |
| `:TemperStop` | End current session |
| `:TemperStatus` | Show session status |
| `:TemperHealth` | Check daemon health |

### Pairing Commands

| Command | Description |
|---------|-------------|
| `:TemperHint` | Request a hint |
| `:TemperReview` | Request code review |
| `:TemperStuck` | Signal that you're stuck |
| `:TemperNext` | Ask what to do next |
| `:TemperExplain` | Request an explanation |

### Code Execution

| Command | Description |
|---------|-------------|
| `:TemperRun` | Run format/build/test checks |
| `:TemperFormat` | Format current file |

### Spec Commands (Specular format)

| Command | Description |
|---------|-------------|
| `:TemperSpecCreate <name>` | Create a new spec scaffold |
| `:TemperSpecList` | List specs in workspace |
| `:TemperSpecValidate <path>` | Validate spec completeness |
| `:TemperSpecStatus <path>` | Show spec progress |
| `:TemperSpecLock <path>` | Generate SpecLock for drift detection |
| `:TemperSpecDrift <path>` | Show drift from locked spec |

### Stats/Analytics

| Command | Description |
|---------|-------------|
| `:TemperStats` | Show learning statistics overview |
| `:TemperStatsSkills` | Show skill progression by topic |
| `:TemperStatsErrors` | Show common error patterns |
| `:TemperStatsTrend` | Show hint dependency over time |

### Other

| Command | Description |
|---------|-------------|
| `:TemperMode <mode>` | Set learning track (practice/interview-prep) |
| `:TemperExercises` | List available exercises |
| `:TemperToggle` | Toggle the Temper panel |

## Workflow

1. Start the daemon: `temper start`
2. Open Neovim and navigate to your project
3. Start a session: `:TemperStart go-fundamentals/hello-world`
4. Work on the exercise
5. Run checks: `:TemperRun` or `<leader>tR`
6. Request help when needed: `:TemperHint`, `:TemperStuck`, etc.
7. End session: `:TemperStop`

## Learning Contract

Temper enforces a Learning Contract that limits how much help you receive:

- **L0**: Clarifying questions only
- **L1**: Category hints (direction to explore)
- **L2**: Location + concept (default max for practice)
- **L3**: Constrained snippets/outlines
- **L4**: Partial solutions (gated)
- **L5**: Full solutions (rare)

The AI will select the minimum helpful intervention level based on your intent and context.

## License

MIT
