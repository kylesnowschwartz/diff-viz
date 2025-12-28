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
// Each top-level directory gets the next color in sequence.
var sidebarColors = []string{
	"\033[36m", // Cyan
	"\033[33m", // Yellow
	"\033[35m", // Magenta
	"\033[32m", // Green
	"\033[34m", // Blue
}

// SparklineTreeRenderer shows changes with sparkline bars in tree or flat mode.
type SparklineTreeRenderer struct {
	UseColor  bool
	MaxDepth  int    // Aggregation depth for dir mode (default 2)
	ShowFiles bool   // --files flag switches to flat file mode
	N         int    // File limit for --files mode (default 10)
	SortBy    SortBy // Sort order for --files mode
	w         io.Writer
}

// NewSparklineTreeRenderer creates a sparkline tree renderer.
func NewSparklineTreeRenderer(w io.Writer, useColor bool) *SparklineTreeRenderer {
	return &SparklineTreeRenderer{
		UseColor: useColor,
		MaxDepth: sparklineDefaultDepth,
		N:        sparklineDefaultN,
		SortBy:   SortByTotal,
		w:        w,
	}
}

// Render outputs the diff stats as sparkline tree or flat file list.
func (r *SparklineTreeRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	if r.ShowFiles {
		r.renderFileMode(stats)
	} else {
		r.renderDirMode(stats)
	}
}

// renderDirMode shows a tree with rainbow sidebar and indentation.
func (r *SparklineTreeRenderer) renderDirMode(stats *diff.DiffStats) {
	// Group files by depth
	groups := GroupByDepth(stats.Files, r.MaxDepth)
	sortedDirs := SortTopDirs(groups)

	// Calculate max path length for alignment (including indent)
	maxPathLen := 0
	for _, topDir := range sortedDirs {
		segments := groups[topDir]
		// Check if we need a header (has child segments that aren't the topDir itself)
		hasChildren := false
		for _, seg := range segments {
			if seg.TopDir != seg.SubPath {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			// Account for header line
			headerLen := 3 + len(topDir) + 1 // "█ topDir/"
			if headerLen > maxPathLen {
				maxPathLen = headerLen
			}
		}
		for _, seg := range segments {
			// +3 for "█ " sidebar, +2 per indent level
			indent := r.indentForSegment(seg, topDir, hasChildren)
			pathLen := 3 + len(indent) + len(r.formatPath(seg, topDir))
			if pathLen > maxPathLen {
				maxPathLen = pathLen
			}
		}
	}

	// Render each top-level directory and its contents
	barConfig := DefaultBarConfig(sparklineBarWidth)
	for dirIdx, topDir := range sortedDirs {
		segments := groups[topDir]
		sidebarColor := sidebarColors[dirIdx%len(sidebarColors)]

		// Check if we need to render a header (group has child segments)
		hasChildren := false
		var totalAdd, totalDel int
		for _, seg := range segments {
			if seg.TopDir != seg.SubPath {
				hasChildren = true
			}
			totalAdd += seg.Add
			totalDel += seg.Del
		}

		if hasChildren {
			// Render directory header
			r.renderDirHeader(topDir, totalAdd, totalDel, sidebarColor, maxPathLen, barConfig)
		}

		// Render each segment
		for _, seg := range segments {
			r.renderSegment(seg, topDir, sidebarColor, maxPathLen, barConfig, hasChildren)
		}
	}

	// Summary line
	r.renderSummary(stats)
}

// renderDirHeader renders the directory header line.
func (r *SparklineTreeRenderer) renderDirHeader(topDir string, totalAdd, totalDel int, sidebarColor string, maxPathLen int, barConfig BarConfig) {
	var sb strings.Builder

	// Rainbow sidebar block
	sb.WriteString(r.color(sidebarColor))
	sb.WriteString(BlockFull)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Directory name (no indent for header)
	sb.WriteString(r.color(ColorDir))
	path := topDir + "/"
	paddedWidth := maxPathLen - 3 // -3 for "█ " sidebar
	sb.WriteString(fmt.Sprintf("%-*s", paddedWidth, path))
	sb.WriteString(r.color(ColorReset))

	// Sparkline bar
	sb.WriteString("  ")
	total := totalAdd + totalDel
	filled := barConfig.FilledFor(total)
	block := barConfig.BlockChar(total)
	sb.WriteString(RatioBar(totalAdd, totalDel, filled, sparklineBarWidth, block, r.color))

	// Stats: +X -Y
	sb.WriteString("  ")
	sb.WriteString(r.formatStats(totalAdd, totalDel))

	fmt.Fprintln(r.w, sb.String())
}

// indentForSegment returns the indent string based on segment depth.
func (r *SparklineTreeRenderer) indentForSegment(seg PathSegment, topDir string, hasParentHeader bool) string {
	// Top-level items (TopDir == SubPath) get no indent
	// Children get 2 spaces indent if there's a parent header
	if seg.TopDir == seg.SubPath {
		return ""
	}
	if hasParentHeader {
		return "  "
	}
	return ""
}

// formatPath returns the display path for a segment.
func (r *SparklineTreeRenderer) formatPath(seg PathSegment, topDir string) string {
	if seg.TopDir == seg.SubPath {
		// Top-level: show just the name
		if seg.IsFile {
			return seg.SubPath
		}
		return seg.SubPath + "/"
	}

	// Child: show just the subpath (indent provides context)
	if seg.IsFile {
		return seg.SubPath
	}
	return seg.SubPath + "/"
}

// renderSegment outputs a single segment line with sidebar.
func (r *SparklineTreeRenderer) renderSegment(seg PathSegment, topDir string, sidebarColor string, maxPathLen int, barConfig BarConfig, hasParentHeader bool) {
	// Skip if this is the aggregate entry and we already rendered a header
	if hasParentHeader && seg.TopDir == seg.SubPath {
		return
	}

	var sb strings.Builder

	// Rainbow sidebar block
	sb.WriteString(r.color(sidebarColor))
	sb.WriteString(BlockFull)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")

	// Indent
	indent := r.indentForSegment(seg, topDir, hasParentHeader)
	sb.WriteString(indent)

	// Path with appropriate color
	path := r.formatPath(seg, topDir)
	pathColor := ColorReset
	if seg.HasNew {
		pathColor = ColorNew
	} else if !seg.IsFile {
		pathColor = ColorDir
	}
	sb.WriteString(r.color(pathColor))
	// Pad to align bars (account for sidebar + indent)
	paddedWidth := maxPathLen - 3 - len(indent)
	sb.WriteString(fmt.Sprintf("%-*s", paddedWidth, path))
	sb.WriteString(r.color(ColorReset))

	// Sparkline bar
	sb.WriteString("  ")
	total := seg.Add + seg.Del
	filled := barConfig.FilledFor(total)
	block := barConfig.BlockChar(total)
	sb.WriteString(RatioBar(seg.Add, seg.Del, filled, sparklineBarWidth, block, r.color))

	// Stats: +X -Y
	sb.WriteString("  ")
	sb.WriteString(r.formatStats(seg.Add, seg.Del))

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
		r.renderFile(f, maxPathLen, barConfig)
	}

	// Summary line with hidden file context
	r.renderFileSummary(stats, showCount)
}

// renderFile outputs a single file line (flat mode).
func (r *SparklineTreeRenderer) renderFile(f diff.FileStat, maxPathLen int, barConfig BarConfig) {
	var sb strings.Builder

	// Path
	pathColor := ColorReset
	if f.IsUntracked {
		pathColor = ColorNew
	}
	sb.WriteString(r.color(pathColor))
	sb.WriteString(fmt.Sprintf("%-*s", maxPathLen, f.Path))
	sb.WriteString(r.color(ColorReset))

	// Sparkline bar
	sb.WriteString("  ")
	total := f.Additions + f.Deletions
	filled := barConfig.FilledFor(total)
	block := barConfig.BlockChar(total)
	sb.WriteString(RatioBar(f.Additions, f.Deletions, filled, sparklineBarWidth, block, r.color))

	// Stats: +X -Y
	sb.WriteString("  ")
	sb.WriteString(r.formatStats(f.Additions, f.Deletions))

	fmt.Fprintln(r.w, sb.String())
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

// renderSummary outputs the totals line (dir mode).
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

// renderFileSummary outputs the totals line with hidden file context (file mode).
func (r *SparklineTreeRenderer) renderFileSummary(stats *diff.DiffStats, shown int) {
	fmt.Fprintln(r.w)

	hiddenCount := stats.TotalFiles - shown

	var sb strings.Builder
	sb.WriteString(r.color(ColorAdd))
	sb.WriteString(fmt.Sprintf("+%d", stats.TotalAdd))
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(" ")
	sb.WriteString(r.color(ColorDel))
	sb.WriteString(fmt.Sprintf("-%d", stats.TotalDel))
	sb.WriteString(r.color(ColorReset))

	if hiddenCount > 0 {
		sb.WriteString(fmt.Sprintf(" (%d of %d files)", shown, stats.TotalFiles))
	} else {
		sb.WriteString(fmt.Sprintf(" (%d files)", stats.TotalFiles))
	}

	fmt.Fprintln(r.w, sb.String())
}

// color returns the ANSI code if color is enabled.
func (r *SparklineTreeRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
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
