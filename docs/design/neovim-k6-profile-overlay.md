# Neovim k6 Profile Overlay

This repository now includes a small Neovim plugin that overlays JS-attributed profile
metrics as virtual text on each matching line of the current file.

## Installation

This plugin is currently local to this repository (not published as a separate plugin),
so point your plugin manager to this repo path or clone it locally.

### lazy.nvim

```lua
{
  dir = "/path/to/k6",
  name = "k6_profile",
}
```

### packer.nvim

```lua
use({
  "/path/to/k6",
  as = "k6_profile",
})
```

### Manual

1. Clone this repository locally.
2. Ensure the `plugin/` and `lua/` folders from this repo are in your `runtimepath`.
3. Restart Neovim (or run `:source` on your plugin manager config).

## Files

- `plugin/k6_profile.lua`
- `lua/k6_profile/init.lua`
- `cmds/k6_profile_export/main.go` (helper that converts pprof to JSON per file/line)

## What it shows

For each line with profile samples, the plugin renders virtual text like:

- `CPU <duration>`
- `MEM <alloc_space>`
- `OBJ <alloc_objects>`

## Commands

- `:K6ProfileLoad <path-to-pprof>`
  - runs the helper via `go run cmds/k6_profile_export/main.go -pprof <file>`
  - loads per-file/per-line metrics and profile totals into memory
  - applies overlays to current buffer
- `:K6ProfileApply`
  - reapplies overlays for current buffer
- `:K6ProfileClear`
  - clears overlays in current buffer
- `:K6ProfileRunCurrent`
  - runs current file with `k6` profiling enabled
  - writes artifacts to `/tmp`
  - auto-loads the generated CPU profile

The plugin also reapplies overlays automatically on `BufEnter`/`BufWinEnter`.

## Notes

- File matching is best-effort (absolute path, relative path, and suffix fallback).
- Attribution shown in the overlay is inclusive and may overlap across caller/callee lines.
- Requires `go` in `PATH` to run the helper.
