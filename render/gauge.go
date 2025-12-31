package render

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

const (
	defaultGaugeWidth = 20   // Default width of the gauge bar
	defaultMaxDirs    = 4    // Max directories to show in percentage breakdown
	scaleMaxReference = 2000 // Reference max for sqrt scale (2000 lines = full bar)
)

// GaugeRenderer shows git diff changes as a single-line HUD gauge.
// Uses square root scaling to visualize magnitude and directory percentages
// to show change distribution.
type GaugeRenderer struct {
	UseColor bool
	Width    int // Gauge bar width (default 20)
	MaxDirs  int // Max directories in percentage breakdown (default 4)
	w        io.Writer
}

// NewGaugeRenderer creates a HUD-style gauge renderer.
func NewGaugeRenderer(w io.Writer, useColor bool) *GaugeRenderer {
	return &GaugeRenderer{
		UseColor: useColor,
		Width:    defaultGaugeWidth,
		MaxDirs:  defaultMaxDirs,
		w:        w,
	}
}

// Render outputs the gauge visualization as a single HUD-style line.
// Format: ████████████░░░░░░░░ +312 -13  src 55% | tests 30% | docs 15%
func (r *GaugeRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	total := stats.TotalAdd + stats.TotalDel

	// Build the single-line HUD output
	var sb strings.Builder

	// Part 1: The gauge bar (log scale)
	sb.WriteString(r.renderBar(stats, total))

	// Part 2: The numbers (+add -del)
	sb.WriteString(" ")
	sb.WriteString(r.renderNumbers(stats))

	// Part 3: Directory percentages
	dirPcts := r.renderDirPercentages(stats, total)
	if dirPcts != "" {
		sb.WriteString("  ")
		sb.WriteString(dirPcts)
	}

	fmt.Fprintln(r.w, sb.String())
}

// renderBar creates the gauge bar using sqrt scale for magnitude visualization.
func (r *GaugeRenderer) renderBar(stats *diff.DiffStats, total int) string {
	filled := r.sqrtScaleFilled(total)
	return r.buildBar(stats.TotalAdd, stats.TotalDel, filled)
}

// sqrtScaleFilled calculates bar fill using square root scale.
// Sqrt provides better visual differentiation than log scale:
// - 10 lines = ~10%, 100 lines = ~30%, 500 lines = ~70%, 1000+ lines = full
func (r *GaugeRenderer) sqrtScaleFilled(total int) int {
	if total <= 0 {
		return 0
	}
	if total >= scaleMaxReference {
		return r.Width
	}

	// Sqrt scale: filled = sqrt(total) / sqrt(max) * width
	sqrtTotal := math.Sqrt(float64(total))
	sqrtMax := math.Sqrt(float64(scaleMaxReference))
	filled := int(math.Round(sqrtTotal / sqrtMax * float64(r.Width)))

	// Ensure minimum 1 block for any change
	if filled < 1 {
		filled = 1
	}
	return filled
}

// buildBar creates the gauge bar with colored add/del blocks.
func (r *GaugeRenderer) buildBar(add, del, filled int) string {
	total := add + del
	if total == 0 || filled == 0 {
		return strings.Repeat(BlockEmpty, r.Width)
	}

	// Split filled portion between adds (green) and dels (red)
	addBlocks := (add * filled) / total
	delBlocks := filled - addBlocks

	// Ensure at least 1 block for non-zero values
	if add > 0 && addBlocks == 0 && filled > 0 {
		addBlocks = 1
		delBlocks = filled - 1
	} else if del > 0 && delBlocks == 0 && filled > 0 {
		delBlocks = 1
		addBlocks = filled - 1
	}

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

	// Pad with empty blocks to fixed width
	if padding := r.Width - filled; padding > 0 {
		sb.WriteString(strings.Repeat(BlockEmpty, padding))
	}

	return sb.String()
}

// renderNumbers formats the add/del counts, omitting zeros.
func (r *GaugeRenderer) renderNumbers(stats *diff.DiffStats) string {
	var sb strings.Builder

	if stats.TotalAdd > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
		sb.WriteString(r.color(ColorReset))
	}
	if stats.TotalDel > 0 {
		if stats.TotalAdd > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
		sb.WriteString(r.color(ColorReset))
	}

	return sb.String()
}

// renderDirPercentages calculates and formats directory distribution.
// Returns format like "src 55% | tests 30% | docs 15%"
func (r *GaugeRenderer) renderDirPercentages(stats *diff.DiffStats, total int) string {
	if total == 0 || len(stats.Files) == 0 {
		return ""
	}

	// Group by top-level directory
	groups := GroupByDepth(stats.Files, 1)
	if len(groups) == 0 {
		return ""
	}

	// Sort directories by total changes descending
	sortedDirs := SortTopDirs(groups)

	// Calculate percentages
	type dirPct struct {
		name string
		pct  int
	}
	pcts := make([]dirPct, 0, len(sortedDirs))
	for _, dir := range sortedDirs {
		dirTotal := 0
		for _, seg := range groups[dir] {
			dirTotal += seg.Total()
		}
		pct := (dirTotal * 100) / total
		// Only include dirs with at least 1%
		if pct >= 1 {
			pcts = append(pcts, dirPct{name: dir, pct: pct})
		}
	}

	if len(pcts) == 0 {
		return ""
	}

	// Build output string
	var sb strings.Builder
	showCount := len(pcts)
	if showCount > r.MaxDirs {
		showCount = r.MaxDirs
	}

	sep := Separator(r.UseColor)
	for i := 0; i < showCount; i++ {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(fmt.Sprintf("%s %d%%", pcts[i].name, pcts[i].pct))
	}

	if len(pcts) > showCount {
		sb.WriteString(fmt.Sprintf(" +%d more", len(pcts)-showCount))
	}

	return sb.String()
}

// color returns the ANSI code if color is enabled.
func (r *GaugeRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
