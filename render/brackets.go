// Package render provides diff visualization renderers.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

// BracketsRenderer renders diff stats as nested brackets showing hierarchy.
// Format: [src/ [lib/ parser +45, lexer +23] main +8] | [tests/ parser_test +89]
//
// The hierarchy is encoded via bracket nesting, magnitude via numbers.
// Single-child paths are collapsed: [cmd/git-diff-tree/ main.go +63]
// Groups separated by │, items separated by commas.
//
// ExpandDepth controls multi-line expansion:
//
//	-1 = auto (expand if inline exceeds Width)
//	 0 = inline only (word-wrap at Width)
//	 1 = top-level dirs on separate lines
//	 2 = expand to depth 2 with indentation, etc.
type BracketsRenderer struct {
	UseColor    bool
	ShowCounts  bool   // Show +N-M instead of bars
	MaxBarLen   int    // Max bar characters per file (default 4)
	Width       int    // Max line width before wrapping (default 100)
	Separator   string // Separator between top-level groups (default " │ ")
	ExpandDepth int    // Expansion depth: -1=auto, 0=inline, 1+=expand to depth
	w           io.Writer
}

// NewBracketsRenderer creates a brackets renderer.
func NewBracketsRenderer(w io.Writer, useColor bool) *BracketsRenderer {
	return &BracketsRenderer{
		UseColor:    useColor,
		ShowCounts:  true, // +N-M is more readable than bars in dense output
		MaxBarLen:   4,
		Width:       100,
		Separator:   " │ ",
		ExpandDepth: -1, // auto by default
		w:           w,
	}
}

// Render outputs diff stats as nested bracket notation.
func (r *BracketsRenderer) Render(stats *diff.DiffStats) {
	if stats.TotalFiles == 0 {
		fmt.Fprintln(r.w, "No changes")
		return
	}

	// Build tree from files
	tree := buildBracketTree(stats.Files)

	// Collapse single-child directory chains for cleaner output
	collapseSingleChildPaths(tree)

	// Find max value for scaling bars
	maxVal := r.findMaxValue(tree)

	// Separate directories from root files
	var dirNodes []*bracketNode
	var rootFiles []*bracketNode
	for _, node := range tree {
		if node.IsDir {
			dirNodes = append(dirNodes, node)
		} else {
			rootFiles = append(rootFiles, node)
		}
	}

	// Handle explicit expand depth (not auto)
	if r.ExpandDepth >= 0 {
		if r.ExpandDepth > 0 {
			r.renderExpanded(dirNodes, rootFiles, maxVal, r.ExpandDepth)
		} else {
			r.renderInline(dirNodes, rootFiles, maxVal)
		}
		return
	}

	// Auto mode: smart per-group width evaluation
	r.renderSmart(dirNodes, rootFiles, maxVal)
}

// renderSmart uses per-group width evaluation.
// Groups that fit together share a line; wide groups get their own line and may expand.
func (r *BracketsRenderer) renderSmart(dirs []*bracketNode, rootFiles []*bracketNode, maxVal int) {
	// Build list of renderable groups with their inline representations
	type group struct {
		node        *bracketNode // nil for root files group
		inline      string       // inline rendered string
		width       int          // visible width
		needsExpand bool         // true if too wide even alone
	}

	var groups []group
	for _, node := range dirs {
		inline := r.renderNode(node, maxVal, 0, "")
		w := visibleWidth(inline)
		groups = append(groups, group{
			node:        node,
			inline:      inline,
			width:       w,
			needsExpand: w > r.Width,
		})
	}

	// Add root files as a group if present
	if len(rootFiles) > 0 {
		var rootPart strings.Builder
		rootPart.WriteString(r.color(ColorFile))
		rootPart.WriteString("root:")
		rootPart.WriteString(r.color(ColorReset))
		for i, f := range rootFiles {
			rootPart.WriteString(" ")
			rootPart.WriteString(r.renderNode(f, maxVal, 0, ""))
			if i < len(rootFiles)-1 {
				rootPart.WriteString(",")
			}
		}
		inline := rootPart.String()
		groups = append(groups, group{
			node:        nil,
			inline:      inline,
			width:       visibleWidth(inline),
			needsExpand: false, // root files don't expand
		})
	}

	// Render groups with smart line packing
	sepWidth := visibleWidth(r.Separator)
	var currentLine strings.Builder
	currentWidth := 0

	for i, g := range groups {
		if g.needsExpand {
			// Flush current line if any
			if currentWidth > 0 {
				fmt.Fprintln(r.w, currentLine.String())
				currentLine.Reset()
				currentWidth = 0
			}
			// Render this group expanded
			fmt.Fprint(r.w, r.renderNodeExpanded(g.node, maxVal, 0, "", 1))
		} else if currentWidth == 0 {
			// First group on line
			currentLine.WriteString(g.inline)
			currentWidth = g.width
		} else if currentWidth+sepWidth+g.width <= r.Width {
			// Fits on current line
			currentLine.WriteString(r.Separator)
			currentLine.WriteString(g.inline)
			currentWidth += sepWidth + g.width
		} else {
			// Doesn't fit - start new line
			fmt.Fprintln(r.w, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(g.inline)
			currentWidth = g.width
		}

		// Flush on last group
		if i == len(groups)-1 && currentWidth > 0 {
			fmt.Fprintln(r.w, currentLine.String())
		}
	}
}

// calcInlineWidth estimates total width if rendered inline.
func (r *BracketsRenderer) calcInlineWidth(dirs []*bracketNode, rootFiles []*bracketNode, maxVal int) int {
	var parts []string
	for _, node := range dirs {
		parts = append(parts, r.renderNode(node, maxVal, 0, ""))
	}
	if len(rootFiles) > 0 {
		var rootPart strings.Builder
		rootPart.WriteString("root:")
		for i, f := range rootFiles {
			rootPart.WriteString(" ")
			rootPart.WriteString(r.renderNode(f, maxVal, 0, ""))
			if i < len(rootFiles)-1 {
				rootPart.WriteString(",")
			}
		}
		parts = append(parts, rootPart.String())
	}

	total := 0
	sepWidth := visibleWidth(r.Separator)
	for i, p := range parts {
		total += visibleWidth(p)
		if i < len(parts)-1 {
			total += sepWidth
		}
	}
	return total
}

// renderInline renders using word-wrap at Width (original behavior).
func (r *BracketsRenderer) renderInline(dirs []*bracketNode, rootFiles []*bracketNode, maxVal int) {
	var parts []string
	for _, node := range dirs {
		parts = append(parts, r.renderNode(node, maxVal, 0, ""))
	}

	if len(rootFiles) > 0 {
		var rootPart strings.Builder
		rootPart.WriteString(r.color(ColorFile))
		rootPart.WriteString("root:")
		rootPart.WriteString(r.color(ColorReset))
		for i, f := range rootFiles {
			rootPart.WriteString(" ")
			rootPart.WriteString(r.renderNode(f, maxVal, 0, ""))
			if i < len(rootFiles)-1 {
				rootPart.WriteString(",")
			}
		}
		parts = append(parts, rootPart.String())
	}

	fmt.Fprintln(r.w, r.wrapJoin(parts))
}

// renderExpanded renders with multi-line expansion at specified depth.
func (r *BracketsRenderer) renderExpanded(dirs []*bracketNode, rootFiles []*bracketNode, maxVal int, expandDepth int) {
	for _, node := range dirs {
		fmt.Fprint(r.w, r.renderNodeExpanded(node, maxVal, 0, "", expandDepth))
	}

	if len(rootFiles) > 0 {
		var rootPart strings.Builder
		rootPart.WriteString(r.color(ColorFile))
		rootPart.WriteString("root:")
		rootPart.WriteString(r.color(ColorReset))
		for i, f := range rootFiles {
			rootPart.WriteString(" ")
			rootPart.WriteString(r.renderNode(f, maxVal, 0, ""))
			if i < len(rootFiles)-1 {
				rootPart.WriteString(",")
			}
		}
		fmt.Fprintln(r.w, rootPart.String())
	}
}

// renderNodeExpanded renders a node with depth-based line expansion.
// When depth < expandDepth, children go on separate indented lines.
func (r *BracketsRenderer) renderNodeExpanded(node *bracketNode, maxVal int, depth int, indent string, expandDepth int) string {
	var sb strings.Builder

	if !node.IsDir {
		// Files are always inline
		return r.renderNode(node, maxVal, depth, indent)
	}

	// Directory rendering
	bracketColor := bracketColors[depth%len(bracketColors)]

	// Write the directory name (no bracket at depth 0)
	if depth > 0 {
		sb.WriteString(r.color(bracketColor))
		sb.WriteString("[")
		sb.WriteString(r.color(ColorReset))
	}
	sb.WriteString(r.color(ColorDir))
	name := node.Name
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}
	sb.WriteString(name)
	sb.WriteString(r.color(ColorReset))

	// Decide: expand children to new lines or keep inline?
	if depth < expandDepth && len(node.Children) > 0 {
		// Expanded: each child on its own indented line
		childIndent := indent + "  "
		for _, child := range node.Children {
			sb.WriteString("\n")
			sb.WriteString(childIndent)
			sb.WriteString(r.renderNodeExpanded(child, maxVal, depth+1, childIndent, expandDepth))
		}
		if depth > 0 {
			sb.WriteString(r.color(bracketColor))
			sb.WriteString("]")
			sb.WriteString(r.color(ColorReset))
		}
		sb.WriteString("\n")
	} else {
		// Inline: render children on same line
		for i, child := range node.Children {
			sb.WriteString(" ")
			sb.WriteString(r.renderNode(child, maxVal, depth+1, indent))
			if i < len(node.Children)-1 {
				sb.WriteString(",")
			}
		}
		if depth > 0 {
			sb.WriteString(r.color(bracketColor))
			sb.WriteString("]")
			sb.WriteString(r.color(ColorReset))
		}
		if depth == 0 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// wrapJoin joins parts with separator and word-wrap semantics.
// Each part stays intact; wraps to new line when width exceeded.
func (r *BracketsRenderer) wrapJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	sep := r.Separator
	sepWidth := visibleWidth(sep)

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for i, part := range parts {
		partWidth := visibleWidth(part)

		// First part on line, or fits on current line
		if currentWidth == 0 {
			currentLine.WriteString(part)
			currentWidth = partWidth
		} else if currentWidth+sepWidth+partWidth <= r.Width {
			// Fits with separator
			currentLine.WriteString(sep)
			currentLine.WriteString(part)
			currentWidth += sepWidth + partWidth
		} else {
			// Wrap to new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(part)
			currentWidth = partWidth
		}

		// Handle last part
		if i == len(parts)-1 && currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return strings.Join(lines, "\n")
}

// visibleWidth calculates display width excluding ANSI escape sequences.
func visibleWidth(s string) int {
	// Strip ANSI escape sequences: \033[...m
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}

// bracketNode represents a node in the bracket tree.
type bracketNode struct {
	Name     string
	Add      int
	Del      int
	IsDir    bool
	HasNew   bool
	Children []*bracketNode
}

func (n *bracketNode) Total() int {
	return n.Add + n.Del
}

// buildBracketTree constructs a tree from file stats.
// Groups files by path segments, aggregating stats at each level.
func buildBracketTree(files []diff.FileStat) []*bracketNode {
	root := &bracketNode{IsDir: true}

	for _, f := range files {
		parts := strings.Split(f.Path, "/")
		node := root

		for i, part := range parts {
			isLast := i == len(parts)-1

			// Find or create child
			var child *bracketNode
			for _, c := range node.Children {
				if c.Name == part {
					child = c
					break
				}
			}
			if child == nil {
				child = &bracketNode{
					Name:  part,
					IsDir: !isLast,
				}
				node.Children = append(node.Children, child)
			}

			// Accumulate stats at this level
			child.Add += f.Additions
			child.Del += f.Deletions
			if f.IsUntracked {
				child.HasNew = true
			}

			node = child
		}
	}

	// Sort children by total at each level (descending)
	sortBracketTree(root)

	return root.Children
}

// sortBracketTree recursively sorts children by total changes.
func sortBracketTree(node *bracketNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Total() > node.Children[j].Total()
	})
	for _, child := range node.Children {
		if child.IsDir {
			sortBracketTree(child)
		}
	}
}

// collapseSingleChildPaths merges directory chains with single children.
// Example: [cmd [git-diff-tree main.go]] -> [cmd/git-diff-tree/ main.go]
func collapseSingleChildPaths(nodes []*bracketNode) {
	for i, node := range nodes {
		if !node.IsDir {
			continue
		}
		// Keep collapsing while we have a single directory child
		for len(node.Children) == 1 && node.Children[0].IsDir {
			child := node.Children[0]
			node.Name = node.Name + "/" + child.Name
			node.Children = child.Children
		}
		nodes[i] = node
		// Recurse into remaining children
		collapseSingleChildPaths(node.Children)
	}
}

// findMaxValue finds the maximum total across all leaf nodes.
func (r *BracketsRenderer) findMaxValue(nodes []*bracketNode) int {
	max := 0
	var walk func([]*bracketNode)
	walk = func(nodes []*bracketNode) {
		for _, n := range nodes {
			if !n.IsDir {
				if n.Total() > max {
					max = n.Total()
				}
			} else {
				walk(n.Children)
			}
		}
	}
	walk(nodes)
	return max
}

// Rainbow bracket colors - cycle through these based on depth
var bracketColors = []string{
	"\033[36m", // Cyan
	"\033[33m", // Yellow
	"\033[35m", // Magenta
	"\033[32m", // Green
	"\033[34m", // Blue
}

// renderNode recursively renders a node and its children.
// indent is used for multi-line expanded output.
func (r *BracketsRenderer) renderNode(node *bracketNode, maxVal int, depth int, indent string) string {
	var sb strings.Builder

	if node.IsDir {
		// Directory: [name/ children...] with rainbow brackets
		// Skip brackets at depth 0 (top-level) to reduce visual noise
		bracketColor := bracketColors[depth%len(bracketColors)]
		if depth > 0 {
			sb.WriteString(r.color(bracketColor))
			sb.WriteString("[")
			sb.WriteString(r.color(ColorReset))
		}
		sb.WriteString(r.color(ColorDir))
		// Add trailing slash to make directories obvious
		name := node.Name
		if !strings.HasSuffix(name, "/") {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString(r.color(ColorReset))

		// Render children at next depth, separated by commas
		for i, child := range node.Children {
			sb.WriteString(" ")
			sb.WriteString(r.renderNode(child, maxVal, depth+1, indent))
			// Add comma between children (not after last)
			if i < len(node.Children)-1 {
				sb.WriteString(",")
			}
		}
		if depth > 0 {
			sb.WriteString(r.color(bracketColor))
			sb.WriteString("]")
			sb.WriteString(r.color(ColorReset))
		}
	} else {
		// File: name + bar or counts
		nameColor := ColorFile
		if node.HasNew {
			nameColor = ColorNew
		}
		sb.WriteString(r.color(nameColor))
		sb.WriteString(node.Name)
		sb.WriteString(r.color(ColorReset))

		if r.ShowCounts {
			// Show +N -M format with spacing
			if node.Add > 0 {
				sb.WriteString(" ")
				sb.WriteString(r.color(ColorAdd))
				sb.WriteString(fmt.Sprintf("+%d", node.Add))
				sb.WriteString(r.color(ColorReset))
			}
			if node.Del > 0 {
				sb.WriteString(" ")
				sb.WriteString(r.color(ColorDel))
				sb.WriteString(fmt.Sprintf("-%d", node.Del))
				sb.WriteString(r.color(ColorReset))
			}
		} else {
			// Show magnitude bar
			bar := r.makeBar(node.Total(), maxVal)
			if bar != "" {
				sb.WriteString(r.color(ColorAdd))
				sb.WriteString(bar)
				sb.WriteString(r.color(ColorReset))
			}
		}
	}

	return sb.String()
}

// makeBar creates a proportional bar based on value.
func (r *BracketsRenderer) makeBar(val, maxVal int) string {
	if maxVal == 0 || val == 0 {
		return ""
	}

	// Scale to MaxBarLen
	filled := (val * r.MaxBarLen) / maxVal
	if filled == 0 && val > 0 {
		filled = 1 // Always show at least one block for non-zero
	}

	return strings.Repeat("█", filled)
}

// color returns the ANSI code if color is enabled.
func (r *BracketsRenderer) color(code string) string {
	if r.UseColor {
		return code
	}
	return ""
}
