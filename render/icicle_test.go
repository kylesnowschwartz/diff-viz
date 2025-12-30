package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// TestIcicleRendererMatrix tests the icicle renderer across various width/depth combinations.
// This serves as a regression test and a template for future renderer version comparisons.
func TestIcicleRendererMatrix(t *testing.T) {
	stats := &diff.DiffStats{
		TotalFiles: 5,
		TotalAdd:   500,
		TotalDel:   100,
		Files: []diff.FileStat{
			{Path: "cmd/server/main.go", Additions: 200, Deletions: 50},
			{Path: "internal/api/handlers/user.go", Additions: 150, Deletions: 30},
			{Path: "internal/api/handlers/auth.go", Additions: 100, Deletions: 10},
			{Path: "pkg/utils/strings.go", Additions: 30, Deletions: 5},
			{Path: "go.mod", Additions: 20, Deletions: 5},
		},
	}

	widths := []int{60, 80, 100, 120, 150}
	depths := []int{0, 2, 3, 4, 6} // 0 = unlimited

	for _, width := range widths {
		for _, depth := range depths {
			t.Run(testName(width, depth, true), func(t *testing.T) {
				var buf bytes.Buffer
				r := NewIcicleRenderer(&buf, true)
				r.Width = width
				r.MaxDepth = depth
				r.Render(stats)

				output := buf.String()
				if output == "" {
					t.Error("expected non-empty output")
				}
				if !strings.Contains(output, "+500") {
					t.Error("expected output to contain total additions")
				}
			})
		}
	}
}

// TestIcicleRendererNoColor tests without color codes.
func TestIcicleRendererNoColor(t *testing.T) {
	stats := &diff.DiffStats{
		TotalFiles: 2,
		TotalAdd:   50,
		TotalDel:   10,
		Files: []diff.FileStat{
			{Path: "main.go", Additions: 30, Deletions: 5},
			{Path: "util.go", Additions: 20, Deletions: 5},
		},
	}

	for _, width := range []int{80, 100} {
		for _, depth := range []int{3, 4} {
			t.Run(testName(width, depth, false), func(t *testing.T) {
				var buf bytes.Buffer
				r := NewIcicleRenderer(&buf, false)
				r.Width = width
				r.MaxDepth = depth
				r.Render(stats)

				output := buf.String()
				// No ANSI escape codes in output
				if strings.Contains(output, "\033[") {
					t.Error("expected no ANSI color codes in no-color mode")
				}
			})
		}
	}
}

// TestIcicleRendererEdgeCases tests edge case configurations.
func TestIcicleRendererEdgeCases(t *testing.T) {
	stats := &diff.DiffStats{
		TotalFiles: 3,
		TotalAdd:   100,
		TotalDel:   20,
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 60, Deletions: 10},
			{Path: "src/util.go", Additions: 30, Deletions: 5},
			{Path: "README.md", Additions: 10, Deletions: 5},
		},
	}

	cases := []struct {
		width, depth int
		desc         string
	}{
		{40, 4, "narrow"},
		{200, 4, "wide"},
		{100, 1, "depth=1"},
		{50, 0, "narrow unlimited"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer
			r := NewIcicleRenderer(&buf, true)
			r.Width = tc.width
			r.MaxDepth = tc.depth
			r.Render(stats)

			if buf.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

// TestIcicleRendererEmpty tests empty input handling.
func TestIcicleRendererEmpty(t *testing.T) {
	stats := &diff.DiffStats{
		TotalFiles: 0,
		TotalAdd:   0,
		TotalDel:   0,
		Files:      nil,
	}

	var buf bytes.Buffer
	r := NewIcicleRenderer(&buf, true)
	r.Render(stats)

	output := buf.String()
	if !strings.Contains(output, "No changes") {
		t.Errorf("expected 'No changes' message, got %q", output)
	}
}

// BenchmarkIcicleRenderer benchmarks the icicle renderer.
func BenchmarkIcicleRenderer(b *testing.B) {
	stats := &diff.DiffStats{
		TotalFiles: 20,
		TotalAdd:   1000,
		TotalDel:   200,
		Files:      generateTestFiles(20),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		r := NewIcicleRenderer(&buf, true)
		r.Render(stats)
	}
}

func testName(width, depth int, color bool) string {
	mode := "color"
	if !color {
		mode = "nocolor"
	}
	return strings.ReplaceAll(
		strings.TrimSpace(
			strings.Join([]string{
				"w", itoa(width),
				"d", itoa(depth),
				mode,
			}, ""),
		), " ", "_")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

func generateTestFiles(n int) []diff.FileStat {
	files := make([]diff.FileStat, n)
	dirs := []string{"src", "pkg", "internal", "cmd", "test"}

	for i := 0; i < n; i++ {
		dir := dirs[i%len(dirs)]
		files[i] = diff.FileStat{
			Path:      dir + "/file" + string(rune('a'+i%26)) + ".go",
			Additions: (i + 1) * 10,
			Deletions: (i + 1) * 2,
		}
	}
	return files
}
