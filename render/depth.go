package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

// depthLevelStats holds the accumulated changes at a specific depth level.
type depthLevelStats struct {
	add int
	del int
}

// DepthRenderer renders diff stats as nested progress gauges by depth level.
// Shows change distribution across directory hierarchy depths.
type DepthRenderer struct {
	UseColor bool
	MaxDepth int // Number of levels to show (default 4)
	Width    int // Gauge width in characters (default 20)
	w        io.Writer
}

// NewDepthRenderer creates a depth gauge renderer.
func NewDepthRenderer(w io.Writer, useColor bool) *DepthRenderer {
	return &DepthRenderer{
		UseColor: useColor,
		MaxDepth: 4,
		Width:    20,
		w:        w,
	}
}

// Render outputs the diff stats as nested depth gauges.
func (r *DepthRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree and calculate totals
	tree := BuildTreeFromFiles(stats.Files)
	CalcTotals(tree)

	// Accumulate changes by depth (depth 1 = root's children, etc.)
	depthTotals := make(map[int]*depthLevelStats)
	r.walkDepths(tree, 0, depthTotals)

	// Find the max depth we have data for
	maxFoundDepth := 0
	for d := range depthTotals {
		if d > maxFoundDepth {
			maxFoundDepth = d
		}
	}

	// Limit to configured MaxDepth
	displayDepth := maxFoundDepth
	if r.MaxDepth > 0 && r.MaxDepth < displayDepth {
		displayDepth = r.MaxDepth
	}

	// Calculate total changes for percentage
	totalChanges := stats.TotalAdd + stats.TotalDel
	if totalChanges == 0 {
		totalChanges = 1 // Avoid division by zero
	}

	// Render each depth level
	for depth := 1; depth <= displayDepth; depth++ {
		ds, ok := depthTotals[depth]
		if !ok {
			ds = &depthLevelStats{}
		}

		levelTotal := ds.add + ds.del
		percentage := (levelTotal * 100) / totalChanges

		// Build the gauge bar
		gauge := r.buildGauge(ds.add, ds.del, levelTotal, totalChanges)

		// Format stats
		statsStr := r.formatStats(ds.add, ds.del)

		fmt.Fprintf(r.w, "Depth %d: %s  %3d%%  %s\n", depth, gauge, percentage, statsStr)
	}

	// Summary line
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
		r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
		r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
		stats.TotalFiles)
}

// walkDepths recursively walks the tree and accumulates changes by depth.
// Depth 0 is the virtual root, depth 1 is root's children, etc.
func (r *DepthRenderer) walkDepths(node *TreeNode, depth int, totals map[int]*depthLevelStats) {
	// Only count non-directory nodes (files) OR directory totals at leaf level
	if !node.IsDir {
		// This is a file - add its changes to this depth
		if _, ok := totals[depth]; !ok {
			totals[depth] = &depthLevelStats{}
		}
		totals[depth].add += node.Add
		totals[depth].del += node.Del
	}

	// Recurse into children
	for _, child := range node.Children {
		r.walkDepths(child, depth+1, totals)
	}
}

// buildGauge creates the visual gauge bar using dual-color encoding.
// The bar is split proportionally between green (adds) and red (dels).
func (r *DepthRenderer) buildGauge(add, del, levelTotal, grandTotal int) string {
	if grandTotal == 0 {
		grandTotal = 1
	}

	// Calculate how many blocks to fill based on proportion of total changes
	filledCount := (levelTotal * r.Width) / grandTotal
	if levelTotal > 0 && filledCount == 0 {
		filledCount = 1 // Show at least one block if there are changes
	}

	return RatioBar(add, del, filledCount, r.Width, BlockFull, r.color)
}

// formatStats formats the +N -M stats with colors.
func (r *DepthRenderer) formatStats(add, del int) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s+%d%s", r.color(ColorAdd), add, r.color(ColorReset)))
	parts = append(parts, fmt.Sprintf("%s-%d%s", r.color(ColorDel), del, r.color(ColorReset)))
	return strings.Join(parts, " ")
}

// color returns the ANSI code if color is enabled, empty string otherwise.
func (r *DepthRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
