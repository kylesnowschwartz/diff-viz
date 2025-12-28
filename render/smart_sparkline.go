package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

const (
	smartBarWidth    = 8  // Slightly smaller bars for density
	smartMinColWidth = 30 // Minimum column width (path + bar + spacing)
)

// SmartSparklineRenderer renders diff stats as a multi-column table.
// Files are sorted by magnitude and arranged in columns to efficiently
// use terminal width while maintaining readability.
type SmartSparklineRenderer struct {
	UseColor bool
	Width    int // Terminal width for column calculation
	MaxDepth int // Aggregation depth (1=dirs only, 2+=show files)
	w        io.Writer
}

// NewSmartSparklineRenderer creates a multi-column smart renderer.
func NewSmartSparklineRenderer(w io.Writer, useColor bool) *SmartSparklineRenderer {
	return &SmartSparklineRenderer{
		UseColor: useColor,
		Width:    140, // Default to 3-column layout on wide terminals
		MaxDepth: 2,
		w:        w,
	}
}

// smartEntry represents a single item to display.
type smartEntry struct {
	path  string
	add   int
	del   int
	total int
}

// Render outputs diff stats as a multi-column table.
func (r *SmartSparklineRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build entries based on depth
	entries := r.buildEntries(stats)
	if len(entries) == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Sort by total changes descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].total > entries[j].total
	})

	// Calculate column layout
	maxPathLen := 0
	for _, e := range entries {
		if len(e.path) > maxPathLen {
			maxPathLen = len(e.path)
		}
	}

	// Column width: path + 2 spaces + bar + 2 spaces minimum
	colWidth := maxPathLen + 2 + smartBarWidth + 2
	if colWidth < smartMinColWidth {
		colWidth = smartMinColWidth
	}

	// How many columns fit?
	numCols := r.Width / colWidth
	if numCols < 1 {
		numCols = 1
	}

	// Cap path display width to leave room for bar
	displayPathWidth := colWidth - smartBarWidth - 4
	if displayPathWidth > maxPathLen {
		displayPathWidth = maxPathLen
	}

	// Render in column-major order (read down, then right)
	numRows := (len(entries) + numCols - 1) / numCols
	barConfig := DefaultBarConfig(smartBarWidth)

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			idx := col*numRows + row
			if idx >= len(entries) {
				break
			}

			e := entries[idx]
			r.renderEntry(e, displayPathWidth, barConfig, col < numCols-1)
		}
		fmt.Fprintln(r.w)
	}

	// Summary
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
		r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
		r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
		stats.TotalFiles)
}

// buildEntries creates the list of items to display based on depth.
func (r *SmartSparklineRenderer) buildEntries(stats *diff.DiffStats) []smartEntry {
	depth := r.MaxDepth
	if depth < 1 {
		depth = 1
	}

	if depth == 1 {
		// Aggregate by top-level directory
		return r.buildAggregatedEntries(stats)
	}

	// Show individual files with directory prefix
	return r.buildFileEntries(stats, depth)
}

// buildAggregatedEntries aggregates files by top-level directory.
func (r *SmartSparklineRenderer) buildAggregatedEntries(stats *diff.DiffStats) []smartEntry {
	groups := GroupByDepth(stats.Files, 1)
	entries := make([]smartEntry, 0, len(groups))

	for dir, segments := range groups {
		// Sum all segments in this group
		var add, del int
		for _, seg := range segments {
			add += seg.Add
			del += seg.Del
		}

		name := dir
		if len(segments) > 1 || (len(segments) == 1 && !segments[0].IsFile) {
			name = fmt.Sprintf("%s(%d)", dir, len(segments))
		}

		entries = append(entries, smartEntry{
			path:  name,
			add:   add,
			del:   del,
			total: add + del,
		})
	}

	return entries
}

// buildFileEntries creates entries for individual files or depth-truncated paths.
func (r *SmartSparklineRenderer) buildFileEntries(stats *diff.DiffStats, depth int) []smartEntry {
	// Aggregate by depth-truncated path
	pathStats := make(map[string]*smartEntry)

	for _, f := range stats.Files {
		truncPath := truncatePathToDepth(f.Path, depth)

		if existing, ok := pathStats[truncPath]; ok {
			existing.add += f.Additions
			existing.del += f.Deletions
			existing.total += f.Additions + f.Deletions
		} else {
			pathStats[truncPath] = &smartEntry{
				path:  truncPath,
				add:   f.Additions,
				del:   f.Deletions,
				total: f.Additions + f.Deletions,
			}
		}
	}

	entries := make([]smartEntry, 0, len(pathStats))
	for _, e := range pathStats {
		entries = append(entries, *e)
	}

	return entries
}

// truncatePathToDepth returns the path truncated to the given depth.
// depth=1: "src" (top-level only)
// depth=2: "src/lib" or "src/main.go"
// depth=3: "src/lib/utils" or "src/lib/file.go"
func truncatePathToDepth(path string, depth int) string {
	parts := strings.Split(path, "/")
	if depth >= len(parts) {
		return path // Full path if shallower than depth
	}
	return strings.Join(parts[:depth], "/")
}

// renderEntry outputs a single entry with path and bar.
func (r *SmartSparklineRenderer) renderEntry(e smartEntry, pathWidth int, barConfig BarConfig, addSpacer bool) {
	// Truncate or pad path
	path := e.path
	if len(path) > pathWidth {
		path = path[:pathWidth-1] + "~"
	}

	// Color based on content
	pathColor := ColorDir
	// Check if it looks like a file (has extension or no trailing number in parens)
	if !isAggregatedPath(path) {
		pathColor = ColorReset
	}

	fmt.Fprintf(r.w, "%s%-*s%s  ", r.color(pathColor), pathWidth, path, r.color(ColorReset))

	// Bar
	filled := barConfig.FilledFor(e.total)
	block := barConfig.BlockChar(e.total)
	fmt.Fprint(r.w, RatioBar(e.add, e.del, filled, smartBarWidth, block, r.color))

	if addSpacer {
		fmt.Fprint(r.w, "  ")
	}
}

// isAggregatedPath returns true if the path looks like an aggregated group.
func isAggregatedPath(path string) bool {
	if len(path) < 3 {
		return false
	}
	// Check for trailing (N) pattern
	return path[len(path)-1] == ')' && path[len(path)-2] >= '0' && path[len(path)-2] <= '9'
}

// color returns the ANSI code if color is enabled.
func (r *SmartSparklineRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
