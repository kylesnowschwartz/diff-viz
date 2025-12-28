package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	ratioBarWidth     = 10 // Default bar width in characters
	ratioDefaultDepth = 2  // Default aggregation depth
)

// RatioRenderer shows directories with dual-encoding bars where
// bar length represents magnitude and color split shows add/del ratio.
type RatioRenderer struct {
	UseColor bool
	Depth    int // Aggregation depth (default 2)
	barWidth int // Bar width (default 10, not exposed via CLI)
	w        io.Writer
}

// NewRatioRenderer creates a ratio visualization renderer.
func NewRatioRenderer(w io.Writer, useColor bool) *RatioRenderer {
	return &RatioRenderer{
		UseColor: useColor,
		Depth:    ratioDefaultDepth,
		barWidth: ratioBarWidth,
		w:        w,
	}
}

// Render outputs directories with dual-encoding ratio bars.
func (r *RatioRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Group files by depth
	groups := GroupByDepth(stats.Files, r.Depth)
	sortedDirs := SortTopDirs(groups)

	// Build flat list of segments for display
	segments := r.flattenSegments(groups, sortedDirs)

	// Calculate max path length for alignment
	maxPathLen := 0
	for _, seg := range segments {
		pathLen := len(r.formatPath(seg))
		if pathLen > maxPathLen {
			maxPathLen = pathLen
		}
	}

	// Render each segment
	barConfig := DefaultBarConfig(r.barWidth)
	for _, seg := range segments {
		r.renderSegment(seg, maxPathLen, barConfig)
	}

	// Summary line
	r.renderSummary(stats)
}

// flattenSegments creates a flat list of segments for rendering.
// At depth=1, shows only top-level directories.
// At depth=2+, shows subdirectories/files under each top-level.
func (r *RatioRenderer) flattenSegments(groups map[string][]PathSegment, sortedDirs []string) []PathSegment {
	var result []PathSegment

	for _, topDir := range sortedDirs {
		segments := groups[topDir]
		if r.Depth == 1 {
			// At depth 1, aggregate everything under topDir
			var totalAdd, totalDel, totalFiles int
			for _, seg := range segments {
				totalAdd += seg.Add
				totalDel += seg.Del
				totalFiles += seg.FileCount
			}
			result = append(result, PathSegment{
				TopDir:    topDir,
				SubPath:   topDir,
				Add:       totalAdd,
				Del:       totalDel,
				FileCount: totalFiles,
				IsFile:    false,
			})
		} else {
			// At depth 2+, show each segment with full path
			for _, seg := range segments {
				result = append(result, seg)
			}
		}
	}

	return result
}

// formatPath returns the display path for a segment.
func (r *RatioRenderer) formatPath(seg PathSegment) string {
	if r.Depth == 1 || seg.TopDir == seg.SubPath {
		// Top-level: show just the dir name with trailing slash (if not a file)
		if seg.IsFile {
			return seg.SubPath
		}
		return seg.SubPath + "/"
	}

	// Deeper: show topDir/subPath
	if seg.IsFile {
		return seg.TopDir + "/" + seg.SubPath
	}
	return seg.TopDir + "/" + seg.SubPath + "/"
}

// renderSegment outputs a single segment line.
func (r *RatioRenderer) renderSegment(seg PathSegment, maxPathLen int, barConfig BarConfig) {
	var sb strings.Builder

	// Path with trailing slash for directories
	path := r.formatPath(seg)
	pathColor := ColorReset
	if seg.HasNew {
		pathColor = ColorNew
	} else if !seg.IsFile {
		pathColor = ColorDir
	}
	sb.WriteString(r.color(pathColor))
	sb.WriteString(fmt.Sprintf("%-*s", maxPathLen, path))
	sb.WriteString(r.color(ColorReset))

	// Ratio bar
	sb.WriteString("  ")
	total := seg.Add + seg.Del
	filled := barConfig.FilledFor(total)
	block := barConfig.BlockChar(total)
	sb.WriteString(RatioBar(seg.Add, seg.Del, filled, r.barWidth, block, r.color))

	// Stats: +X -Y
	sb.WriteString("  ")
	sb.WriteString(r.formatStats(seg.Add, seg.Del))

	fmt.Fprintln(r.w, sb.String())
}

// formatStats returns colored +X -Y string.
func (r *RatioRenderer) formatStats(add, del int) string {
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
func (r *RatioRenderer) renderSummary(stats *diff.DiffStats) {
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

// color returns the ANSI code if color is enabled.
func (r *RatioRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
