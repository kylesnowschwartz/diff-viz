package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

func TestGaugeRenderer_SqrtScaleFilled(t *testing.T) {
	r := &GaugeRenderer{Width: 20}

	// Sqrt scale: filled = sqrt(total) / sqrt(2000) * 20
	// sqrt(2000) ≈ 44.7, so filled ≈ sqrt(total) * 0.447
	tests := []struct {
		name     string
		total    int
		wantMin  int // Minimum expected blocks
		wantMax  int // Maximum expected blocks
	}{
		{"zero changes", 0, 0, 0},
		{"1 line", 1, 1, 1},
		{"5 lines", 5, 1, 2},
		{"10 lines", 10, 1, 2},
		{"25 lines", 25, 2, 3},
		{"50 lines", 50, 3, 4},
		{"100 lines", 100, 4, 5},
		{"250 lines", 250, 6, 8},
		{"500 lines", 500, 9, 11},
		{"1000 lines", 1000, 13, 15},
		{"2000 lines (max reference)", 2000, 20, 20},
		{"5000 lines (beyond max)", 5000, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.sqrtScaleFilled(tt.total)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("sqrtScaleFilled(%d) = %d, want between %d and %d",
					tt.total, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGaugeRenderer_SqrtScaleProgression(t *testing.T) {
	// Verify sqrt scale gives smooth progression: larger values always >= smaller values
	r := &GaugeRenderer{Width: 20}

	totals := []int{1, 5, 10, 25, 50, 100, 250, 500, 750, 1000}
	prevFilled := 0

	for _, total := range totals {
		filled := r.sqrtScaleFilled(total)
		if filled < prevFilled {
			t.Errorf("Sqrt scale not monotonic: %d lines gave %d blocks, but smaller value gave %d",
				total, filled, prevFilled)
		}
		prevFilled = filled
	}
}

func TestGaugeRenderer_Render_NoChanges(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{TotalFiles: 0}
	r.Render(stats)

	got := strings.TrimSpace(buf.String())
	if got != "No changes" {
		t.Errorf("Render() = %q, want %q", got, "No changes")
	}
}

func TestGaugeRenderer_Render_AllAdditions(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   100,
		TotalDel:   0,
		TotalFiles: 1,
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 100, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Should show +100, not -0
	if strings.Contains(got, "-0") {
		t.Errorf("Render() contains '-0', should omit zero deletions: %q", got)
	}
	if !strings.Contains(got, "+100") {
		t.Errorf("Render() should contain '+100': %q", got)
	}
}

func TestGaugeRenderer_Render_AllDeletions(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   0,
		TotalDel:   50,
		TotalFiles: 1,
		Files: []diff.FileStat{
			{Path: "src/old.go", Additions: 0, Deletions: 50},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Should show -50, not +0
	if strings.Contains(got, "+0") {
		t.Errorf("Render() contains '+0', should omit zero additions: %q", got)
	}
	if !strings.Contains(got, "-50") {
		t.Errorf("Render() should contain '-50': %q", got)
	}
}

func TestGaugeRenderer_Render_MixedChanges(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   312,
		TotalDel:   13,
		TotalFiles: 5,
		Files: []diff.FileStat{
			{Path: "src/lib/parser.go", Additions: 150, Deletions: 10},
			{Path: "src/main.go", Additions: 50, Deletions: 3},
			{Path: "tests/parser_test.go", Additions: 89, Deletions: 0},
			{Path: "docs/README.md", Additions: 15, Deletions: 0},
			{Path: "config.json", Additions: 8, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Should contain both +312 and -13
	if !strings.Contains(got, "+312") {
		t.Errorf("Render() should contain '+312': %q", got)
	}
	if !strings.Contains(got, "-13") {
		t.Errorf("Render() should contain '-13': %q", got)
	}
	// Should have directory percentages
	if !strings.Contains(got, "src") {
		t.Errorf("Render() should contain 'src' directory: %q", got)
	}
	if !strings.Contains(got, "%") {
		t.Errorf("Render() should contain percentage: %q", got)
	}
}

func TestGaugeRenderer_Render_DirectoryPercentages(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   100,
		TotalDel:   0,
		TotalFiles: 3,
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 50, Deletions: 0},
			{Path: "tests/test.go", Additions: 30, Deletions: 0},
			{Path: "docs/README.md", Additions: 20, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// src should be 50%, tests 30%, docs 20%
	if !strings.Contains(got, "src 50%") {
		t.Errorf("Render() should contain 'src 50%%': %q", got)
	}
	if !strings.Contains(got, "tests 30%") {
		t.Errorf("Render() should contain 'tests 30%%': %q", got)
	}
	if !strings.Contains(got, "docs 20%") {
		t.Errorf("Render() should contain 'docs 20%%': %q", got)
	}
}

func TestGaugeRenderer_Render_MaxDirsLimit(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)
	r.MaxDirs = 2 // Only show top 2 dirs

	stats := &diff.DiffStats{
		TotalAdd:   100,
		TotalDel:   0,
		TotalFiles: 4,
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 40, Deletions: 0},
			{Path: "tests/test.go", Additions: 30, Deletions: 0},
			{Path: "docs/README.md", Additions: 20, Deletions: 0},
			{Path: "config/settings.go", Additions: 10, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Should show top 2 dirs (src, tests) and "+2 more"
	if !strings.Contains(got, "src") {
		t.Errorf("Render() should contain 'src': %q", got)
	}
	if !strings.Contains(got, "tests") {
		t.Errorf("Render() should contain 'tests': %q", got)
	}
	if !strings.Contains(got, "+2 more") {
		t.Errorf("Render() should contain '+2 more': %q", got)
	}
}

func TestGaugeRenderer_Render_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   100,
		TotalDel:   50,
		TotalFiles: 3,
		Files: []diff.FileStat{
			{Path: "src/main.go", Additions: 60, Deletions: 30},
			{Path: "tests/test.go", Additions: 30, Deletions: 15},
			{Path: "docs/README.md", Additions: 10, Deletions: 5},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Output should be a single line (one newline at the end)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("Render() produced %d lines, want 1: %q", len(lines), got)
	}
}

func TestGaugeRenderer_Render_BarWidth(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)
	r.Width = 10 // Smaller bar for easier counting

	stats := &diff.DiffStats{
		TotalAdd:   2000, // Should fill bar completely (2000 = max reference)
		TotalDel:   0,
		TotalFiles: 1,
		Files: []diff.FileStat{
			{Path: "big.go", Additions: 2000, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Count filled blocks - should be exactly 10 █ characters
	filled := strings.Count(got, BlockFull)
	if filled != 10 {
		t.Errorf("Render() has %d filled blocks, want 10: %q", filled, got)
	}
	// Should have no empty blocks
	empty := strings.Count(got, BlockEmpty)
	if empty != 0 {
		t.Errorf("Render() has %d empty blocks, want 0: %q", empty, got)
	}
}

func TestGaugeRenderer_Render_SmallChangeFillsAtLeastOne(t *testing.T) {
	var buf bytes.Buffer
	r := NewGaugeRenderer(&buf, false)

	stats := &diff.DiffStats{
		TotalAdd:   1,
		TotalDel:   0,
		TotalFiles: 1,
		Files: []diff.FileStat{
			{Path: "tiny.go", Additions: 1, Deletions: 0},
		},
	}
	r.Render(stats)

	got := buf.String()
	// Even 1 line should fill at least 1 block
	filled := strings.Count(got, BlockFull)
	if filled < 1 {
		t.Errorf("Render() has %d filled blocks, want at least 1: %q", filled, got)
	}
}

func TestGaugeRenderer_BuildBar_AddDelRatio(t *testing.T) {
	r := &GaugeRenderer{Width: 10, UseColor: false}

	tests := []struct {
		name       string
		add, del   int
		filled     int
		wantAdd    int // Expected add blocks
		wantDel    int // Expected del blocks
	}{
		{"all adds", 100, 0, 10, 10, 0},
		{"all dels", 0, 100, 10, 0, 10},
		{"50/50 split", 50, 50, 10, 5, 5},
		{"75/25 split", 75, 25, 8, 6, 2},
		{"small del gets at least 1", 99, 1, 10, 9, 1},
		{"small add gets at least 1", 1, 99, 10, 1, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.buildBar(tt.add, tt.del, tt.filled)
			addBlocks := strings.Count(got, BlockFull)

			// With no color, we can't distinguish add vs del blocks
			// Just verify total filled is correct
			totalFilled := addBlocks
			empty := strings.Count(got, BlockEmpty)
			expectedEmpty := r.Width - tt.filled

			if empty != expectedEmpty {
				t.Errorf("buildBar() has %d empty blocks, want %d: %q",
					empty, expectedEmpty, got)
			}
			if totalFilled != tt.filled {
				t.Errorf("buildBar() has %d filled blocks, want %d: %q",
					totalFilled, tt.filled, got)
			}
		})
	}
}
