package render

import (
	"testing"
)

func TestDiceLevel_Basic(t *testing.T) {
	// Create test nodes
	nodes := []*TreeNode{
		{Name: "big", Path: "big", Add: 80, Del: 0},
		{Name: "small", Path: "small", Add: 20, Del: 0},
	}

	var dropped int
	cells := diceLevel(nodes, 0, 100, 100, 10, &dropped)

	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(cells))
	}

	// First cell (big) should be larger
	if cells[0].Label != "big" {
		t.Errorf("expected first cell to be 'big', got %q", cells[0].Label)
	}

	// Check proportional widths (min 10 each, 80 extra distributed)
	// big: 10 + (80 * 80 / 100) = 10 + 64 = 74
	// small: 10 + (20 * 80 / 100) = 10 + 16 = 26
	// Total: 100

	bigWidth := cells[0].Width()
	smallWidth := cells[1].Width()

	if bigWidth+smallWidth != 100 {
		t.Errorf("expected total width 100, got %d", bigWidth+smallWidth)
	}

	if bigWidth <= smallWidth {
		t.Errorf("expected big (%d) > small (%d)", bigWidth, smallWidth)
	}
}

func TestDiceLevel_MinimumWidth(t *testing.T) {
	// Many small nodes - some should be dropped
	nodes := make([]*TreeNode, 20)
	for i := 0; i < 20; i++ {
		nodes[i] = &TreeNode{Name: "f", Path: "f", Add: 1, Del: 0}
	}

	var dropped int
	cells := diceLevel(nodes, 0, 50, 20, 10, &dropped)

	// With width 50 and min 10, only 5 cells fit
	if len(cells) != 5 {
		t.Errorf("expected 5 cells with width 50, min 10, got %d", len(cells))
	}

	if dropped != 15 {
		t.Errorf("expected 15 dropped, got %d", dropped)
	}
}

func TestDiceLevel_NoGaps(t *testing.T) {
	nodes := []*TreeNode{
		{Name: "a", Path: "a", Add: 33, Del: 0},
		{Name: "b", Path: "b", Add: 33, Del: 0},
		{Name: "c", Path: "c", Add: 34, Del: 0},
	}

	var dropped int
	cells := diceLevel(nodes, 0, 100, 100, 10, &dropped)

	// No gaps - total width should equal available width
	// (first cell gets the gap fill via the algorithm)
	totalWidth := 0
	for _, c := range cells {
		totalWidth += c.Width()
	}

	if totalWidth != 100 {
		t.Errorf("expected total width 100 (no gaps), got %d", totalWidth)
	}
}

func TestDiceLevel_DirectoryLabel(t *testing.T) {
	nodes := []*TreeNode{
		{Name: "src", Path: "src", IsDir: true, Add: 50, Del: 0},
		{Name: "main.go", Path: "main.go", IsDir: false, Add: 50, Del: 0},
	}

	var dropped int
	cells := diceLevel(nodes, 0, 100, 100, 10, &dropped)

	// Find the directory cell
	var dirCell, fileCell *LayoutCell
	for i := range cells {
		if cells[i].Path == "src" {
			dirCell = &cells[i]
		}
		if cells[i].Path == "main.go" {
			fileCell = &cells[i]
		}
	}

	if dirCell == nil || fileCell == nil {
		t.Fatal("missing expected cells")
	}

	if dirCell.Label != "src/" {
		t.Errorf("expected dir label 'src/', got %q", dirCell.Label)
	}

	if fileCell.Label != "main.go" {
		t.Errorf("expected file label 'main.go', got %q", fileCell.Label)
	}
}

func TestDiceLevel_ZeroChanges(t *testing.T) {
	nodes := []*TreeNode{
		{Name: "empty", Path: "empty", Add: 0, Del: 0},
	}

	var dropped int
	cells := diceLevel(nodes, 0, 100, 100, 10, &dropped)

	// Nodes with zero changes should be filtered out
	if len(cells) != 0 {
		t.Errorf("expected 0 cells for zero-change nodes, got %d", len(cells))
	}
}

func TestHasBoundary(t *testing.T) {
	boundaries := []int{10, 20, 30, 50}

	tests := []struct {
		pos  int
		want bool
	}{
		{10, true},
		{20, true},
		{30, true},
		{50, true},
		{0, false},
		{15, false},
		{25, false},
		{100, false},
	}

	for _, tt := range tests {
		got := HasBoundary(boundaries, tt.pos)
		if got != tt.want {
			t.Errorf("HasBoundary(%v, %d) = %v, want %v", boundaries, tt.pos, got, tt.want)
		}
	}
}

func TestHasBoundary_Empty(t *testing.T) {
	var boundaries []int

	if HasBoundary(boundaries, 10) {
		t.Error("expected false for empty boundaries")
	}
}

func TestExtractBoundaries(t *testing.T) {
	cells := []LayoutCell{
		{X0: 0, X1: 30},
		{X0: 30, X1: 60},
		{X0: 60, X1: 98}, // X1 < usableWidth, should be boundary
	}

	boundaries := extractBoundaries(cells, 98)

	// Should have boundaries at 30 and 60 (not 98, which is the edge)
	if len(boundaries) != 2 {
		t.Errorf("expected 2 boundaries, got %d: %v", len(boundaries), boundaries)
	}

	if !HasBoundary(boundaries, 30) {
		t.Error("expected boundary at 30")
	}

	if !HasBoundary(boundaries, 60) {
		t.Error("expected boundary at 60")
	}
}

func TestComputeLayout_Basic(t *testing.T) {
	// Build a simple tree: root -> [a/, b.go]
	root := &TreeNode{
		Name:  "",
		Path:  "",
		IsDir: true,
		Add:   100,
		Del:   0,
		Children: []*TreeNode{
			{Name: "a", Path: "a", IsDir: true, Add: 60, Del: 0},
			{Name: "b.go", Path: "b.go", IsDir: false, Add: 40, Del: 0},
		},
	}

	layout := ComputeLayout(root, 102, 4, 10) // 102 - 2 borders = 100 usable

	if len(layout.Cells) != 1 {
		t.Fatalf("expected 1 level (no children in children), got %d", len(layout.Cells))
	}

	if len(layout.Cells[0]) != 2 {
		t.Fatalf("expected 2 cells at level 0, got %d", len(layout.Cells[0]))
	}

	// Check boundaries were computed
	if len(layout.Boundaries) != 1 {
		t.Errorf("expected 1 boundary level, got %d", len(layout.Boundaries))
	}
}

func TestComputeLayout_MultiLevel(t *testing.T) {
	// Build a tree: root -> [a/ -> [x.go, y.go]]
	root := &TreeNode{
		Name:  "",
		Path:  "",
		IsDir: true,
		Add:   100,
		Del:   0,
		Children: []*TreeNode{
			{
				Name:  "a",
				Path:  "a",
				IsDir: true,
				Add:   100,
				Del:   0,
				Children: []*TreeNode{
					{Name: "x.go", Path: "a/x.go", IsDir: false, Add: 60, Del: 0},
					{Name: "y.go", Path: "a/y.go", IsDir: false, Add: 40, Del: 0},
				},
			},
		},
	}

	layout := ComputeLayout(root, 102, 4, 10)

	if len(layout.Cells) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(layout.Cells))
	}

	// Level 0: a/
	// Level 1: x.go, y.go
	if len(layout.Cells[0]) != 1 {
		t.Errorf("expected 1 cell at level 0, got %d", len(layout.Cells[0]))
	}

	if len(layout.Cells[1]) != 2 {
		t.Errorf("expected 2 cells at level 1, got %d", len(layout.Cells[1]))
	}
}

func TestComputeLayout_Empty(t *testing.T) {
	root := &TreeNode{
		Name:     "",
		Path:     "",
		IsDir:    true,
		Children: nil,
	}

	layout := ComputeLayout(root, 100, 4, 10)

	if len(layout.Cells) != 0 {
		t.Errorf("expected 0 levels for empty tree, got %d", len(layout.Cells))
	}
}

func TestComputeLayout_Nil(t *testing.T) {
	layout := ComputeLayout(nil, 100, 4, 10)

	if layout == nil {
		t.Fatal("expected non-nil layout for nil root")
	}

	if len(layout.Cells) != 0 {
		t.Errorf("expected 0 levels, got %d", len(layout.Cells))
	}
}

func TestCollectLeafCells(t *testing.T) {
	layout := &Layout{
		Cells: [][]LayoutCell{
			// Level 0: parent cell
			{{Path: "a", X0: 0, X1: 100}},
			// Level 1: children (these are the leaves)
			{{Path: "a/x.go", X0: 0, X1: 50}, {Path: "a/y.go", X0: 50, X1: 100}},
		},
	}

	leaves := CollectLeafCells(layout)

	// a/ has children at level 1, so it's not a leaf
	// a/x.go and a/y.go have no children at level 2, so they are leaves
	if len(leaves) != 2 {
		t.Errorf("expected 2 leaf cells, got %d", len(leaves))
	}

	// Should be sorted by X0
	if leaves[0].Path != "a/x.go" {
		t.Errorf("expected first leaf 'a/x.go', got %q", leaves[0].Path)
	}
}

func TestLayoutCell_Width(t *testing.T) {
	cell := LayoutCell{X0: 10, X1: 50}
	if cell.Width() != 40 {
		t.Errorf("expected width 40, got %d", cell.Width())
	}
}

func TestLayoutCell_Total(t *testing.T) {
	cell := LayoutCell{Add: 30, Del: 20}
	if cell.Total() != 50 {
		t.Errorf("expected total 50, got %d", cell.Total())
	}
}

// Benchmark: HasBoundary vs map lookup
func BenchmarkHasBoundary(b *testing.B) {
	boundaries := make([]int, 100)
	for i := range boundaries {
		boundaries[i] = i * 10
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HasBoundary(boundaries, 500)
	}
}

func BenchmarkMapBoundary(b *testing.B) {
	boundaries := make(map[int]bool, 100)
	for i := 0; i < 100; i++ {
		boundaries[i*10] = true
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = boundaries[500]
	}
}
