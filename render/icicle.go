package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// BoxStyle holds the Unicode box-drawing characters for rendering.
type BoxStyle struct {
	TopLeft     string // ┌
	TopRight    string // ┐
	BottomLeft  string // └
	BottomRight string // ┘
	LeftSep     string // ├
	RightSep    string // ┤
	TopSep      string // ┬
	BottomSep   string // ┴
	Cross       string // ┼
	Horizontal  string // ─
	Vertical    string // │
}

// DefaultBoxStyle returns the standard light box style.
func DefaultBoxStyle() BoxStyle {
	return BoxStyle{
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		LeftSep:     "├",
		RightSep:    "┤",
		TopSep:      "┬",
		BottomSep:   "┴",
		Cross:       "┼",
		Horizontal:  "─",
		Vertical:    "│",
	}
}

// ASCIIBoxStyle returns ASCII-safe box characters.
func ASCIIBoxStyle() BoxStyle {
	return BoxStyle{
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
		LeftSep:     "+",
		RightSep:    "+",
		TopSep:      "+",
		BottomSep:   "+",
		Cross:       "+",
		Horizontal:  "-",
		Vertical:    "|",
	}
}

// IcicleRenderer renders diff stats as a horizontal icicle/flame chart.
// Uses D3-style separated layout/render architecture for efficiency.
type IcicleRenderer struct {
	UseColor     bool
	Width        int // Total width of the chart
	MaxDepth     int // Maximum depth levels to render (0 = unlimited)
	MinCellWidth int // Minimum width per cell (wider = less visual clutter)
	w            io.Writer
	style        BoxStyle
}

// NewIcicleRenderer creates an icicle renderer.
func NewIcicleRenderer(w io.Writer, useColor bool) *IcicleRenderer {
	style := DefaultBoxStyle()
	if !useColor {
		style = ASCIIBoxStyle()
	}
	return &IcicleRenderer{
		UseColor:     useColor,
		Width:        100, // Default width (standard terminal)
		MaxDepth:     4,   // Default max depth (shows 4 hierarchy levels)
		MinCellWidth: 12,  // Default min cell width
		w:            w,
		style:        style,
	}
}

// Render outputs the diff stats as a horizontal icicle chart.
func (r *IcicleRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Phase 1: Build tree
	tree := r.buildTree(stats.Files)

	// Phase 2: Compute layout (D3-style separated phase)
	layout := ComputeLayout(tree, r.Width, r.MaxDepth, r.MinCellWidth)
	if len(layout.Cells) == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Phase 3: Render using pre-computed layout
	r.renderBorder(layout, 0, true)

	lastLevel := len(layout.Cells) - 1
	for depth := 0; depth < len(layout.Cells); depth++ {
		r.renderContentRow(layout, depth)

		if depth < lastLevel {
			r.renderSeparator(layout, depth, depth+1)
		}
	}

	// Render stats footer row (aligned to leaf cell columns)
	leafCells := CollectLeafCells(layout)
	r.renderLeafSeparator(layout, lastLevel, leafCells)
	r.renderStatsFooterFromCells(leafCells)
	r.renderLeafBorder(leafCells)

	// Summary line
	if layout.Dropped > 0 {
		fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files (%d hidden)\n",
			r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
			r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
			stats.TotalFiles, layout.Dropped)
	} else {
		fmt.Fprintf(r.w, "%s+%d%s %s-%d%s in %d files\n",
			r.color(ColorAdd), stats.TotalAdd, r.color(ColorReset),
			r.color(ColorDel), stats.TotalDel, r.color(ColorReset),
			stats.TotalFiles)
	}
}

// buildTree constructs a tree from flat file paths.
func (r *IcicleRenderer) buildTree(files []diff.FileStat) *TreeNode {
	root := BuildTreeFromFiles(files)
	CalcTotals(root)
	CollapseSingleChildPaths(root)
	return root
}

// renderBorder renders the top or bottom border.
func (r *IcicleRenderer) renderBorder(layout *Layout, levelIdx int, isTop bool) {
	boundaries := layout.Boundaries[levelIdx]

	var sb strings.Builder
	sb.Grow(r.Width + 10)

	if isTop {
		sb.WriteString(r.style.TopLeft)
	} else {
		sb.WriteString(r.style.BottomLeft)
	}

	for pos := 1; pos < r.Width-1; pos++ {
		if HasBoundary(boundaries, pos) {
			if isTop {
				sb.WriteString(r.style.TopSep)
			} else {
				sb.WriteString(r.style.BottomSep)
			}
		} else {
			sb.WriteString(r.style.Horizontal)
		}
	}

	if isTop {
		sb.WriteString(r.style.TopRight)
	} else {
		sb.WriteString(r.style.BottomRight)
	}

	fmt.Fprintln(r.w, sb.String())
}

// renderContentRow renders the content row for a level.
func (r *IcicleRenderer) renderContentRow(layout *Layout, levelIdx int) {
	level := layout.Cells[levelIdx]

	// Get parent boundaries to draw separators in empty regions
	var parentBoundaries []int
	if levelIdx > 0 {
		parentBoundaries = layout.Boundaries[levelIdx-1]
	}

	var sb strings.Builder
	sb.Grow(r.Width + 64)
	sb.WriteString(r.style.Vertical)

	pos := 1 // Start after left border
	for i, cell := range level {
		// Fill gap before cell, respecting parent boundaries
		for pos < cell.X0+1 { // +1 for border offset
			if HasBoundary(parentBoundaries, pos) {
				sb.WriteString(r.style.Vertical)
			} else {
				sb.WriteString(" ")
			}
			pos++
		}

		// Render centered, colored cell content
		content, visualWidth := r.formatCentered(cell, cell.Width(), 1)
		sb.WriteString(content)
		pos = cell.X0 + 1 + visualWidth

		// Cell separator (not after last cell)
		if i < len(level)-1 {
			sb.WriteString(r.style.Vertical)
			pos++
		}
	}

	// Fill remaining space, respecting parent boundaries
	for pos < r.Width-1 {
		if HasBoundary(parentBoundaries, pos) {
			sb.WriteString(r.style.Vertical)
		} else {
			sb.WriteString(" ")
		}
		pos++
	}

	sb.WriteString(r.style.Vertical)
	fmt.Fprintln(r.w, sb.String())
}

// renderSeparator renders the separator row between two levels.
func (r *IcicleRenderer) renderSeparator(layout *Layout, aboveIdx, belowIdx int) {
	aboveBoundaries := layout.Boundaries[aboveIdx]
	belowBoundaries := layout.Boundaries[belowIdx]

	var sb strings.Builder
	sb.Grow(r.Width + 10)
	sb.WriteString(r.style.LeftSep)

	for pos := 1; pos < r.Width-1; pos++ {
		above := HasBoundary(aboveBoundaries, pos)
		below := HasBoundary(belowBoundaries, pos)

		switch {
		case above && below:
			sb.WriteString(r.style.Cross)
		case above:
			sb.WriteString(r.style.BottomSep)
		case below:
			sb.WriteString(r.style.TopSep)
		default:
			sb.WriteString(r.style.Horizontal)
		}
	}

	sb.WriteString(r.style.RightSep)
	fmt.Fprintln(r.w, sb.String())
}

// renderLeafSeparator renders the separator between the last content row and footer.
func (r *IcicleRenderer) renderLeafSeparator(layout *Layout, lastLevelIdx int, leaves []LayoutCell) {
	aboveBoundaries := layout.Boundaries[lastLevelIdx]
	leafBoundaries := GetLeafBoundaries(leaves, r.Width-2)

	var sb strings.Builder
	sb.Grow(r.Width + 10)
	sb.WriteString(r.style.LeftSep)

	for pos := 1; pos < r.Width-1; pos++ {
		above := HasBoundary(aboveBoundaries, pos)
		below := leafBoundaries[pos]

		switch {
		case above && below:
			sb.WriteString(r.style.Cross)
		case above:
			sb.WriteString(r.style.BottomSep)
		case below:
			sb.WriteString(r.style.TopSep)
		default:
			sb.WriteString(r.style.Horizontal)
		}
	}

	sb.WriteString(r.style.RightSep)
	fmt.Fprintln(r.w, sb.String())
}

// renderStatsFooterFromCells renders the stats row from pre-collected leaf cells.
func (r *IcicleRenderer) renderStatsFooterFromCells(leaves []LayoutCell) {
	var sb strings.Builder
	sb.Grow(r.Width + 64)
	sb.WriteString(r.style.Vertical)

	pos := 1
	for i, cell := range leaves {
		// Fill gap before cell
		for pos < cell.X0+1 {
			sb.WriteString(" ")
			pos++
		}

		// Format stats with colors
		addPart := fmt.Sprintf("+%d", cell.Add)
		delPart := ""
		if cell.Del > 0 {
			delPart = fmt.Sprintf(" -%d", cell.Del)
		}

		statsLen := utf8.RuneCountInString(addPart + delPart)
		cellWidth := cell.Width()
		availWidth := cellWidth - 1

		// Build colored stats string
		var coloredStats strings.Builder
		coloredStats.WriteString(r.color(ColorAdd))
		coloredStats.WriteString(addPart)
		coloredStats.WriteString(r.color(ColorReset))
		if delPart != "" {
			coloredStats.WriteString(r.color(ColorDel))
			coloredStats.WriteString(delPart)
			coloredStats.WriteString(r.color(ColorReset))
		}

		// Truncate if needed
		if statsLen > availWidth {
			plainStats := addPart + delPart
			plainStats = plainStats[:availWidth]
			statsLen = availWidth
			coloredStats.Reset()
			coloredStats.WriteString(plainStats)
		}

		padding := availWidth - statsLen
		leftPad := padding / 2
		rightPad := padding - leftPad

		sb.WriteString(strings.Repeat(" ", leftPad))
		sb.WriteString(coloredStats.String())
		sb.WriteString(strings.Repeat(" ", rightPad))

		pos = cell.X0 + 1 + availWidth

		if i < len(leaves)-1 {
			sb.WriteString(r.style.Vertical)
			pos++
		}
	}

	for pos < r.Width-1 {
		sb.WriteString(" ")
		pos++
	}

	sb.WriteString(r.style.Vertical)
	fmt.Fprintln(r.w, sb.String())
}

// renderLeafBorder renders the bottom border aligned to leaf cells.
func (r *IcicleRenderer) renderLeafBorder(leaves []LayoutCell) {
	boundaries := GetLeafBoundaries(leaves, r.Width-2)

	var sb strings.Builder
	sb.Grow(r.Width + 10)
	sb.WriteString(r.style.BottomLeft)

	for pos := 1; pos < r.Width-1; pos++ {
		if boundaries[pos] {
			sb.WriteString(r.style.BottomSep)
		} else {
			sb.WriteString(r.style.Horizontal)
		}
	}

	sb.WriteString(r.style.BottomRight)
	fmt.Fprintln(r.w, sb.String())
}

// formatCentered returns the label centered within width, with ANSI color codes.
func (r *IcicleRenderer) formatCentered(cell LayoutCell, width, reserveRight int) (content string, visualWidth int) {
	label := r.truncate(cell.Label, width-reserveRight)
	labelLen := utf8.RuneCountInString(label)

	padding := width - labelLen - reserveRight
	if padding < 0 {
		padding = 0
	}
	leftPad := padding / 2
	rightPad := padding - leftPad

	var sb strings.Builder
	sb.WriteString(strings.Repeat(" ", leftPad))
	sb.WriteString(r.color(cell.Color()))
	sb.WriteString(label)
	sb.WriteString(r.color(ColorReset))
	sb.WriteString(strings.Repeat(" ", rightPad))

	return sb.String(), leftPad + labelLen + rightPad
}

// truncate shortens a string to fit within maxLen runes.
// Preserves file extensions when possible.
func (r *IcicleRenderer) truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxLen {
		return s
	}

	// Handle directories (trailing "/")
	isDir := len(s) > 0 && s[len(s)-1] == '/'
	if isDir {
		s = s[:len(s)-1]
		maxLen--
		runeCount--
	}

	var result string
	if maxLen <= 2 {
		result = string([]rune(s)[:min(runeCount, maxLen)])
	} else {
		lastDot := strings.LastIndex(s, ".")
		if lastDot > 0 {
			ext := s[lastDot:]
			extLen := utf8.RuneCountInString(ext)

			if maxLen >= 2+1+extLen {
				nameLen := maxLen - 1 - extLen
				result = string([]rune(s[:lastDot])[:nameLen]) + "…" + ext
			} else {
				result = string([]rune(s)[:maxLen-1]) + "…"
			}
		} else {
			result = string([]rune(s)[:maxLen-1]) + "…"
		}
	}

	if isDir {
		result += "/"
	}
	return result
}

// color returns the ANSI code if color is enabled.
func (r *IcicleRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}

// CollectLeafCellsSorted is a helper that returns leaves sorted by position.
func CollectLeafCellsSorted(layout *Layout) []LayoutCell {
	leaves := CollectLeafCells(layout)
	sort.Slice(leaves, func(i, j int) bool {
		return leaves[i].X0 < leaves[j].X0
	})
	return leaves
}
