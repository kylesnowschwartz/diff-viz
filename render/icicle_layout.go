package render

import (
	"sort"
)

// LayoutCell holds computed pixel coordinates for a cell.
// Unlike IcicleCell, this is purely layout data - no rendering state.
type LayoutCell struct {
	Path           string // Full path for lookup
	Label          string // Display name
	Add            int    // Additions
	Del            int    // Deletions
	X0, X1         int    // Horizontal pixel bounds [X0, X1)
	IsUntracked    bool   // True if this is an untracked file
	HiddenSiblings int    // Count of sibling nodes dropped due to space constraints
}

// Width returns the cell width in pixels.
func (c LayoutCell) Width() int {
	return c.X1 - c.X0
}

// Total returns total changes (add + del).
func (c LayoutCell) Total() int {
	return c.Add + c.Del
}

// Color returns the appropriate color code based on add/del ratio.
func (c LayoutCell) Color() string {
	if c.IsUntracked {
		return ColorNew // Yellow for untracked files
	}
	switch {
	case c.Add > 0 && c.Del == 0:
		return ColorAdd
	case c.Del > 0 && c.Add == 0:
		return ColorDel
	default:
		return ColorDir
	}
}

// Layout holds the computed cell positions for an icicle chart.
// Separates layout calculation from rendering for testability.
type Layout struct {
	Cells      [][]LayoutCell // Cells by depth level
	Boundaries [][]int        // Pre-computed boundary positions per level (sorted)
	Width      int            // Total width in pixels
	Dropped    int            // Count of nodes dropped due to space constraints
}

// ComputeLayout builds the layout for all cells in an icicle chart.
// Uses D3's treemapDice algorithm with minimum-width constraint.
//
// Parameters:
//   - root: Tree root node (contains hierarchical file stats)
//   - width: Total chart width in pixels
//   - maxDepth: Maximum depth levels (0 = unlimited)
//   - minCellWidth: Minimum cell width to guarantee visibility
func ComputeLayout(root *TreeNode, width, maxDepth, minCellWidth int) *Layout {
	layout := &Layout{
		Width:      width,
		Cells:      make([][]LayoutCell, 0, maxDepth),
		Boundaries: make([][]int, 0, maxDepth),
	}

	if root == nil || len(root.Children) == 0 {
		return layout
	}

	usableWidth := width - 2 // Account for left/right borders
	totalChanges := root.Add + root.Del
	if totalChanges == 0 {
		totalChanges = 1
	}

	// Level 0: root's children with proportional widths
	level0 := diceLevel(root.Children, 0, usableWidth, totalChanges, minCellWidth, &layout.Dropped)
	if len(level0) == 0 {
		return layout
	}
	layout.Cells = append(layout.Cells, level0)
	layout.Boundaries = append(layout.Boundaries, extractBoundaries(level0, usableWidth))

	// Build subsequent levels breadth-first
	for depth := 1; maxDepth == 0 || depth < maxDepth; depth++ {
		prevLevel := layout.Cells[depth-1]
		var nextLevel []LayoutCell

		for _, cell := range prevLevel {
			// Find the node for this cell
			node := FindNode(root, cell.Path)
			if node == nil || !node.IsDir || len(node.Children) == 0 {
				continue
			}

			// Build children within this cell's bounds
			children := diceLevel(node.Children, cell.X0, cell.Width(), cell.Total(), minCellWidth, &layout.Dropped)
			nextLevel = append(nextLevel, children...)
		}

		if len(nextLevel) == 0 {
			break // No more children to render
		}
		layout.Cells = append(layout.Cells, nextLevel)
		layout.Boundaries = append(layout.Boundaries, extractBoundaries(nextLevel, usableWidth))
	}

	return layout
}

// diceLevel lays out nodes horizontally with proportional widths + minimum constraint.
// This is a D3-style treemapDice with added minimum width guarantee.
//
// Parameters:
//   - nodes: Child nodes to lay out
//   - startX: Starting X position (0-indexed)
//   - availWidth: Available width in pixels
//   - totalValue: Total value for proportional calculation
//   - minWidth: Minimum cell width
//   - dropped: Counter for dropped nodes (modified in place)
func diceLevel(nodes []*TreeNode, startX, availWidth, totalValue, minWidth int, dropped *int) []LayoutCell {
	if len(nodes) == 0 || availWidth < 1 {
		return nil
	}

	// Filter nodes with changes and sort by total descending
	sorted := filterAndSortNodes(nodes)
	n := len(sorted)
	if n == 0 {
		return nil
	}

	// Track how many siblings will be hidden for this group
	hiddenSiblings := 0

	// Calculate minimum space required
	minReserved := n * minWidth
	if minReserved > availWidth {
		// Not enough space for all nodes - take what fits
		maxNodes := availWidth / minWidth
		if maxNodes == 0 {
			*dropped += n
			return nil
		}
		hiddenSiblings = n - maxNodes
		*dropped += hiddenSiblings
		sorted = sorted[:maxNodes]
		n = maxNodes
		minReserved = n * minWidth
	}

	// Calculate widths FIRST (like v1), then create cells
	extraWidth := availWidth - minReserved
	widths := make([]int, n)
	for i, node := range sorted {
		nodeTotal := node.Add + node.Del
		extra := 0
		if extraWidth > 0 && totalValue > 0 {
			extra = (nodeTotal * extraWidth) / totalValue
		}
		widths[i] = minWidth + extra
	}

	// Fill gap - give extra to first/largest cell (like v1)
	usedWidth := 0
	for _, w := range widths {
		usedWidth += w
	}
	if usedWidth < availWidth && len(widths) > 0 {
		widths[0] += availWidth - usedWidth
	}

	// Now create cells with correct widths
	cells := make([]LayoutCell, 0, n)
	x := startX

	for i, node := range sorted {
		label := node.Name
		if node.IsDir {
			label += "/"
		}

		cells = append(cells, LayoutCell{
			Path:           node.Path,
			Label:          label,
			Add:            node.Add,
			Del:            node.Del,
			X0:             x,
			X1:             x + widths[i],
			IsUntracked:    node.IsUntracked,
			HiddenSiblings: hiddenSiblings,
		})
		x += widths[i]
	}

	return cells
}

// filterAndSortNodes returns nodes with changes, sorted by magnitude descending.
func filterAndSortNodes(nodes []*TreeNode) []*TreeNode {
	sorted := make([]*TreeNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Add+n.Del > 0 {
			sorted = append(sorted, n)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Add+sorted[i].Del > sorted[j].Add+sorted[j].Del
	})
	return sorted
}

// extractBoundaries returns sorted boundary positions for a level.
// Called once per level during layout, not during rendering.
func extractBoundaries(cells []LayoutCell, usableWidth int) []int {
	bounds := make([]int, 0, len(cells))
	for _, cell := range cells {
		// Don't mark the right edge as internal boundary
		if cell.X1 < usableWidth {
			bounds = append(bounds, cell.X1)
		}
	}
	sort.Ints(bounds)
	return bounds
}

// HasBoundary checks if position is a boundary using binary search.
// O(log n) vs O(1) for map, but avoids per-call allocations.
func HasBoundary(boundaries []int, pos int) bool {
	i := sort.SearchInts(boundaries, pos)
	return i < len(boundaries) && boundaries[i] == pos
}

// GetBoundaries returns a map of boundary positions for compatibility.
// Prefer HasBoundary for new code to avoid allocations.
func GetBoundaries(cells []LayoutCell, usableWidth int) map[int]bool {
	boundaries := make(map[int]bool)
	for _, cell := range cells {
		if cell.X1 < usableWidth {
			boundaries[cell.X1] = true
		}
	}
	return boundaries
}

// CollectLeafCells returns all leaf cells (cells with no children in next level).
func CollectLeafCells(layout *Layout) []LayoutCell {
	var leaves []LayoutCell

	for depth := 0; depth < len(layout.Cells); depth++ {
		for _, cell := range layout.Cells[depth] {
			isLeaf := true

			// Check if any cell in the next level falls within this cell's bounds
			if depth+1 < len(layout.Cells) {
				for _, child := range layout.Cells[depth+1] {
					if child.X0 >= cell.X0 && child.X0 < cell.X1 {
						isLeaf = false
						break
					}
				}
			}

			if isLeaf {
				leaves = append(leaves, cell)
			}
		}
	}

	// Sort by Start position for proper rendering order
	sort.Slice(leaves, func(i, j int) bool {
		return leaves[i].X0 < leaves[j].X0
	})

	return leaves
}

// GetLeafBoundaries returns boundary positions for leaf cells.
func GetLeafBoundaries(leaves []LayoutCell, usableWidth int) map[int]bool {
	boundaries := make(map[int]bool)
	for _, cell := range leaves {
		if cell.X1 < usableWidth {
			boundaries[cell.X1] = true
		}
	}
	return boundaries
}
