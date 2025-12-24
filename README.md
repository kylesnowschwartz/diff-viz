# diff-viz

Hierarchical git diff visualization for terminals.

## Install

```bash
go install github.com/kylesnowschwartz/diff-viz/cmd/git-diff-tree@latest
```

## Usage

```bash
git-diff-tree                    # Working tree vs HEAD
git-diff-tree HEAD~3             # Last 3 commits
git-diff-tree main feature       # Compare branches
git-diff-tree -m icicle          # Different visualization mode
```

## Modes

| Mode | Description |
|------|-------------|
| `tree` | Indented file tree with +/- stats (default) |
| `collapsed` | Single-line per directory |
| `smart` | Depth-2 aggregated sparkline |
| `topn` | Top 5 files by change size |
| `icicle` | Horizontal area chart (width = magnitude) |
| `brackets` | Nested `[dir file]` single-line |

## JSON Output

For programmatic consumption:

```bash
git-diff-tree --stats-json
```

```json
{"files":[{"path":"src/main.go","adds":10,"dels":5}],"totals":{"adds":10,"dels":5,"fileCount":1}}
```

## License

MIT
