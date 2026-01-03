# Neovim Plugin

Lua plugin for Neovim with Telescope and which-key integration.

## Installation

### lazy.nvim

```lua
{
  "felixgeelhaar/temper",
  ft = { "go", "python", "typescript" },
  dependencies = {
    "nvim-telescope/telescope.nvim",
    "folke/which-key.nvim",
  },
  config = function()
    require("temper").setup()
  end,
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

## Configuration

```lua
require("temper").setup({
  host = "127.0.0.1",
  port = 7432,
  panel_width = 60,
  panel_position = "right",
  keymaps = {
    hint = "<leader>th",
    review = "<leader>tr",
    stuck = "<leader>ts",
    run = "<leader>tR",
    toggle = "<leader>tt",
  },
})
```

## Commands

| Command | Description |
|---------|-------------|
| `:TemperStart` | Start session |
| `:TemperHint` | Request hint |
| `:TemperRun` | Run checks |
| `:TemperToggle` | Toggle panel |

## Telescope Integration

```lua
require("temper.telescope").exercises()
require("temper.telescope").specs()
require("temper.telescope").stats()
```
