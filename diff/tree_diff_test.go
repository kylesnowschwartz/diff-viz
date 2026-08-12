package diff

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// gitIn runs a git command in dir and fails the test on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestGetTreeDiffStats_RenameDetection verifies that a moved file reports
// only its edited lines (-M), not a full delete plus re-add.
func TestGetTreeDiffStats_RenameDetection(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", "main")

	var lines strings.Builder
	for i := 0; i < 100; i++ {
		lines.WriteString("line\n")
	}
	os.WriteFile(dir+"/big.txt", []byte(lines.String()), 0644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-m", "one")
	baseTree := gitIn(t, dir, "rev-parse", "HEAD^{tree}")

	// Rename with a one-line edit
	os.MkdirAll(dir+"/moved", 0755)
	gitIn(t, dir, "mv", "big.txt", "moved/big.txt")
	f, _ := os.OpenFile(dir+"/moved/big.txt", os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("extra\n")
	f.Close()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-m", "two")
	currentTree := gitIn(t, dir, "rev-parse", "HEAD^{tree}")

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	stats, warnings, err := GetTreeDiffStats(baseTree, currentTree)
	if err != nil {
		t.Fatalf("GetTreeDiffStats: %v (warnings: %v)", err, warnings)
	}

	if stats.TotalFiles != 1 {
		t.Fatalf("TotalFiles = %d, want 1 (rename detected as one file); files: %+v", stats.TotalFiles, stats.Files)
	}
	file := stats.Files[0]
	if file.Path != "moved/big.txt" {
		t.Errorf("Path = %q, want moved/big.txt (rename destination)", file.Path)
	}
	if file.Additions != 1 {
		t.Errorf("Additions = %d, want 1 (only the edited line, not the whole moved file)", file.Additions)
	}
	if file.IsUntracked {
		t.Errorf("IsUntracked = true, want false (a rename is not a new file)")
	}
}
