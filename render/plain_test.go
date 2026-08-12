package render

import (
	"bytes"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

func renderPlain(t *testing.T, stats *diff.DiffStats) string {
	t.Helper()
	var buf bytes.Buffer
	NewPlainRenderer(&buf).Render(stats)
	return buf.String()
}

func TestPlainRenderer_Golden(t *testing.T) {
	stats := &diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "src/api/routes.go", Additions: 40, Deletions: 20},
			{Path: "src/api/handler.go", Additions: 100, Deletions: 0, IsUntracked: true},
			{Path: "src/util.go", Additions: 10, Deletions: 10, IsBinary: true},
		},
		TotalAdd:   150,
		TotalDel:   30,
		TotalFiles: 3,
	}

	want := `3 files changed, +150 -30
all under src/
+100 -0 api/handler.go [new]
+40 -20 api/routes.go
+10 -10 util.go [binary]
`
	if got := renderPlain(t, stats); got != want {
		t.Errorf("plain output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlainRenderer_StableOrdering(t *testing.T) {
	// Equal additions rank by deletions, then path.
	stats := &diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "b.go", Additions: 5, Deletions: 0},
			{Path: "a.go", Additions: 5, Deletions: 0},
			{Path: "c.go", Additions: 5, Deletions: 9},
		},
		TotalAdd:   15,
		TotalDel:   9,
		TotalFiles: 3,
	}

	want := `3 files changed, +15 -9
+5 -9 c.go
+5 -0 a.go
+5 -0 b.go
`
	if got := renderPlain(t, stats); got != want {
		t.Errorf("ordering mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlainRenderer_NoCommonPrefix(t *testing.T) {
	stats := &diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "README.md", Additions: 2, Deletions: 1},
			{Path: "src/main.go", Additions: 1, Deletions: 0},
		},
		TotalAdd:   3,
		TotalDel:   1,
		TotalFiles: 2,
	}

	want := `2 files changed, +3 -1
+2 -1 README.md
+1 -0 src/main.go
`
	if got := renderPlain(t, stats); got != want {
		t.Errorf("no-prefix output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlainRenderer_SingleFileKeepsFullPath(t *testing.T) {
	stats := &diff.DiffStats{
		Files:      []diff.FileStat{{Path: "deep/nested/dir/file.go", Additions: 7, Deletions: 2}},
		TotalAdd:   7,
		TotalDel:   2,
		TotalFiles: 1,
	}

	want := `1 files changed, +7 -2
+7 -2 deep/nested/dir/file.go
`
	if got := renderPlain(t, stats); got != want {
		t.Errorf("single-file output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlainRenderer_Empty(t *testing.T) {
	if got := renderPlain(t, &diff.DiffStats{}); got != "no changes\n" {
		t.Errorf("empty output = %q, want %q", got, "no changes\n")
	}
	if got := renderPlain(t, nil); got != "no changes\n" {
		t.Errorf("nil output = %q, want %q", got, "no changes\n")
	}
}

func TestPlainRenderer_NoANSIOrGlyphs(t *testing.T) {
	stats := &diff.DiffStats{
		Files:      []diff.FileStat{{Path: "a/b/c.go", Additions: 3, Deletions: 1}},
		TotalAdd:   3,
		TotalDel:   1,
		TotalFiles: 1,
	}
	got := renderPlain(t, stats)
	for _, banned := range []string{"\033", " ", "├", "─", "▂"} {
		if bytes.Contains([]byte(got), []byte(banned)) {
			t.Errorf("plain output contains banned sequence %q: %q", banned, got)
		}
	}
}
