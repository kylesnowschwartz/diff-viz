// Command git-diff-tree displays hierarchical diff visualization.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/kylesnowschwartz/diff-viz/v2/config"
	"github.com/kylesnowschwartz/diff-viz/v2/diff"
	"github.com/kylesnowschwartz/diff-viz/v2/render"
	"golang.org/x/term"
)

func usage() string {
	var sb strings.Builder
	sb.WriteString(`git-diff-tree - Hierarchical diff visualization

Usage:
  git-diff-tree [flags] [mode] [<commit> [<commit>]]

Examples:
  git-diff-tree                    Working tree vs HEAD
  git-diff-tree --cached           Staged changes only
  git-diff-tree HEAD~3             Last 3 commits
  git-diff-tree main feature       Compare branches
  git-diff-tree smart              Compact sparkline view (mode as positional)
  git-diff-tree -m smart           Compact sparkline view (mode as flag)
  git-diff-tree icicle HEAD~5      Icicle chart of last 5 commits
  git-diff-tree --demo             Show all modes (root..HEAD)
  git-diff-tree --stats-json       Output raw diff stats as JSON
  git-diff-tree --config cfg.json  Use config file for mode defaults
  git-diff-tree --dump-defaults    Output default config as JSON template

Modes:
`)
	for _, mode := range render.ValidModes {
		sb.WriteString(fmt.Sprintf("  %-10s %s\n", mode, render.ModeDescriptions[mode]))
	}
	sb.WriteString("\nFlags:\n")
	return sb.String()
}

func main() {
	// Custom usage
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage())
		flag.PrintDefaults()
	}

	// Parse flags (pflag supports interspersed flags and positional args)
	mode := flag.StringP("mode", "m", "tree", "Output mode: "+strings.Join(render.ValidModes, ", "))
	noColor := flag.Bool("no-color", false, "Disable color output")
	width := flag.Int("width", 100, "Output width in columns (smart, icicle, brackets)")
	depth := flag.Int("depth", 2, "Hierarchy depth (smart: 1=top-level, 2+=subdir depth; icicle: 0=unlimited)")
	help := flag.BoolP("help", "h", false, "Show help")
	listModes := flag.Bool("list-modes", false, "List valid modes (for scripting)")
	demo := flag.Bool("demo", false, "Show all visualization modes (compares HEAD to root commit)")
	statsJSON := flag.Bool("stats-json", false, "Output raw diff stats as JSON (for programmatic consumption)")
	version := flag.Bool("version", false, "Show version information")
	baseline := flag.String("baseline", "", "Baseline tree SHA to compare against (uses current working tree)")
	cached := flag.Bool("cached", false, "Show staged changes only")
	quiet := flag.BoolP("quiet", "q", false, "Suppress warnings")
	expand := flag.Int("expand", -1, "Expansion depth for brackets mode (-1=auto, 0=inline, 1+=expand to depth)")
	topnCount := flag.Int("count", 10, "Number of files to show (sparkline-tree --files)")
	topnSort := flag.String("sort", "total", "Sort order (sparkline-tree --files): total, adds, dels")
	showFiles := flag.Bool("files", false, "Show flat file list instead of tree (sparkline-tree)")
	configPath := flag.String("config", "", "Path to JSON config file")
	dumpDefaults := flag.Bool("dump-defaults", false, "Output default config as JSON")
	flag.Parse()

	if *version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			fmt.Printf("git-diff-tree %s\n", info.Main.Version)
		} else {
			fmt.Println("git-diff-tree (development build)")
		}
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *dumpDefaults {
		cfg := config.DefaultConfigJSON()
		output, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(output))
		os.Exit(0)
	}

	if *listModes {
		fmt.Println(strings.Join(render.ValidModes, " "))
		os.Exit(0)
	}

	// Check if mode was explicitly set
	selectedMode := *mode
	modeExplicitlySet := flagWasSet("mode")

	// Consume mode from positional args if present (e.g., "git-diff-tree tree" works like "git-diff-tree -m tree")
	args := flag.Args()
	if len(args) > 0 && render.IsValidMode(args[0]) && !modeExplicitlySet {
		selectedMode = args[0]
		modeExplicitlySet = true
		args = args[1:]
	}

	// Load config file (if provided) - needed for demo and regular modes
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Build CLI flags struct (only for explicitly-set flags)
	var cliFlags *config.ModeConfig
	if flagWasSet("width") || flagWasSet("depth") || flagWasSet("expand") || flagWasSet("count") {
		cliFlags = &config.ModeConfig{}
		if flagWasSet("width") {
			cliFlags.Width = width
		}
		if flagWasSet("depth") {
			cliFlags.Depth = depth
		}
		if flagWasSet("expand") {
			cliFlags.Expand = expand
		}
		if flagWasSet("count") {
			cliFlags.N = topnCount
		}
	}

	// Resolve warnings flag (shown by default, --quiet suppresses)
	showWarnings := !*quiet

	if *demo {
		if modeExplicitlySet {
			if !render.IsValidMode(selectedMode) {
				fmt.Fprintf(os.Stderr, "unknown mode: %s (valid: %s)\n", selectedMode, strings.Join(render.ValidModes, ", "))
				os.Exit(1)
			}
			runDemoSingleMode(selectedMode, !*noColor, showWarnings, cfg, cliFlags, *topnSort, *showFiles)
		} else {
			runDemo(!*noColor, showWarnings, cfg, cliFlags, *topnSort)
		}
		return
	}

	// Handle --cached flag (prepend to args)
	if *cached {
		args = append([]string{"--cached"}, args...)
	}

	// Handle --stats-json mode (raw stats for programmatic consumption)
	if *statsJSON {
		outputStatsJSON(*baseline, *cached, showWarnings, args)
		return
	}

	// Validate mode
	if !render.IsValidMode(selectedMode) {
		fmt.Fprintf(os.Stderr, "unknown mode: %s (valid: %s)\n", selectedMode, strings.Join(render.ValidModes, ", "))
		os.Exit(1)
	}

	// Resolve final configuration (config already loaded above)
	resolved := cfg.Resolve(selectedMode, cliFlags)

	// Get diff stats (handles --baseline and --cached)
	stats, warnings, err := getStats(*baseline, *cached, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWarnings(warnings, showWarnings)

	useColor := !*noColor

	// Select renderer based on mode
	renderer := getRenderer(selectedMode, useColor, resolved.Width, resolved.Depth, resolved.Expand, resolved.N, *topnSort, *showFiles, args)
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

// getStats returns diff stats, handling baseline comparison and cached mode.
// - baseline != "": Compare current working tree against baseline SHA
// - cached: Show only staged changes (no untracked files)
// - default: Show working tree vs HEAD (includes untracked files)
func getStats(baseline string, cached bool, args []string) (*diff.DiffStats, []string, error) {
	if baseline != "" {
		currentTree, err := diff.CaptureCurrentTree()
		if err != nil {
			return nil, nil, fmt.Errorf("capturing tree: %w", err)
		}
		return diff.GetTreeDiffStats(baseline, currentTree)
	}
	if cached {
		return diff.GetDiffStats(args...)
	}
	return diff.GetAllStats(args...)
}

// outputStatsJSON outputs raw diff stats as JSON.
// This provides a stable interface for programmatic consumers
// without requiring Go import coupling.
func outputStatsJSON(baseline string, cached bool, verbose bool, args []string) {
	stats, warnings, err := getStats(baseline, cached, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWarnings(warnings, verbose)

	output, err := json.Marshal(stats.ToJSON())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

// getDemoStats returns diff stats, git args, and warnings for root..HEAD (used by demo modes).
func getDemoStats() (*diff.DiffStats, []string, []string, error) {
	out, err := exec.Command("git", "rev-list", "--max-parents=0", "HEAD").Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not find root commit: %w", err)
	}
	roots := strings.Split(strings.TrimSpace(string(out)), "\n")
	root := roots[0] // Take first root if multiple (grafted history, merged repos)
	diffRange := root + "..HEAD"

	stats, warnings, err := diff.GetDiffStats(diffRange)
	if err != nil {
		return nil, nil, nil, err
	}
	return stats, []string{diffRange}, warnings, nil
}

// runDemoSingleMode shows a single visualization mode using root..HEAD diff.
func runDemoSingleMode(mode string, useColor bool, showWarnings bool, cfg *config.Config, cliFlags *config.ModeConfig, topnSort string, showFiles bool) {
	stats, args, warnings, err := getDemoStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWarnings(warnings, showWarnings)

	if stats.TotalFiles == 0 {
		fmt.Println("No changes to display (root..HEAD is empty)")
		return
	}

	resolved := cfg.Resolve(mode, cliFlags)
	fmt.Printf("=== %s ===\n", mode)
	renderer := getRenderer(mode, useColor, resolved.Width, resolved.Depth, resolved.Expand, resolved.N, topnSort, showFiles, args)
	renderer.Render(stats)
}

// runDemo shows all visualization modes using root..HEAD diff.
func runDemo(useColor bool, showWarnings bool, cfg *config.Config, cliFlags *config.ModeConfig, topnSort string) {
	stats, args, warnings, err := getDemoStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWarnings(warnings, showWarnings)

	if stats.TotalFiles == 0 {
		fmt.Println("No changes to display (root..HEAD is empty)")
		return
	}

	for i, mode := range render.ValidModes {
		if i > 0 {
			fmt.Println()
		}
		resolved := cfg.Resolve(mode, cliFlags)
		fmt.Printf("=== %s ===\n", mode)
		renderer := getRenderer(mode, useColor, resolved.Width, resolved.Depth, resolved.Expand, resolved.N, topnSort, false, args)
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

func getRenderer(mode string, useColor bool, width, depth, expand, count int, sortBy string, showFiles bool, args []string) render.Renderer {
	switch mode {
	case "tree":
		return render.NewTreeRenderer(os.Stdout, useColor)
	case "plain":
		return render.NewPlainRenderer(os.Stdout)
	case "smart":
		r := render.NewSmartSparklineRenderer(os.Stdout, useColor)
		r.MaxDepth = depth
		r.Width = getTerminalWidth(width)
		return r
	case "sparkline-tree":
		r := render.NewSparklineTreeRenderer(os.Stdout, useColor)
		r.MaxDepth = depth
		r.ShowFiles = showFiles
		r.N = count
		r.SortBy = render.SortBy(sortBy)
		return r
	case "hotpath":
		r := render.NewHotpathRenderer(os.Stdout, useColor)
		r.MaxDepth = depth
		return r
	case "icicle":
		r := render.NewIcicleRenderer(os.Stdout, useColor)
		r.Width = getTerminalWidth(width)
		r.MaxDepth = depth
		return r
	case "brackets":
		r := render.NewBracketsRenderer(os.Stdout, useColor)
		r.Width = getTerminalWidth(width)
		r.ExpandDepth = expand
		return r
	case "gauge":
		return render.NewGaugeRenderer(os.Stdout, useColor)
	case "depth":
		r := render.NewDepthRenderer(os.Stdout, useColor)
		r.MaxDepth = depth
		return r
	case "stat":
		return render.NewStatRenderer(os.Stdout, args)
	default:
		// Should never reach here if isValidMode was called first
		return render.NewTreeRenderer(os.Stdout, useColor)
	}
}

// flagWasSet returns true if the flag was explicitly provided on command line.
func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
