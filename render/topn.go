package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	barWidth     = 10 // Width of the sparkline bar
	defaultCount = 5  // Default number of files to show
)

// SortBy specifies the sorting criteria for topn mode.
type SortBy string

const (
	SortByTotal SortBy = "total" // Sort by total changes (adds + dels)
	SortByAdds  SortBy = "adds"  // Sort by additions only
	SortByDels  SortBy = "dels"  // Sort by deletions only
)

// TopNRenderer shows the N files with the most changes.
type TopNRenderer struct {
	N        int
	SortBy   SortBy // Sorting criteria (default: total)
	UseColor bool
	w        io.Writer
}

// NewTopNRenderer creates a top-N summary renderer.
func NewTopNRenderer(w io.Writer, useColor bool, n int) *TopNRenderer {
	if n <= 0 {
		n = defaultCount
	}
	return &TopNRenderer{N: n, SortBy: SortByTotal, UseColor: useColor, w: w}
}

// Render outputs the top N files by configured sort criteria.
func (r *TopNRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Sort files by configured criteria (descending)
	files := make([]diff.FileStat, len(stats.Files))
	copy(files, stats.Files)
	sort.Slice(files, func(i, j int) bool {
		return r.sortValue(files[i]) > r.sortValue(files[j])
	})

	// Take top N
	showCount := min(r.N, len(files))
	topFiles := files[:showCount]

	// Calculate max path length for alignment.
	// Display paths as-is (no truncation) to maintain alignment of stats column.
	maxPathLen := 0
	for _, f := range topFiles {
		maxPathLen = max(maxPathLen, len(f.Path))
	}

	// Print each file
	for _, f := range topFiles {
		r.renderFile(f, maxPathLen)
	}

	// Summary line
	r.renderSummary(stats, showCount)
}

// renderFile outputs a single file line.
func (r *TopNRenderer) renderFile(f diff.FileStat, maxPathLen int) {
	var sb strings.Builder

	// Path (left-aligned with padding, no indent for compact status line display)
	path := f.Path
	pathColor := ColorReset
	if f.IsUntracked {
		pathColor = ColorNew
	}
	sb.WriteString(r.color(pathColor))
	sb.WriteString(fmt.Sprintf("%-*s", maxPathLen, path))
	sb.WriteString(r.color(ColorReset))

	// Stats: +X -Y (right-aligned in fixed width)
	statsStr := r.formatStats(f.Additions, f.Deletions)
	sb.WriteString("  ")
	sb.WriteString(statsStr)

	// Sparkline bar
	sb.WriteString("  ")
	sb.WriteString(r.formatBar(f.Additions, f.Deletions))

	fmt.Fprintln(r.w, sb.String())
}

// formatStats returns colored +X -Y string.
func (r *TopNRenderer) formatStats(add, del int) string {
	var sb strings.Builder

	// Fixed width: +XXX -XXX (14 chars total)
	if add > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(fmt.Sprintf("+%-4d", add))
		sb.WriteString(r.color(ColorReset))
	} else {
		sb.WriteString("     ")
	}

	if del > 0 {
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(fmt.Sprintf("-%-4d", del))
		sb.WriteString(r.color(ColorReset))
	} else {
		sb.WriteString("     ")
	}

	return sb.String()
}

// formatBar creates a sparkline bar with absolute scaling.
func (r *TopNRenderer) formatBar(add, del int) string {
	total := add + del
	filled := filledFromTotal(total)
	block := blockChar(total)
	return RatioBar(add, del, filled, barWidth, block, r.color)
}

// renderSummary outputs the totals line with hidden file context.
func (r *TopNRenderer) renderSummary(stats *diff.DiffStats, shown int) {
	fmt.Fprintln(r.w)

	hiddenCount := stats.TotalFiles - shown

	var sb strings.Builder

	// Always show total stats first
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))

	// File count with hidden context
	if hiddenCount > 0 {
		sb.WriteString(fmt.Sprintf(" (%d of %d files)", shown, stats.TotalFiles))
	} else {
		sb.WriteString(fmt.Sprintf(" (%d files)", stats.TotalFiles))
	}

	fmt.Fprintln(r.w, sb.String())
}

// color returns the ANSI code if color is enabled.
func (r *TopNRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

// sortValue returns the value to sort by for a file.
func (r *TopNRenderer) sortValue(f diff.FileStat) int {
	switch r.SortBy {
	case SortByAdds:
		return f.Additions
	case SortByDels:
		return f.Deletions
	default:
		return f.Additions + f.Deletions
	}
}
