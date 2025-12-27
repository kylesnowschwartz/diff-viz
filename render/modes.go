package render

// ValidModes is the canonical list of available visualization modes.
var ValidModes = []string{"tree", "collapsed", "smart", "topn", "icicle", "brackets"}

// ModeDescriptions provides help text for each mode.
var ModeDescriptions = map[string]string{
	"tree":      "Indented tree with file stats (default)",
	"collapsed": "Single-line summary per directory",
	"smart":     "Depth-2 aggregated sparkline",
	"topn":      "Top N files by change size (hotspots)",
	"icicle":    "Horizontal icicle chart (width = magnitude)",
	"brackets":  "Nested brackets [dir file... file...] (single-line hierarchy)",
}

// IsValidMode returns true if mode is a recognized visualization mode.
func IsValidMode(mode string) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}
