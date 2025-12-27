package render

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
)

// PathSegment represents aggregated file changes at depth-2.
// Used by renderers that group files by directory structure.
type PathSegment struct {
	TopDir    string   // Top-level directory (e.g., "src", "tests")
	SubPath   string   // Depth-2 path or filename (e.g., "lib", "main.go")
	Files     []string // List of file paths in this segment
	Add       int      // Total additions
	Del       int      // Total deletions
	FileCount int      // Number of files
	HasNew    bool     // Contains untracked/new files
	IsFile    bool     // True if SubPath is a single file (not aggregated dir)
}

// Total returns the sum of additions and deletions.
func (p PathSegment) Total() int {
	return p.Add + p.Del
}

// Totaler is implemented by types that can report their total changes.
type Totaler interface {
	Total() int
}

// GetTopDir extracts the top-level directory from a file path.
// Returns the filename itself if the file is at the root.
func GetTopDir(path string) string {
	idx := strings.Index(path, "/")
	if idx == -1 {
		return path
	}
	return path[:idx]
}

// ParseDepthPath extracts grouping based on max depth.
// Returns (groupKey, subPath, isFile).
//
// groupKey is always the top-level directory (or filename for root files).
// subPath is the component at the requested depth level.
// isFile is true if subPath represents a single file rather than an aggregate.
//
// Examples for "src/lib/utils/helper.go":
//   - depth=1: ("src", "src", false)       - aggregate all under src/
//   - depth=2: ("src", "lib", false)       - aggregate all under src/lib/
//   - depth=3: ("src", "utils", false)     - aggregate all under src/lib/utils/
//   - depth=4: ("src", "helper.go", true)  - show individual file
//
// Files shallower than requested depth show as files:
//   - depth=3, "src/main.go": ("src", "main.go", true)
func ParseDepthPath(filePath string, maxDepth int) (groupKey, subPath string, isFile bool) {
	parts := strings.Split(filePath, "/")

	// Root file (no directories)
	if len(parts) == 1 {
		return parts[0], parts[0], true
	}

	groupKey = parts[0]
	displayIndex := maxDepth - 1 // depth=1 → index 0, depth=2 → index 1, etc.

	if displayIndex == 0 {
		// depth=1: aggregate everything under top-level
		return groupKey, groupKey, false
	}

	if displayIndex >= len(parts) {
		// File is shallower than requested depth - show filename
		return groupKey, parts[len(parts)-1], true
	}

	// File at or deeper than display depth
	isFileAtDepth := displayIndex == len(parts)-1
	return groupKey, parts[displayIndex], isFileAtDepth
}

// ParseDepth2Path extracts top-level dir and depth-2 grouping from a path.
// Deprecated: Use ParseDepthPath with maxDepth=2 instead.
func ParseDepth2Path(filePath string) (topDir, subPath string, isFile bool) {
	return ParseDepthPath(filePath, 2)
}

// GroupByDepth groups files by directory structure at the specified depth.
// maxDepth=1: aggregate at top-level only (collapsed behavior)
// maxDepth=2: group by top-level, then depth-2
// Returns a map of groupKey -> sorted slice of PathSegments.
func GroupByDepth(files []diff.FileStat, maxDepth int) map[string][]PathSegment {
	// First pass: build nested map
	groupMap := make(map[string]map[string]*PathSegment)

	for _, f := range files {
		groupKey, subPath, isFile := ParseDepthPath(f.Path, maxDepth)

		if groupMap[groupKey] == nil {
			groupMap[groupKey] = make(map[string]*PathSegment)
		}

		if groupMap[groupKey][subPath] == nil {
			groupMap[groupKey][subPath] = &PathSegment{
				TopDir:  groupKey,
				SubPath: subPath,
				IsFile:  isFile,
			}
		}

		seg := groupMap[groupKey][subPath]
		seg.Files = append(seg.Files, f.Path)
		seg.Add += f.Additions
		seg.Del += f.Deletions
		seg.FileCount++
		if f.IsUntracked {
			seg.HasNew = true
		}
	}

	// Convert to slices, sorted by total changes within each group
	result := make(map[string][]PathSegment)
	for groupKey, subGroups := range groupMap {
		segments := make([]PathSegment, 0, len(subGroups))
		for _, seg := range subGroups {
			// At depth=2+, convert single-file groups to file display
			// At depth=1, keep directory aggregates even for single files
			if maxDepth >= 2 && seg.FileCount == 1 {
				seg.SubPath = filepath.Base(seg.Files[0])
				seg.IsFile = true
			}
			segments = append(segments, *seg)
		}
		// Sort by total changes descending
		sort.Slice(segments, func(i, j int) bool {
			return segments[i].Total() > segments[j].Total()
		})
		result[groupKey] = segments
	}

	return result
}

// GroupByTopDir groups files first by top-level dir, then by depth-2 path.
// Deprecated: Use GroupByDepth with maxDepth=2 instead.
func GroupByTopDir(files []diff.FileStat) map[string][]PathSegment {
	// First pass: build nested map
	groupMap := make(map[string]map[string]*PathSegment)

	for _, f := range files {
		topDir, subPath, isFile := ParseDepth2Path(f.Path)

		if groupMap[topDir] == nil {
			groupMap[topDir] = make(map[string]*PathSegment)
		}

		if groupMap[topDir][subPath] == nil {
			groupMap[topDir][subPath] = &PathSegment{
				TopDir:  topDir,
				SubPath: subPath,
				IsFile:  isFile,
			}
		}

		seg := groupMap[topDir][subPath]
		seg.Files = append(seg.Files, f.Path)
		seg.Add += f.Additions
		seg.Del += f.Deletions
		seg.FileCount++
		if f.IsUntracked {
			seg.HasNew = true
		}
	}

	// Convert to slices, sorted by total changes within each top dir
	result := make(map[string][]PathSegment)
	for topDir, subGroups := range groupMap {
		segments := make([]PathSegment, 0, len(subGroups))
		for _, seg := range subGroups {
			// If group has only 1 file, convert to file display
			if seg.FileCount == 1 {
				seg.SubPath = filepath.Base(seg.Files[0])
				seg.IsFile = true
			}
			segments = append(segments, *seg)
		}
		// Sort by total changes descending
		sort.Slice(segments, func(i, j int) bool {
			return segments[i].Total() > segments[j].Total()
		})
		result[topDir] = segments
	}

	return result
}

// SortTopDirs returns top-level directory names sorted by total changes (descending).
// Works with any slice type that implements Totaler.
func SortTopDirs[T Totaler](groups map[string][]T) []string {
	type dirTotal struct {
		name  string
		total int
	}

	totals := make([]dirTotal, 0, len(groups))
	for name, items := range groups {
		total := 0
		for _, item := range items {
			total += item.Total()
		}
		totals = append(totals, dirTotal{name, total})
	}

	sort.Slice(totals, func(i, j int) bool {
		return totals[i].total > totals[j].total
	})

	result := make([]string, len(totals))
	for i, t := range totals {
		result[i] = t.name
	}
	return result
}
