package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	defaultGaugeWidth = 30 // Default width of the gauge bar
)

// GaugeRenderer shows budget/threshold consumption as a single-line gauge.
type GaugeRenderer struct {
	UseColor  bool
	Width     int // Gauge bar width (default 30)
	Threshold int // Optional budget threshold (default 0 = show totals only)
	w         io.Writer
}

// NewGaugeRenderer creates a progress gauge renderer.
func NewGaugeRenderer(w io.Writer, useColor bool) *GaugeRenderer {
	return &GaugeRenderer{
		UseColor:  useColor,
		Width:     defaultGaugeWidth,
		Threshold: 0,
		w:         w,
	}
}

// Render outputs the gauge visualization.
func (r *GaugeRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	total := stats.TotalAdd + stats.TotalDel

	// Main gauge line
	if r.Threshold > 0 {
		r.renderWithThreshold(stats, total)
	} else {
		r.renderTotalsOnly(stats, total)
	}

	// Per-directory breakdown (compact single line)
	r.renderDirBreakdown(stats)
}

// renderWithThreshold shows percentage-based budget consumption.
func (r *GaugeRenderer) renderWithThreshold(stats *diff.DiffStats, total int) {
	percentage := (total * 100) / r.Threshold
	if percentage > 100 {
		percentage = 100
	}

	filled := (total * r.Width) / r.Threshold
	if filled > r.Width {
		filled = r.Width
	}

	// Split filled portion between adds (green) and dels (red)
	addBlocks, delBlocks := r.splitBlocks(stats.TotalAdd, stats.TotalDel, filled)

	var sb strings.Builder
	sb.WriteString("Budget: ")
	sb.WriteString(r.renderBar(addBlocks, delBlocks, r.Width-filled))
	sb.WriteString(fmt.Sprintf("  %d%% (%d/%d)", percentage, total, r.Threshold))

	fmt.Fprintln(r.w, sb.String())
}

// renderTotalsOnly shows total changes without threshold.
func (r *GaugeRenderer) renderTotalsOnly(stats *diff.DiffStats, total int) {
	// Use full width for display, filled proportional to total
	filled := r.Width
	if total < r.Width {
		filled = total
		if filled < 1 && total > 0 {
			filled = 1
		}
	}

	// Split filled portion between adds (green) and dels (red)
	addBlocks, delBlocks := r.splitBlocks(stats.TotalAdd, stats.TotalDel, filled)

	var sb strings.Builder
	sb.WriteString("Total: ")
	sb.WriteString(r.renderBar(addBlocks, delBlocks, r.Width-filled))
	sb.WriteString("  ")
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(fmt.Sprintf(" (%d lines)", total))

	fmt.Fprintln(r.w, sb.String())
}

// splitBlocks divides filled blocks proportionally between adds and dels.
func (r *GaugeRenderer) splitBlocks(add, del, filled int) (addBlocks, delBlocks int) {
	total := add + del
	if total == 0 || filled == 0 {
		return 0, 0
	}

	addBlocks = (add * filled) / total
	delBlocks = filled - addBlocks

	// Ensure at least 1 block for non-zero values
	if add > 0 && addBlocks == 0 && filled > 0 {
		addBlocks = 1
		delBlocks = filled - 1
	} else if del > 0 && delBlocks == 0 && filled > 0 {
		delBlocks = 1
		addBlocks = filled - 1
	}

	return addBlocks, delBlocks
}

// renderBar creates the gauge bar with colored add/del blocks and empty padding.
func (r *GaugeRenderer) renderBar(addBlocks, delBlocks, emptyBlocks int) string {
	var sb strings.Builder

	if addBlocks > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(strings.Repeat(BlockFull, addBlocks))
		sb.WriteString(r.color(ColorReset))
	}

	if delBlocks > 0 {
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(strings.Repeat(BlockFull, delBlocks))
		sb.WriteString(r.color(ColorReset))
	}

	if emptyBlocks > 0 {
		sb.WriteString(strings.Repeat(BlockEmpty, emptyBlocks))
	}

	return sb.String()
}

// renderDirBreakdown shows a compact per-directory summary.
func (r *GaugeRenderer) renderDirBreakdown(stats *diff.DiffStats) {
	groups := GroupByDepth(stats.Files, 1)
	if len(groups) == 0 {
		return
	}

	// Sort directories by total changes descending
	sortedDirs := SortTopDirs(groups)

	// Calculate max total for scaling mini-bars
	maxTotal := 0
	for _, dir := range sortedDirs {
		dirTotal := 0
		for _, seg := range groups[dir] {
			dirTotal += seg.Total()
		}
		if dirTotal > maxTotal {
			maxTotal = dirTotal
		}
	}

	// Build compact breakdown line
	var sb strings.Builder
	sb.WriteString("By dir: ")

	// Collect dir entries with their totals for display
	type dirEntry struct {
		name  string
		add   int
		del   int
		total int
	}
	entries := make([]dirEntry, 0, len(sortedDirs))
	for _, dir := range sortedDirs {
		var dirAdd, dirDel int
		for _, seg := range groups[dir] {
			dirAdd += seg.Add
			dirDel += seg.Del
		}
		entries = append(entries, dirEntry{name: dir, add: dirAdd, del: dirDel, total: dirAdd + dirDel})
	}

	// Limit to top 5 directories for compact display
	showCount := len(entries)
	if showCount > 5 {
		showCount = 5
	}

	for i := 0; i < showCount; i++ {
		if i > 0 {
			sb.WriteString("  ")
		}
		entry := entries[i]

		// Mini-bar: scale to 8 blocks max, with add/del ratio coloring
		miniWidth := 8
		filled := 1
		if maxTotal > 0 {
			filled = (entry.total * miniWidth) / maxTotal
			if filled < 1 {
				filled = 1
			}
		}

		sb.WriteString(entry.name)
		sb.WriteString(" ")
		sb.WriteString(RatioBar(entry.add, entry.del, filled, filled, BlockFull, r.color))
	}

	if len(entries) > showCount {
		sb.WriteString(fmt.Sprintf("  +%d more", len(entries)-showCount))
	}

	fmt.Fprintln(r.w, sb.String())
}

// color returns the ANSI code if color is enabled.
func (r *GaugeRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
