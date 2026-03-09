# Neovim k6 Profile Overlay

This repository now includes a small Neovim plugin that overlays JS-attributed profile
metrics as virtual text on each matching line of the current file.

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
  - loads per-file/per-line metrics into memory
  - applies overlays to current buffer
- `:K6ProfileApply`
  - reapplies overlays for current buffer
- `:K6ProfileClear`
  - clears overlays in current buffer

The plugin also reapplies overlays automatically on `BufEnter`/`BufWinEnter`.

## Notes

- File matching is best-effort (absolute path, relative path, and suffix fallback).
- The plugin currently uses the leaf frame attribution from the pprof profile.
- Requires `go` in `PATH` to run the helper.
