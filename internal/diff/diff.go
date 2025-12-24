// Package diff parses git diff output into structured data.
package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// FileStat represents changes to a single file.
type FileStat struct {
	Path        string
	Additions   int
	Deletions   int
	IsBinary    bool
	IsUntracked bool
}

// FileStatJSON is the JSON-serializable representation of a file's stats.
type FileStatJSON struct {
	Path   string `json:"path"`
	Adds   int    `json:"adds"`
	Dels   int    `json:"dels"`
	Binary bool   `json:"binary,omitempty"`
	New    bool   `json:"new,omitempty"`
}

// TotalsJSON is the JSON-serializable representation of total stats.
type TotalsJSON struct {
	Adds      int `json:"adds"`
	Dels      int `json:"dels"`
	FileCount int `json:"fileCount"`
}

// StatsJSON is the JSON-serializable representation of diff stats.
// This is the output format for --stats-json flag.
type StatsJSON struct {
	Files  []FileStatJSON `json:"files"`
	Totals TotalsJSON     `json:"totals"`
}

// ToJSON converts DiffStats to JSON-serializable format.
func (s *DiffStats) ToJSON() StatsJSON {
	files := make([]FileStatJSON, len(s.Files))
	for i, f := range s.Files {
		files[i] = FileStatJSON{
			Path:   f.Path,
			Adds:   f.Additions,
			Dels:   f.Deletions,
			Binary: f.IsBinary,
			New:    f.IsUntracked,
		}
	}
	return StatsJSON{
		Files: files,
		Totals: TotalsJSON{
			Adds:      s.TotalAdd,
			Dels:      s.TotalDel,
			FileCount: s.TotalFiles,
		},
	}
}

// DiffStats holds all file changes from a git diff.
type DiffStats struct {
	Files      []FileStat
	TotalAdd   int
	TotalDel   int
	TotalFiles int
}

// GetDiffStats runs git diff --numstat and parses the output.
// args are passed directly to git diff (e.g., "HEAD", "--cached", "main..feature").
// Returns warnings for non-fatal issues (git errors that might indicate problems).
func GetDiffStats(args ...string) (*DiffStats, []string, error) {
	var warnings []string
	cmdArgs := append([]string{"diff", "--numstat"}, args...)
	cmd := exec.Command("git", cmdArgs...)

	output, err := cmd.Output()
	if err != nil {
		// Check if it's an ExitError with stderr info
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				warnings = append(warnings, fmt.Sprintf("git diff: %s", stderr))
			} else {
				warnings = append(warnings, fmt.Sprintf("git diff exited with code %d", exitErr.ExitCode()))
			}
		}
		// Fail-open: return empty stats with warning
		return &DiffStats{}, warnings, nil
	}

	stats, parseWarnings, err := ParseNumstat(string(output))
	warnings = append(warnings, parseWarnings...)
	return stats, warnings, err
}

// ParseNumstat parses git diff --numstat output.
// Format: "additions\tdeletions\tpath" or "-\t-\tpath" for binary files.
// Returns warnings for malformed lines (fail-open: skips bad lines, continues parsing).
func ParseNumstat(output string) (*DiffStats, []string, error) {
	stats := &DiffStats{}
	var warnings []string
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			warnings = append(warnings, fmt.Sprintf("malformed numstat line (expected 3 fields): %q", line))
			continue
		}

		file := FileStat{Path: parts[2]}

		if parts[0] == "-" {
			// Binary file
			file.IsBinary = true
		} else {
			var err error
			file.Additions, err = strconv.Atoi(parts[0])
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid additions count %q for %s: %v", parts[0], parts[2], err))
			}
			file.Deletions, err = strconv.Atoi(parts[1])
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid deletions count %q for %s: %v", parts[1], parts[2], err))
			}
		}

		stats.Files = append(stats.Files, file)
		stats.TotalAdd += file.Additions
		stats.TotalDel += file.Deletions
	}

	stats.TotalFiles = len(stats.Files)
	return stats, warnings, scanner.Err()
}

// GetUntrackedFiles returns stats for untracked files (additions only).
// Returns warnings for git errors and file read failures.
func GetUntrackedFiles() ([]FileStat, []string, error) {
	var warnings []string
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				warnings = append(warnings, fmt.Sprintf("git ls-files: %s", stderr))
			} else {
				warnings = append(warnings, fmt.Sprintf("git ls-files exited with code %d", exitErr.ExitCode()))
			}
		}
		// Fail-open: return empty with warning
		return nil, warnings, nil
	}

	var files []FileStat
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}

		lines, readErr := countLines(path)
		file := FileStat{
			Path:        path,
			IsUntracked: true,
		}
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not read %s: %v", path, readErr))
			// Fail-open: include file but with zero additions
		}
		if lines == -1 {
			file.IsBinary = true
		} else {
			file.Additions = lines
		}
		files = append(files, file)
	}

	return files, warnings, scanner.Err()
}

// countLines counts lines in a file (for untracked files).
// Returns -1 for binary files, or an error if the file cannot be read.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	// Check for binary: look for null bytes in first 8KB
	checkLen := 8192
	if len(data) < checkLen {
		checkLen = len(data)
	}
	if bytes.Contains(data[:checkLen], []byte{0}) {
		return -1, nil // Binary file
	}
	// Count newlines, add 1 if file doesn't end with newline
	count := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}

// GetAllStats returns diff stats including untracked files.
// Aggregates warnings from all underlying operations.
func GetAllStats(args ...string) (*DiffStats, []string, error) {
	stats, warnings, err := GetDiffStats(args...)
	if err != nil {
		return nil, warnings, err
	}

	// Only include untracked for working tree diffs (no args or just "HEAD")
	includeUntracked := len(args) == 0 || (len(args) == 1 && args[0] == "HEAD")

	if includeUntracked {
		untracked, untrackedWarnings, _ := GetUntrackedFiles()
		warnings = append(warnings, untrackedWarnings...)
		for _, f := range untracked {
			stats.Files = append(stats.Files, f)
			stats.TotalAdd += f.Additions
			stats.TotalFiles++
		}
	}

	return stats, warnings, nil
}

// GetTreeDiffStats compares two git tree SHAs using git diff-tree.
// This is used for comparing against a baseline snapshot.
// Returns warnings for git command failures.
func GetTreeDiffStats(baseTree, currentTree string) (*DiffStats, []string, error) {
	var warnings []string

	// git diff-tree --numstat baseline current
	cmd := exec.Command("git", "diff-tree", "--numstat", "-r", baseTree, currentTree)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				warnings = append(warnings, fmt.Sprintf("git diff-tree: %s", stderr))
			} else {
				warnings = append(warnings, fmt.Sprintf("git diff-tree exited with code %d", exitErr.ExitCode()))
			}
		}
		// Fail-open: return empty stats with warning
		return &DiffStats{}, warnings, nil
	}

	stats, parseWarnings, err := ParseNumstat(string(output))
	warnings = append(warnings, parseWarnings...)
	if err != nil {
		return nil, warnings, err
	}

	// Get file status (A=Added, M=Modified) for weighted scoring
	statusCmd := exec.Command("git", "diff-tree", "-r", "--name-status", "--diff-filter=AM", baseTree, currentTree)
	statusOutput, statusErr := statusCmd.Output()
	if statusErr != nil {
		if exitErr, ok := statusErr.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				warnings = append(warnings, fmt.Sprintf("git diff-tree --name-status: %s", stderr))
			}
		}
		// Fail-open: skip status enrichment, continue with basic stats
	}

	// Mark files as new or modified based on status
	statusLines := make(map[string]byte)
	scanner := bufio.NewScanner(bytes.NewReader(statusOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 2 && line[1] == '\t' {
			status := line[0]
			path := line[2:]
			statusLines[path] = status
		}
	}

	// Update file stats with new/modified status
	for i := range stats.Files {
		if status, ok := statusLines[stats.Files[i].Path]; ok {
			stats.Files[i].IsUntracked = (status == 'A') // Treat "Added" as new file
		}
	}

	return stats, warnings, nil
}

// CaptureCurrentTree returns the SHA of the current working tree.
// Uses a temporary index file to avoid modifying the real staging area.
// This matches the bash implementation in git-state.sh.
func CaptureCurrentTree() (string, error) {
	// Create temp index file
	tmpIndex, err := os.CreateTemp("", "git-index-*")
	if err != nil {
		return "", err
	}
	tmpIndexPath := tmpIndex.Name()
	tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	// Helper to run git commands with GIT_INDEX_FILE set
	gitWithTempIndex := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)
		return cmd
	}

	// Initialize temp index with HEAD tree (or empty if no commits)
	headRef, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil && len(headRef) > 0 {
		gitWithTempIndex("read-tree", strings.TrimSpace(string(headRef))).Run()
	} else {
		gitWithTempIndex("read-tree", "--empty").Run()
	}

	// Add tracked file changes (staged and unstaged)
	gitWithTempIndex("add", "-u", ".").Run()

	// Add untracked files (respecting .gitignore)
	lsCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedOutput, _ := lsCmd.Output()
	if len(untrackedOutput) > 0 {
		scanner := bufio.NewScanner(bytes.NewReader(untrackedOutput))
		for scanner.Scan() {
			path := scanner.Text()
			if path != "" {
				gitWithTempIndex("add", path).Run()
			}
		}
	}

	// Write tree from temp index
	writeCmd := gitWithTempIndex("write-tree")
	output, err := writeCmd.Output()
	if err != nil {
		return "", err
	}

	treeSHA := strings.TrimSpace(string(output))
	if treeSHA == "" {
		return "", exec.ErrNotFound
	}

	return treeSHA, nil
}
