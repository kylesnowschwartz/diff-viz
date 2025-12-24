# diff-viz

Git diff visualization tool. Renders git diffs in multiple formats optimized for quick comprehension.

## Development

```bash
just test      # Run tests
just build     # Build binary
just check     # Vet + build
just demo      # See all modes in action
```

## Architecture

```
cmd/git-diff-tree/    CLI entry point
internal/
  diff/               Git diff parsing (git diff-tree, git write-tree)
  render/             Visualization renderers (one per mode)
```

## Adding a New Renderer

1. Create `internal/render/yourmode.go` implementing `Renderer` interface
2. Add mode to `validModes` slice in `cmd/git-diff-tree/main.go`
3. Add case to `getRenderer()` switch in main.go
4. Add description to `modeDescriptions` map

## Key Types

- `diff.DiffStats` - Parsed diff data (files, adds, dels)
- `render.TreeNode` - Hierarchical file tree for visualization
- `Renderer` interface - `Render(stats *diff.DiffStats)`

## JSON Output

`--stats-json` provides stable programmatic output:

```json
{"files":[{"path":"src/main.go","adds":10,"dels":5}],"totals":{"adds":10,"dels":5,"fileCount":1}}
```

Used by tools like bumper-lanes for threshold calculations.
