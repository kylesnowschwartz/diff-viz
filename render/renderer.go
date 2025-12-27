package render

import "github.com/kylesnowschwartz/diff-viz/diff"

// Renderer defines the interface for diff visualization renderers.
type Renderer interface {
	Render(stats *diff.DiffStats)
}
