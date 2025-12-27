package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/diff"
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

func TestSmartSparkline_GroupsByTopDir(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.MaxDepth = 2
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 10},
			{Path: "tests/main_test.go", Additions: 20},
		},
		TotalFiles: 2,
	})

	got := buf.String()
	// Should contain both top-level dirs
	if !strings.Contains(got, "src/") {
		t.Errorf("expected output to contain 'src/', got %q", got)
	}
	if !strings.Contains(got, "tests/") {
		t.Errorf("expected output to contain 'tests/', got %q", got)
	}
}

func TestSmartSparkline_WidthNoWrap(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 0 // No wrapping
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "a/file1.go", Additions: 10},
			{Path: "b/file2.go", Additions: 20},
			{Path: "c/file3.go", Additions: 30},
		},
		TotalFiles: 3,
	})

	// Should be single line (plus trailing newline)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line with Width=0, got %d lines", len(lines))
	}
}

func TestSmartSparkline_WidthWraps(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 30 // Very narrow - force wrapping
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "a/file1.go", Additions: 10},
			{Path: "b/file2.go", Additions: 20},
			{Path: "c/file3.go", Additions: 30},
		},
		TotalFiles: 3,
	})

	// With width=30, should wrap to multiple lines
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Errorf("expected multiple lines with Width=30, got %d lines", len(lines))
	}
}

func TestSmartSparkline_DepthAggregation(t *testing.T) {
	files := []diff.FileStat{
		{Path: "src/lib/a.go", Additions: 10},
		{Path: "src/lib/b.go", Additions: 20},
		{Path: "src/main.go", Additions: 5},
	}

	// Depth 2: should show lib aggregate and main.go separately
	var buf2 bytes.Buffer
	r2 := NewSmartSparklineRenderer(&buf2, false)
	r2.MaxDepth = 2
	r2.Render(&diff.DiffStats{Files: files, TotalFiles: 3})
	output2 := buf2.String()

	if !strings.Contains(output2, "lib") {
		t.Errorf("depth=2 should show 'lib', got %q", output2)
	}

	// Depth 1: should just show "src" aggregate
	var buf1 bytes.Buffer
	r1 := NewSmartSparklineRenderer(&buf1, false)
	r1.MaxDepth = 1
	r1.Render(&diff.DiffStats{Files: files, TotalFiles: 3})
	output1 := buf1.String()

	// At depth 1, everything under src is aggregated
	if !strings.Contains(output1, "src") {
		t.Errorf("depth=1 should show 'src', got %q", output1)
	}
	// lib shouldn't appear as a separate segment at depth 1
	if strings.Contains(output1, "lib") {
		t.Errorf("depth=1 should not show 'lib' as separate segment, got %q", output1)
	}
}

func TestSmartSparkline_FileCount(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.MaxDepth = 2
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "src/lib/a.go", Additions: 10},
			{Path: "src/lib/b.go", Additions: 20},
			{Path: "src/lib/c.go", Additions: 30},
		},
		TotalFiles: 3,
	})

	got := buf.String()
	// Should show (3) file count for aggregated lib
	if !strings.Contains(got, "(3)") {
		t.Errorf("expected output to contain '(3)' file count, got %q", got)
	}
}

func TestSmartSparkline_SortsByTotal(t *testing.T) {
	var buf bytes.Buffer
	r := NewSmartSparklineRenderer(&buf, false)
	r.Width = 0 // Single line for easier testing
	r.Render(&diff.DiffStats{
		Files: []diff.FileStat{
			{Path: "small/a.go", Additions: 10},
			{Path: "large/b.go", Additions: 100},
			{Path: "medium/c.go", Additions: 50},
		},
		TotalFiles: 3,
	})

	got := buf.String()
	// "large" should appear before "medium" which should appear before "small"
	largeIdx := strings.Index(got, "large")
	mediumIdx := strings.Index(got, "medium")
	smallIdx := strings.Index(got, "small")

	if largeIdx > mediumIdx || mediumIdx > smallIdx {
		t.Errorf("expected large > medium > small ordering, got %q", got)
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
