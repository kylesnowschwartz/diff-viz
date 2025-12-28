package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	sparklineBarWidth     = 10 // Width of sparkline bar
	sparklineDefaultDepth = 2  // Default aggregation depth
	sparklineDefaultN     = 10 // Default file limit for --files mode
)

// SortBy specifies the sorting criteria for file mode.
type SortBy string

const (
	SortByTotal SortBy = "total" // Sort by total changes (adds + dels)
	SortByAdds  SortBy = "adds"  // Sort by additions only
	SortByDels  SortBy = "dels"  // Sort by deletions only
)

// sidebarColors cycles through these colors for the rainbow sidebar.
var sidebarColors = []string{
	"\033[36m", // Cyan
	"\033[33m", // Yellow
	"\033[35m", // Magenta
	"\033[32m", // Green
	"\033[34m", // Blue
}

// SparklineTreeRenderer uses proper tree structure for hierarchical display.
type SparklineTreeRenderer struct {
	UseColor  bool
	MaxDepth  int    // How deep to render (0 = unlimited)
	ShowFiles bool   // --files flag switches to flat file mode
	N         int    // File limit for --files mode
	SortBy    SortBy // Sort order for --files mode
	w         io.Writer
}

// NewSparklineTreeRenderer creates a sparkline tree renderer.
func NewSparklineTreeRenderer(w io.Writer, useColor bool) *SparklineTreeRenderer {
	return &SparklineTreeRenderer{
		UseColor: useColor,
		MaxDepth: 2,
		N:        sparklineDefaultN,
		SortBy:   SortByTotal,
		w:        w,
	}
}

// Render outputs the diff stats as a proper hierarchical tree.
func (r *SparklineTreeRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	if r.ShowFiles {
		r.renderFileMode(stats)
		return
	}

	// Build proper tree structure
	root := BuildTreeFromFiles(stats.Files)
	CalcTotals(root)

	// Sort children by total changes (descending) at each level
	sortTreeByTotal(root)

	// Calculate max display width for alignment
	maxWidth := r.calcMaxWidth(root, 0)

	// Assign colors to top-level groups
	colorMap := r.assignColors(root)

	// Render the tree
	barConfig := DefaultBarConfig(sparklineBarWidth)
	for _, child := range root.Children {
		r.renderNode(child, 0, colorMap[child.Name], maxWidth, barConfig)
	}

	// Summary
	r.renderSummary(stats)
}

// sortTreeByTotal sorts children at each level by total changes descending.
func sortTreeByTotal(node *TreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Add+node.Children[i].Del > node.Children[j].Add+node.Children[j].Del
	})
	for _, child := range node.Children {
		sortTreeByTotal(child)
	}
}

// assignColors gives each top-level item a color. Root files share one color.
func (r *SparklineTreeRenderer) assignColors(root *TreeNode) map[string]string {
	colorMap := make(map[string]string)
	colorIdx := 0
	var rootFiles []string

	for _, child := range root.Children {
		if child.IsDir {
			colorMap[child.Name] = sidebarColors[colorIdx%len(sidebarColors)]
			colorIdx++
		} else {
			rootFiles = append(rootFiles, child.Name)
		}
	}

	// All root files share one color
	if len(rootFiles) > 0 {
		rootColor := sidebarColors[colorIdx%len(sidebarColors)]
		for _, name := range rootFiles {
			colorMap[name] = rootColor
		}
	}

	return colorMap
}

// calcMaxWidth finds the widest display path for alignment.
func (r *SparklineTreeRenderer) calcMaxWidth(node *TreeNode, depth int) int {
	maxWidth := 0

	for _, child := range node.Children {
		// Width: sidebar (2) + indent (depth*2) + name + possible "/"
		width := 2 + depth*2 + len(child.Name)
		if child.IsDir {
			width++ // trailing slash
		}
		if width > maxWidth {
			maxWidth = width
		}

		// Recurse if within depth limit
		if child.IsDir && (r.MaxDepth == 0 || depth+1 < r.MaxDepth) {
			childMax := r.calcMaxWidth(child, depth+1)
			if childMax > maxWidth {
				maxWidth = childMax
			}
		}
	}

	return maxWidth
}

// renderNode renders a single tree node and its children.
func (r *SparklineTreeRenderer) renderNode(node *TreeNode, depth int, sidebarColor string, maxWidth int, barConfig BarConfig) {
	var sb strings.Builder

	// Rainbow sidebar
	sb.WriteString(r.color(sidebarColor))
	sb.WriteString(BlockFull)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Indent based on depth
	indent := strings.Repeat("  ", depth)
	sb.WriteString(indent)

	// Name with appropriate color
	name := node.Name
	if node.IsDir {
		name += "/"
	}

	nameColor := ColorReset
	if node.IsUntracked {
		nameColor = ColorNew
	} else if node.IsDir {
		nameColor = ColorDir
	}

	sb.WriteString(r.color(nameColor))
	// Pad for alignment: maxWidth - sidebar(2) - indent
	padWidth := maxWidth - 2 - len(indent)
	sb.WriteString(fmt.Sprintf("%-*s", padWidth, name))
	sb.WriteString(r.color(ColorReset))

	// Sparkline bar
	sb.WriteString("  ")
	total := node.Add + node.Del
	filled := barConfig.FilledFor(total)
	block := barConfig.BlockChar(total)
	sb.WriteString(RatioBar(node.Add, node.Del, filled, sparklineBarWidth, block, r.color))

	// Stats
	sb.WriteString("  ")
	sb.WriteString(r.formatStats(node.Add, node.Del))

	fmt.Fprintln(r.w, sb.String())

	// Render children if directory and within depth limit
	if node.IsDir && (r.MaxDepth == 0 || depth+1 < r.MaxDepth) {
		for _, child := range node.Children {
			r.renderNode(child, depth+1, sidebarColor, maxWidth, barConfig)
		}
	}
}

// formatStats returns colored +X -Y string.
func (r *SparklineTreeRenderer) formatStats(add, del int) string {
	var sb strings.Builder

	if add > 0 {
		sb.WriteString(r.color(ColorAdd))
		sb.WriteString(fmt.Sprintf("+%d", add))
		sb.WriteString(r.color(ColorReset))
	}

	if add > 0 && del > 0 {
		sb.WriteString(" ")
	}

	if del > 0 {
		sb.WriteString(r.color(ColorDel))
		sb.WriteString(fmt.Sprintf("-%d", del))
		sb.WriteString(r.color(ColorReset))
	}

	return sb.String()
}

// renderSummary outputs the totals line.
func (r *SparklineTreeRenderer) renderSummary(stats *diff.DiffStats) {
	fmt.Fprintln(r.w)

	var sb strings.Builder
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(fmt.Sprintf(" in %d files", stats.TotalFiles))

	fmt.Fprintln(r.w, sb.String())
}

// renderFileMode shows a flat list of files sorted by change size.
func (r *SparklineTreeRenderer) renderFileMode(stats *diff.DiffStats) {
	// Sort files by configured criteria (descending)
	files := make([]diff.FileStat, len(stats.Files))
	copy(files, stats.Files)
	sort.Slice(files, func(i, j int) bool {
		return r.sortValue(files[i]) > r.sortValue(files[j])
	})

	// Take top N
	showCount := min(r.N, len(files))
	topFiles := files[:showCount]

	// Calculate max path length for alignment
	maxPathLen := 0
	for _, f := range topFiles {
		maxPathLen = max(maxPathLen, len(f.Path))
	}

	// Render each file
	barConfig := DefaultBarConfig(sparklineBarWidth)
	for _, f := range topFiles {
		var sb strings.Builder

		pathColor := ColorReset
		if f.IsUntracked {
			pathColor = ColorNew
		}
		sb.WriteString(r.color(pathColor))
		sb.WriteString(fmt.Sprintf("%-*s", maxPathLen, f.Path))
		sb.WriteString(r.color(ColorReset))

		sb.WriteString("  ")
		total := f.Additions + f.Deletions
		filled := barConfig.FilledFor(total)
		block := barConfig.BlockChar(total)
		sb.WriteString(RatioBar(f.Additions, f.Deletions, filled, sparklineBarWidth, block, r.color))

		sb.WriteString("  ")
		sb.WriteString(r.formatStats(f.Additions, f.Deletions))

		fmt.Fprintln(r.w, sb.String())
	}

	// Summary with hidden file context
	fmt.Fprintln(r.w)
	hiddenCount := stats.TotalFiles - showCount

	var sb strings.Builder
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))

	if hiddenCount > 0 {
		sb.WriteString(fmt.Sprintf(" (%d of %d files)", showCount, stats.TotalFiles))
	} else {
		sb.WriteString(fmt.Sprintf(" (%d files)", stats.TotalFiles))
	}

	fmt.Fprintln(r.w, sb.String())
}

// sortValue returns the value to sort by for a file.
func (r *SparklineTreeRenderer) sortValue(f diff.FileStat) int {
	switch r.SortBy {
	case SortByAdds:
		return f.Additions
	case SortByDels:
		return f.Deletions
	default:
		return f.Additions + f.Deletions
	}
}

// color returns the ANSI code if color is enabled.
func (r *SparklineTreeRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
