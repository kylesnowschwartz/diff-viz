package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// PlainRenderer is the model-facing renderer: output meant to be read by an
// LLM inside a prompt, not by a human in a terminal. No ANSI, no glyphs, no
// truncation, no alignment padding beyond single spaces, stable ordering.
// Files are ranked by additions (the reviewable surface), with deletions and
// path as tiebreaks, so the most review-worthy files come first.
type PlainRenderer struct {
	w io.Writer
}

// NewPlainRenderer creates a plain renderer writing to w.
func NewPlainRenderer(w io.Writer) *PlainRenderer {
	return &PlainRenderer{w: w}
}

// Render writes one header line and one line per file:
//
//	3 files changed, +150 -30
//	all under src/
//	+100 -0 api/handler.go [new]
//	+40 -20 api/routes.go
//	+10 -10 util.go [binary]
//
// A common directory prefix shared by every file is stated once and
// stripped from the per-file lines to keep the output token-lean.
func (r *PlainRenderer) Render(stats *diff.DiffStats) {
	if stats == nil || len(stats.Files) == 0 {
		fmt.Fprintln(r.w, "no changes")
		return
	}

	files := make([]diff.FileStat, len(stats.Files))
	copy(files, stats.Files)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Additions != files[j].Additions {
			return files[i].Additions > files[j].Additions
		}
		if files[i].Deletions != files[j].Deletions {
			return files[i].Deletions > files[j].Deletions
		}
		return files[i].Path < files[j].Path
	})

	fmt.Fprintf(r.w, "%d files changed, +%d -%d\n", stats.TotalFiles, stats.TotalAdd, stats.TotalDel)

	prefix := commonDirPrefix(files)
	if prefix != "" {
		fmt.Fprintf(r.w, "all under %s\n", prefix)
	}

	for _, f := range files {
		line := fmt.Sprintf("+%d -%d %s", f.Additions, f.Deletions, strings.TrimPrefix(f.Path, prefix))
		if f.IsUntracked {
			line += " [new]"
		}
		if f.IsBinary {
			line += " [binary]"
		}
		fmt.Fprintln(r.w, line)
	}
}

// commonDirPrefix returns the longest directory prefix (ending in "/")
// shared by every file, or "" when there is none or only one file.
func commonDirPrefix(files []diff.FileStat) string {
	if len(files) < 2 {
		return ""
	}
	prefix := files[0].Path
	for _, f := range files[1:] {
		for !strings.HasPrefix(f.Path, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		return prefix[:i+1]
	}
	return ""
}
