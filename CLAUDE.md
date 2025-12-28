# diff-viz

Git diff visualization tool. Renders git diffs in multiple formats optimized for quick comprehension.

## Development

```bash
just test      # Run tests
just build     # Build binary
just check     # Vet + build
just demo      # See all modes in action
git-diff-tree --demo           # Show all modes (root..HEAD)
git-diff-tree --demo -m icicle # Show single mode in demo
```

## Architecture

```
cmd/git-diff-tree/    CLI entry point, flag parsing, renderer dispatch
config/               Mode defaults, config file handling
diff/                 Git diff parsing (numstat, rename resolution)
render/               Visualization renderers (one per mode)
```

## Adding a New Renderer

1. Create `render/yourmode.go` implementing `Renderer` interface
2. Add to `ValidModes` slice in `render/modes.go`
3. Add description to `ModeDescriptions` map in `render/modes.go`
4. Add case to `getRenderer()` switch in `cmd/git-diff-tree/main.go`
5. (Optional) Add `ModeDefaults` entry in `config/defaults.go`

## Shared Utilities

Reuse these instead of reimplementing:
- `render/bar.go` - `RatioBar()`, `BarConfig` for sparkline bars
- `render/colors.go` - ANSI constants (`ColorAdd`, `ColorDel`, `ColorDir`, etc.)
- `render/path.go` - `GroupByDepth()`, `ParseDepthPath()`, `SortTopDirs()`
- `render/tree_builder.go` - `BuildTreeFromFiles()`, `CalcTotals()`

## Config Resolution

Precedence (lowest to highest): hardcoded defaults -> `ModeDefaults` -> config file -> CLI flags

## Data Flow

```
git diff --numstat -> ParseNumstat() -> DiffStats{Files, TotalAdd, TotalDel}
                                              |
                              GroupByDepth() / BuildTreeFromFiles()
                                              |
                              Renderer.Render() -> stdout
```

## Common Gotchas

| Issue | Cause | Fix |
|-------|-------|-----|
| Positional args ignored | Function doesn't pass `flag.Args()` | Pass args through explicitly |
| pflag shorthand `-m` not working | Used `flag.String("m",...)` | Use `flag.StringP("mode","m",...)` |
| Demo ignoring flags | Demo functions hard-code values | Pass flag values through |
| Binary not updating | Go build caching | `rm ./git-diff-tree && go build` |
| `GroupByDepth` loses path info | Returns `(groupKey, subPath)` tuples | Use `truncatePathToDepth()` for flat views needing full truncated paths |

## Key Types

- `diff.DiffStats` - Parsed diff data (files, adds, dels)
- `render.TreeNode` - Hierarchical file tree for visualization
- `Renderer` interface - `Render(stats *diff.DiffStats)`

## Error Handling

Fail-open with warnings. Diff functions return `(*DiffStats, []string, error)`:
- Continues on git errors, malformed input, file read failures
- Warnings collected as `[]string` (idiomatic Go pattern)
- Use `-v`/`--verbose` to print warnings to stderr

## JSON Output

`--stats-json` provides stable programmatic output:

```json
{"files":[{"path":"src/main.go","adds":10,"dels":5}],"totals":{"adds":10,"dels":5,"fileCount":1}}
```

Used by tools like bumper-lanes for threshold calculations.

## Releases

Auto-releases via GitHub Actions on push to main. Uses conventional commits:

- `feat: ...` - minor version bump (v0.1.0 -> v0.2.0)
- `fix: ...` - patch version bump (v0.1.0 -> v0.1.1)
- `docs:`, `chore:`, `style:`, `test:` - no release

Consumers install via:
```bash
go install github.com/kylesnowschwartz/diff-viz/v2/cmd/git-diff-tree@latest
```

No manual tagging required. The workflow creates GitHub Releases with auto-generated notes.

## Go Semantic Import Versioning (v2+)

Go requires v2+ modules to have the major version in the module path. If releasing v2.0.0+:

1. Update `go.mod` module path:
   ```
   module github.com/kylesnowschwartz/diff-viz/v2
   ```

2. Update all internal imports to include `/v2/`:
   ```bash
   find . -name "*.go" -exec sed -i '' 's|github.com/kylesnowschwartz/diff-viz/|github.com/kylesnowschwartz/diff-viz/v2/|g' {} \;
   ```

3. Update install commands in docs (README.md, CLAUDE.md)

4. Commit and tag together so the tag points to a commit with the correct module path

Consumers import as:
```go
import "github.com/kylesnowschwartz/diff-viz/v2/diff"
```
