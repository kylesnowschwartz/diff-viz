// Command git-diff-tree displays hierarchical diff visualization.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kylesnowschwartz/diff-viz/diff"
	"github.com/kylesnowschwartz/diff-viz/render"
	"golang.org/x/term"
)

func usage() string {
	var sb strings.Builder
	sb.WriteString(`git-diff-tree - Hierarchical diff visualization

Usage:
  git-diff-tree [flags] [<commit> [<commit>]]

Examples:
  git-diff-tree                    Working tree vs HEAD
  git-diff-tree --cached           Staged changes only
  git-diff-tree HEAD~3             Last 3 commits
  git-diff-tree main feature       Compare branches
  git-diff-tree -m smart           Compact sparkline view
  git-diff-tree --demo             Show all modes (root..HEAD)
  git-diff-tree --stats-json       Output raw diff stats as JSON

Modes:
`)
	for _, mode := range render.ValidModes {
		sb.WriteString(fmt.Sprintf("  %-10s %s\n", mode, render.ModeDescriptions[mode]))
	}
	sb.WriteString("\nFlags:\n")
	return sb.String()
}

// Renderer interface for diff output.
type Renderer interface {
	Render(stats *diff.DiffStats)
}

func main() {
	// Custom usage
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage())
		flag.PrintDefaults()
	}

	// Parse flags
	mode := flag.String("m", "tree", "Output mode (shorthand)")
	modeLong := flag.String("mode", "tree", "Output mode: "+strings.Join(render.ValidModes, ", "))
	noColor := flag.Bool("no-color", false, "Disable color output")
	width := flag.Int("width", 100, "Output width in columns (for icicle mode)")
	depth := flag.Int("depth", 4, "Max hierarchy depth to render (for icicle mode, 0=unlimited)")
	help := flag.Bool("h", false, "Show help")
	listModes := flag.Bool("list-modes", false, "List valid modes (for scripting)")
	demo := flag.Bool("demo", false, "Show all visualization modes (compares HEAD to root commit)")
	statsJSON := flag.Bool("stats-json", false, "Output raw diff stats as JSON (for programmatic consumption)")
	baseline := flag.String("baseline", "", "Baseline tree SHA to compare against (uses current working tree)")
	verbose := flag.Bool("v", false, "Print warnings to stderr")
	verboseLong := flag.Bool("verbose", false, "Print warnings to stderr")
	expand := flag.Int("expand", -1, "Expansion depth for brackets mode (-1=auto, 0=inline, 1+=expand to depth)")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *listModes {
		fmt.Println(strings.Join(render.ValidModes, " "))
		os.Exit(0)
	}

	// Use -m if set, otherwise --mode
	selectedMode := *modeLong
	modeExplicitlySet := false
	if *mode != "tree" {
		selectedMode = *mode
		modeExplicitlySet = true
	} else if *modeLong != "tree" {
		modeExplicitlySet = true
	}

	if *demo {
		if modeExplicitlySet {
			if !render.IsValidMode(selectedMode) {
				fmt.Fprintf(os.Stderr, "unknown mode: %s (valid: %s)\n", selectedMode, strings.Join(render.ValidModes, ", "))
				os.Exit(1)
			}
			runDemoSingleMode(selectedMode, !*noColor, *width, *depth, *expand)
		} else {
			runDemo(!*noColor, *width, *depth, *expand)
		}
		return
	}

	// Resolve verbose flag
	showWarnings := *verbose || *verboseLong

	// Handle --stats-json mode (raw stats for programmatic consumption)
	if *statsJSON {
		outputStatsJSON(*baseline, showWarnings)
		return
	}

	// Validate mode
	if !render.IsValidMode(selectedMode) {
		fmt.Fprintf(os.Stderr, "unknown mode: %s (valid: %s)\n", selectedMode, strings.Join(render.ValidModes, ", "))
		os.Exit(1)
	}

	// Get diff stats with remaining args
	stats, warnings, err := diff.GetAllStats(flag.Args()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWarnings(warnings, showWarnings)

	useColor := !*noColor

	// Select renderer based on mode
	renderer := getRenderer(selectedMode, useColor, *width, *depth, *expand)
	renderer.Render(stats)
}

// printWarnings outputs warnings to stderr if verbose mode is enabled.
func printWarnings(warnings []string, verbose bool) {
	if !verbose || len(warnings) == 0 {
		return
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
}

// outputStatsJSON outputs raw diff stats as JSON.
// This provides a stable interface for programmatic consumers
// without requiring Go import coupling.
func outputStatsJSON(baseline string, verbose bool) {
	var stats *diff.DiffStats
	var warnings []string
	var err error

	if baseline != "" {
		currentTree, err := diff.CaptureCurrentTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error capturing tree: %v\n", err)
			os.Exit(1)
		}
		stats, warnings, err = diff.GetTreeDiffStats(baseline, currentTree)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		stats, warnings, err = diff.GetAllStats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	printWarnings(warnings, verbose)

	output, err := json.Marshal(stats.ToJSON())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

// getDemoStats returns diff stats for root..HEAD (used by demo modes).
func getDemoStats() (*diff.DiffStats, error) {
	out, err := exec.Command("git", "rev-list", "--max-parents=0", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("could not find root commit: %w", err)
	}
	root := strings.TrimSpace(string(out))

	stats, _, err := diff.GetDiffStats(root + "..HEAD")
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// runDemoSingleMode shows a single visualization mode using root..HEAD diff.
func runDemoSingleMode(mode string, useColor bool, width, depth, expand int) {
	stats, err := getDemoStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if stats.TotalFiles == 0 {
		fmt.Println("No changes to display (root..HEAD is empty)")
		return
	}

	fmt.Printf("=== %s ===\n", mode)
	renderer := getRenderer(mode, useColor, width, depth, expand)
	renderer.Render(stats)
}

// runDemo shows all visualization modes using root..HEAD diff.
func runDemo(useColor bool, width, depth, expand int) {
	stats, err := getDemoStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if stats.TotalFiles == 0 {
		fmt.Println("No changes to display (root..HEAD is empty)")
		return
	}

	for i, mode := range render.ValidModes {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s ===\n", mode)
		renderer := getRenderer(mode, useColor, width, depth, expand)
		renderer.Render(stats)
	}
}

// getTerminalWidth returns the terminal width to use for rendering.
// Priority: flag value (if not default) > terminal detection > default (100).
func getTerminalWidth(flagWidth int) int {
	if flagWidth != 100 { // User explicitly set via flag
		return flagWidth
	}
	// Try to detect terminal width
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	return 100 // sensible default for modern terminals
}

func getRenderer(mode string, useColor bool, width, depth, expand int) Renderer {
	switch mode {
	case "tree":
		return render.NewTreeRenderer(os.Stdout, useColor)
	case "collapsed":
		return render.NewCollapsedRenderer(os.Stdout, useColor)
	case "smart":
		return render.NewSmartSparklineRenderer(os.Stdout, useColor)
	case "topn":
		return render.NewTopNRenderer(os.Stdout, useColor, 5)
	case "icicle":
		r := render.NewIcicleRenderer(os.Stdout, useColor)
		r.Width = width
		r.MaxDepth = depth
		return r
	case "brackets":
		r := render.NewBracketsRenderer(os.Stdout, useColor)
		r.Width = getTerminalWidth(width)
		r.ExpandDepth = expand
		return r
	default:
		// Should never reach here if isValidMode was called first
		return render.NewTreeRenderer(os.Stdout, useColor)
	}
}
