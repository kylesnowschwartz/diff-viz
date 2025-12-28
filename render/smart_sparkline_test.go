package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

func TestSmartSparkline_NoChanges(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Render(&diff.DiffStats{})

	got := strings.TrimSpace(buf.String())
	if got != "No changes" {
		t.Errorf("expected 'No changes', got %q", got)
	}
}

func TestSmartSparkline_SingleFile(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Render(&diff.DiffStats{
		Files:      []diff.FileStat{{Path: "main.go", Additions: 10}},
		TotalFiles: 1,
	})

	got := buf.String()
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected output to contain 'main.go', got %q", got)
	}
}

func TestSmartSparkline_MultiColumn(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 140 // Wide enough for multiple columns
	r.MaxDepth = 2
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 10},
			{Path: "tests/main_test.go", Additions: 20},
			{Path: "lib/util.go", Additions: 15},
		},
		TotalFiles: 3,
	})

	got := buf.String()
	// Should contain all paths
	if !strings.Contains(got, "src/main.go") {
		t.Errorf("expected output to contain 'src/main.go', got %q", got)
	}
	if !strings.Contains(got, "tests/main_test.go") {
		t.Errorf("expected output to contain 'tests/main_test.go', got %q", got)
	}
}

func TestSmartSparkline_NarrowWidth(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 50 // Narrow - single column
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "a/file1.go", Additions: 10},
			{Path: "b/file2.go", Additions: 20},
			{Path: "c/file3.go", Additions: 30},
		},
		TotalFiles: 3,
	})

	// With narrow width, should have more lines (single column)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// At least 3 file lines plus summary
	if len(lines) < 4 {
		t.Errorf("expected at least 4 lines with narrow width, got %d lines", len(lines))
	}
}

func TestSmartSparkline_DepthAggregation(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/lib/a.go", Additions: 10},
		{Path: "src/lib/b.go", Additions: 20},
		{Path: "src/main.go", Additions: 5},
	}

	// Depth 2: should show src/lib (aggregated) and src/main.go
	var buf2 bytes.Buffer
	r2 := NewSmartSparklineRenderer(&buf2, false)
	r2.MaxDepth = 2
	r2.Width = 100
	r2.Render(&diff.DiffStats{Files: files, TotalFiles: 3})
	output2 := buf2.String()

	if !strings.Contains(output2, "src/lib") {
		t.Errorf("depth=2 should show 'src/lib', got %q", output2)
	}
	if !strings.Contains(output2, "src/main.go") {
		t.Errorf("depth=2 should show 'src/main.go', got %q", output2)
	}

	// Depth 1: should aggregate to "src" only
	var buf1 bytes.Buffer
	r1 := NewSmartSparklineRenderer(&buf1, false)
	r1.MaxDepth = 1
	r1.Width = 100
	r1.Render(&diff.DiffStats{Files: files, TotalFiles: 3})
	output1 := buf1.String()

	// At depth 1, should show "src" with file count
	if !strings.Contains(output1, "src") {
		t.Errorf("depth=1 should show 'src', got %q", output1)
	}
}

func TestSmartSparkline_SortsByMagnitude(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 50 // Single column for easier ordering check
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "small.go", Additions: 10},
			{Path: "large.go", Additions: 100},
			{Path: "medium.go", Additions: 50},
		},
		TotalFiles: 3,
	})

	got := buf.String()
	// "large" should appear before "medium" which should appear before "small"
	largeIdx := strings.Index(got, "large")
	mediumIdx := strings.Index(got, "medium")
	smallIdx := strings.Index(got, "small")

	if largeIdx == -1 || mediumIdx == -1 || smallIdx == -1 {
		t.Fatalf("missing expected paths in output: %q", got)
	}

	if largeIdx > mediumIdx || mediumIdx > smallIdx {
		t.Errorf("expected large > medium > small ordering, got %q", got)
	}
}

func TestSmartSparkline_DepthTruncation(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.MaxDepth = 2
	r.Width = 100
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "cmd/git-diff-tree/main.go", Additions: 50},
		},
		TotalFiles: 1,
	})

	got := buf.String()
	// At depth 2, should show "cmd/git-diff-tree" not full path
	if !strings.Contains(got, "cmd/git-diff-tree") {
		t.Errorf("depth=2 should show 'cmd/git-diff-tree', got %q", got)
	}
	// Should NOT show the full path with main.go at depth 2
	if strings.Contains(got, "cmd/git-diff-tree/main.go") {
		t.Errorf("depth=2 should not show full path, got %q", got)
	}
}

func TestTruncatePathToDepth(t *testing.T) {
	tests := []struct {
		path  string
		depth int
		want  string
	}{
		{"main.go", 1, "main.go"},
		{"main.go", 2, "main.go"},
		{"src/main.go", 1, "src"},
		{"src/main.go", 2, "src/main.go"},
		{"src/lib/util.go", 1, "src"},
		{"src/lib/util.go", 2, "src/lib"},
		{"src/lib/util.go", 3, "src/lib/util.go"},
		{"cmd/git-diff-tree/main.go", 2, "cmd/git-diff-tree"},
		{"cmd/git-diff-tree/main.go", 3, "cmd/git-diff-tree/main.go"},
	}

	for _, tt := range tests {
		got := truncatePathToDepth(tt.path, tt.depth)
		if got != tt.want {
			t.Errorf("truncatePathToDepth(%q, %d) = %q, want %q", tt.path, tt.depth, got, tt.want)
		}
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"\033[32mgreen\033[0m", 5},     // Green "green" text
		{"\033[34mblue\033[0m text", 9}, // Blue "blue" + " text"
		{"no colors", 9},                // Plain text
		{"\033[38;5;8mdark\033[0m", 4},  // 256-color dark gray
		{"\033[1m\033[32mbold green\033[0m\033[0m", 10}, // Multiple escapes
	}

	for _, tt := range tests {
		got := VisibleWidth(tt.input)
		if got != tt.want {
			t.Errorf("VisibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
