package render

import (
	"bytes"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// TestV1V2OutputMatch verifies v2 produces identical output to v1.
func TestV1V2OutputMatch(t *testing.T) {
	tests := []struct {
		name  string
		stats *diff.DiffStats
	}{
		{
			name: "single file",
			stats: &diff.DiffStats{
				TotalFiles: 1,
				TotalAdd:   50,
				TotalDel:   10,
				Files: []diff.FileStat{
					{Path: "main.go", Additions: 50, Deletions: 10},
				},
			},
		},
		{
			name: "multiple files flat",
			stats: &diff.DiffStats{
				TotalFiles: 3,
				TotalAdd:   100,
				TotalDel:   20,
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 60, Deletions: 10},
					{Path: "b.go", Additions: 30, Deletions: 5},
					{Path: "c.go", Additions: 10, Deletions: 5},
				},
			},
		},
		{
			name: "nested directories",
			stats: &diff.DiffStats{
				TotalFiles: 4,
				TotalAdd:   200,
				TotalDel:   50,
				Files: []diff.FileStat{
					{Path: "src/main.go", Additions: 80, Deletions: 20},
					{Path: "src/util/helper.go", Additions: 50, Deletions: 10},
					{Path: "test/main_test.go", Additions: 40, Deletions: 10},
					{Path: "README.md", Additions: 30, Deletions: 10},
				},
			},
		},
		{
			name: "deeply nested",
			stats: &diff.DiffStats{
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
			},
		},
		{
			name: "empty",
			stats: &diff.DiffStats{
				TotalFiles: 0,
				TotalAdd:   0,
				TotalDel:   0,
				Files:      nil,
			},
		},
		{
			name: "many small files",
			stats: &diff.DiffStats{
				TotalFiles: 10,
				TotalAdd:   100,
				TotalDel:   0,
				Files: []diff.FileStat{
					{Path: "a.go", Additions: 10, Deletions: 0},
					{Path: "b.go", Additions: 10, Deletions: 0},
					{Path: "c.go", Additions: 10, Deletions: 0},
					{Path: "d.go", Additions: 10, Deletions: 0},
					{Path: "e.go", Additions: 10, Deletions: 0},
					{Path: "f.go", Additions: 10, Deletions: 0},
					{Path: "g.go", Additions: 10, Deletions: 0},
					{Path: "h.go", Additions: 10, Deletions: 0},
					{Path: "i.go", Additions: 10, Deletions: 0},
					{Path: "j.go", Additions: 10, Deletions: 0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render with v1
			var v1Buf bytes.Buffer
			v1 := NewIcicleRenderer(&v1Buf, false) // no color for easier comparison
			v1.Render(tt.stats)

			// Render with v2
			var v2Buf bytes.Buffer
			v2 := NewIcicleRendererV2(&v2Buf, false)
			v2.Render(tt.stats)

			v1Output := v1Buf.String()
			v2Output := v2Buf.String()

			if v1Output != v2Output {
				t.Errorf("v1 and v2 output differ:\n--- v1 ---\n%s\n--- v2 ---\n%s", v1Output, v2Output)
			}
		})
	}
}

// TestV1V2OutputMatch_WithColor tests with color enabled.
func TestV1V2OutputMatch_WithColor(t *testing.T) {
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

	var v1Buf bytes.Buffer
	v1 := NewIcicleRenderer(&v1Buf, true)
	v1.Render(stats)

	var v2Buf bytes.Buffer
	v2 := NewIcicleRendererV2(&v2Buf, true)
	v2.Render(stats)

	if v1Buf.String() != v2Buf.String() {
		t.Errorf("v1 and v2 output differ with color:\n--- v1 ---\n%s\n--- v2 ---\n%s",
			v1Buf.String(), v2Buf.String())
	}
}

// TestV1V2OutputMatch_CustomWidth tests with non-default width.
func TestV1V2OutputMatch_CustomWidth(t *testing.T) {
	stats := &diff.DiffStats{
		TotalFiles: 2,
		TotalAdd:   50,
		TotalDel:   10,
		Files: []diff.FileStat{
			{Path: "main.go", Additions: 30, Deletions: 5},
			{Path: "util.go", Additions: 20, Deletions: 5},
		},
	}

	widths := []int{60, 80, 120, 150}

	for _, width := range widths {
		t.Run(string(rune('0'+width/10)), func(t *testing.T) {
			var v1Buf bytes.Buffer
			v1 := NewIcicleRenderer(&v1Buf, false)
			v1.Width = width
			v1.Render(stats)

			var v2Buf bytes.Buffer
			v2 := NewIcicleRendererV2(&v2Buf, false)
			v2.Width = width
			v2.Render(stats)

			if v1Buf.String() != v2Buf.String() {
				t.Errorf("width=%d: v1 and v2 differ:\n--- v1 ---\n%s\n--- v2 ---\n%s",
					width, v1Buf.String(), v2Buf.String())
			}
		})
	}
}

// BenchmarkIcicleV1 benchmarks the v1 renderer.
func BenchmarkIcicleV1(b *testing.B) {
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

// BenchmarkIcicleV2 benchmarks the v2 renderer.
func BenchmarkIcicleV2(b *testing.B) {
	stats := &diff.DiffStats{
		TotalFiles: 20,
		TotalAdd:   1000,
		TotalDel:   200,
		Files:      generateTestFiles(20),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		r := NewIcicleRendererV2(&buf, true)
		r.Render(stats)
	}
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
