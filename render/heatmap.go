package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

// HeatmapRenderer renders diff stats as a 2D heatmap matrix.
// Rows represent top-level directories, columns represent depth levels.
// Cell density indicates magnitude of changes at that depth.
type HeatmapRenderer struct {
	UseColor  bool
	MaxDepth  int // Number of depth columns to show (default 3)
	CellWidth int // Width per cell in characters (default 2)
	w         io.Writer
}

// NewHeatmapRenderer creates a heatmap renderer.
func NewHeatmapRenderer(w io.Writer, useColor bool) *HeatmapRenderer {
	return &HeatmapRenderer{
		UseColor:  useColor,
		MaxDepth:  3,
		CellWidth: 2,
		w:         w,
	}
}

// heatmapCell holds aggregated stats for a directory at a specific depth.
type heatmapCell struct {
	adds int
	dels int
}

// Render outputs the diff stats as a 2D heatmap matrix.
func (r *HeatmapRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree from files
	root := BuildTreeFromFiles(stats.Files)
	CalcTotals(root)

	// Get top-level directories only
	topLevelDirs := r.getTopLevelDirs(root)
	if len(topLevelDirs) == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Calculate stats for each directory at each depth level
	// dirStats[dirName][depth] = heatmapCell
	dirStats := make(map[string][]heatmapCell)
	maxDirNameLen := 0

	for _, dir := range topLevelDirs {
		name := dir.Name
		if len(name) > maxDirNameLen {
			maxDirNameLen = len(name)
		}
		dirStats[name] = r.calculateDepthStats(dir)
	}

	// Find the maximum magnitude across all cells for normalization
	maxMagnitude := r.findMaxMagnitude(dirStats)

	// Render header row with depth labels
	r.renderHeader(maxDirNameLen)

	// Render each directory row
	for _, dir := range topLevelDirs {
		r.renderRow(dir.Name, dir.IsDir, dirStats[dir.Name], maxDirNameLen, maxMagnitude)
	}

	// Summary line
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
		r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
		r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
		stats.TotalFiles)
}

// getTopLevelDirs returns the top-level directories (children of root) sorted by name.
func (r *HeatmapRenderer) getTopLevelDirs(root *TreeNode) []*TreeNode {
	dirs := make([]*TreeNode, 0, len(root.Children))
	for _, child := range root.Children {
		dirs = append(dirs, child)
	}

	// Sort by name for consistent output
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})

	return dirs
}

// calculateDepthStats calculates the total adds/dels at each depth level for a directory.
// Depth 1 is the immediate children of the directory, depth 2 is grandchildren, etc.
func (r *HeatmapRenderer) calculateDepthStats(dir *TreeNode) []heatmapCell {
	stats := make([]heatmapCell, r.MaxDepth)
	r.collectAtDepth(dir, 0, stats)
	return stats
}

// collectAtDepth recursively collects file stats at each depth level.
func (r *HeatmapRenderer) collectAtDepth(node *TreeNode, depth int, stats []heatmapCell) {
	if depth >= r.MaxDepth {
		return
	}
	for _, child := range node.Children {
		if !child.IsDir {
			stats[depth].adds += child.Add
			stats[depth].dels += child.Del
		}
		r.collectAtDepth(child, depth+1, stats)
	}
}

// findMaxMagnitude finds the maximum total (adds+dels) across all cells.
func (r *HeatmapRenderer) findMaxMagnitude(dirStats map[string][]heatmapCell) int {
	maxMag := 0
	for _, depths := range dirStats {
		for _, ds := range depths {
			total := ds.adds + ds.dels
			if total > maxMag {
				maxMag = total
			}
		}
	}
	return maxMag
}

// renderHeader outputs the header row with depth column labels.
func (r *HeatmapRenderer) renderHeader(labelWidth int) {
	// Right-align the directory label column with spaces
	fmt.Fprintf(r.w, "%s", strings.Repeat(" ", labelWidth+1))

	// Render depth column headers (d1, d2, d3, ...)
	for d := 1; d <= r.MaxDepth; d++ {
		header := fmt.Sprintf("d%d", d)
		// Center the header in the cell width (add 2 for spacing between cells)
		cellWidth := r.CellWidth + 2
		padding := cellWidth - len(header)
		leftPad := padding / 2
		rightPad := padding - leftPad
		fmt.Fprintf(r.w, "%s%s%s", strings.Repeat(" ", leftPad), header, strings.Repeat(" ", rightPad))
	}
	fmt.Fprintln(r.w)
}

// renderRow outputs a single directory row with heatmap cells.
func (r *HeatmapRenderer) renderRow(dirName string, isDir bool, depths []heatmapCell, labelWidth, maxMagnitude int) {
	// Right-align directory name with trailing slash for directories
	name := dirName
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	fmt.Fprintf(r.w, "%*s", labelWidth+1, name)

	// Render each depth cell
	for d := 0; d < r.MaxDepth; d++ {
		var ds heatmapCell
		if d < len(depths) {
			ds = depths[d]
		}
		cell := r.formatCell(ds, maxMagnitude)
		fmt.Fprintf(r.w, "  %s", cell)
	}
	fmt.Fprintln(r.w)
}

// formatCell returns the formatted cell string with density character and color.
func (r *HeatmapRenderer) formatCell(ds heatmapCell, maxMagnitude int) string {
	total := ds.adds + ds.dels

	// Get density character based on magnitude relative to max
	char := r.densityChar(total, maxMagnitude)

	// Build the cell (CellWidth characters of the density char)
	cellContent := strings.Repeat(char, r.CellWidth)

	// Apply color based on add/del ratio
	if total == 0 {
		return cellContent // No color for empty cells
	}

	colorCode := r.ratioColor(ds.adds, ds.dels)
	return fmt.Sprintf("%s%s%s", r.color(colorCode), cellContent, r.color(ColorReset))
}

// densityChar returns the appropriate block character based on magnitude.
// Uses the same characters as bar.go: BlockEmpty, BlockLight, BlockMedium, BlockFull
func (r *HeatmapRenderer) densityChar(total, maxMagnitude int) string {
	if total == 0 || maxMagnitude == 0 {
		return BlockEmpty // Light shade for no changes
	}

	// Calculate relative magnitude (0.0 to 1.0)
	ratio := float64(total) / float64(maxMagnitude)

	switch {
	case ratio >= 0.75:
		return BlockFull // Full block for high magnitude
	case ratio >= 0.5:
		return BlockMedium // Dark shade for medium magnitude
	case ratio >= 0.25:
		return BlockLight // Medium shade for low magnitude
	default:
		return BlockEmpty // Light shade for very low
	}
}

// ratioColor returns the color code based on add/del ratio.
// Green for add-heavy, red for del-heavy, yellow for balanced.
func (r *HeatmapRenderer) ratioColor(adds, dels int) string {
	total := adds + dels
	if total == 0 {
		return ColorReset
	}

	addRatio := float64(adds) / float64(total)

	switch {
	case addRatio >= 0.7:
		return ColorAdd // Green for mostly additions
	case addRatio <= 0.3:
		return ColorDel // Red for mostly deletions
	default:
		return ColorNew // Yellow for balanced
	}
}

// color returns the ANSI code if color is enabled.
func (r *HeatmapRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
