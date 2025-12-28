package render

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	hotpathBarWidth   = 40 // Width of inline bar
	hotpathBarChar    = "━"
	hotpathTreeBranch = "└── "
	hotpathTreeIndent = "    "
)

// HotpathRenderer shows hot trails through the diff tree.
// For each top-level directory, it follows the single largest child
// at each level down to a leaf, compressing all siblings.
type HotpathRenderer struct {
	UseColor bool
	MaxDepth int // Max depth to follow (0 = unlimited)
	w        io.Writer
}

// NewHotpathRenderer creates a hotpath renderer.
func NewHotpathRenderer(w io.Writer, useColor bool) *HotpathRenderer {
	return &HotpathRenderer{
		UseColor: useColor,
		MaxDepth: 0,
		w:        w,
	}
}

// Render outputs the diff stats as hot trails per top-level directory.
func (r *HotpathRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree and calculate totals
	root := BuildTreeFromFiles(stats.Files)
	CalcTotals(root)
	total := stats.TotalAdd + stats.TotalDel

	// Separate directories from root-level files
	var dirs, files []*TreeNode
	for _, child := range root.Children {
		if child.IsDir {
			dirs = append(dirs, child)
		} else {
			files = append(files, child)
		}
	}

	// Sort each group by magnitude (largest first)
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Add+dirs[i].Del > dirs[j].Add+dirs[j].Del
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Add+files[i].Del > files[j].Add+files[j].Del
	})

	// Render directories with spacing
	for i, dir := range dirs {
		if i > 0 {
			fmt.Fprintln(r.w) // blank line between directories
		}
		r.renderTopLevel(dir, total)
	}

	// Render root-level files grouped at bottom (no spacing)
	if len(files) > 0 && len(dirs) > 0 {
		fmt.Fprintln(r.w) // blank line before files group
	}
	for _, file := range files {
		r.renderTopLevel(file, total)
	}

	// Summary
	fmt.Fprintln(r.w)
	r.renderSummary(stats)
}

// renderTopLevel renders a top-level directory/file with its hot trail.
func (r *HotpathRenderer) renderTopLevel(node *TreeNode, total int) {
	magnitude := node.Add + node.Del
	pct := float64(magnitude) / float64(total) * 100

	// Directory/file name with inline bar
	name := node.Name
	if node.IsDir {
		name += "/"
	}

	bar := r.inlineBar(pct, node.Add, node.Del)
	stats := r.formatStats(node.Add, node.Del)
	pctStr := fmt.Sprintf("(%2.0f%%)", pct)

	fmt.Fprintf(r.w, "%s %s %s %s\n", name, bar, stats, pctStr)

	// If directory, follow the hot trail
	if node.IsDir && len(node.Children) > 0 {
		r.renderHotTrail(node.Children, "  ", 1, total)
	}
}

// renderHotTrail follows the hottest child recursively.
func (r *HotpathRenderer) renderHotTrail(children []*TreeNode, prefix string, depth int, total int) {
	if len(children) == 0 {
		return
	}

	// Check depth limit
	if r.MaxDepth > 0 && depth > r.MaxDepth {
		count := countFiles(children)
		r.renderCompressed(prefix, count)
		return
	}

	// Sort by magnitude
	sorted := make([]*TreeNode, len(children))
	copy(sorted, children)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Add+sorted[i].Del > sorted[j].Add+sorted[j].Del
	})

	hottest := sorted[0]
	others := sorted[1:]

	// Render the hottest child
	r.renderHotNode(hottest, prefix)

	// If hottest is a directory, recurse
	if hottest.IsDir && len(hottest.Children) > 0 {
		newPrefix := prefix + hotpathTreeIndent
		r.renderHotTrail(hottest.Children, newPrefix, depth+1, total)
	}

	// Compress remaining siblings (same indent as hot child content)
	if len(others) > 0 {
		count := countFiles(others)
		r.renderCompressed(prefix, count)
	}
}

// renderHotNode renders a single node on the hot trail.
func (r *HotpathRenderer) renderHotNode(node *TreeNode, prefix string) {
	name := node.Name
	if node.IsDir {
		name += "/"
	}

	stats := r.formatStats(node.Add, node.Del)
	fmt.Fprintf(r.w, "%s%s%s %s\n", prefix, hotpathTreeBranch, name, stats)
}

// renderCompressed renders the "...N more file(s)" line.
func (r *HotpathRenderer) renderCompressed(prefix string, count int) {
	suffix := "file"
	if count != 1 {
		suffix = "files"
	}
	fmt.Fprintf(r.w, "%s...%d more %s\n", prefix, count, suffix)
}

// inlineBar renders a proportional bar using ━ characters.
// Color reflects add/del ratio: green for adds, red for dels, yellow for mixed.
func (r *HotpathRenderer) inlineBar(pct float64, add, del int) string {
	filled := int(math.Round(pct / 100 * float64(hotpathBarWidth)))
	if filled < 1 && pct > 0 {
		filled = 1 // Minimum 1 char if non-zero
	}
	if filled > hotpathBarWidth {
		filled = hotpathBarWidth
	}

	bar := strings.Repeat(hotpathBarChar, filled)

	// Choose color based on add/del ratio
	var barColor string
	if del == 0 {
		barColor = ColorAdd // Pure additions: green
	} else if add == 0 {
		barColor = ColorDel // Pure deletions: red
	} else {
		barColor = ColorNew // Mixed: yellow
	}

	return r.color(barColor) + bar + r.color(ColorReset)
}

// formatStats returns colored +X -Y string.
func (r *HotpathRenderer) formatStats(add, del int) string {
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
func (r *HotpathRenderer) renderSummary(stats *diff.DiffStats) {
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

// countFiles counts total files in a slice of nodes (recursively).
func countFiles(nodes []*TreeNode) int {
	count := 0
	for _, n := range nodes {
		if n.IsDir {
			count += countFiles(n.Children)
		} else {
			count++
		}
	}
	return count
}

// color returns the ANSI code if color is enabled.
func (r *HotpathRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
