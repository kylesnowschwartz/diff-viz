package render

import (
	"io"
	"os"
	"os/exec"

	"github.com/kylesnowschwartz/diff-viz/v2/diff"
)

// StatRenderer outputs native git diff --stat, unchanged.
type StatRenderer struct {
	args []string
	w    io.Writer
}

// NewStatRenderer creates a renderer that outputs git diff --stat directly.
// The args are passed to git diff (e.g., "HEAD", "--cached", "main..feature").
func NewStatRenderer(w io.Writer, args []string) *StatRenderer {
	return &StatRenderer{
		args: args,
		w:    w,
	}
}

// Render runs git diff --stat and writes the output directly.
// The stats parameter is ignored - we bypass parsed data entirely.
func (r *StatRenderer) Render(_ *diff.DiffStats) {
	cmdArgs := append([]string{"diff", "--stat"}, r.args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Stdout = r.w
	cmd.Stderr = os.Stderr
	cmd.Run()
}
