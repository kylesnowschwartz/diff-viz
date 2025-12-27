package render

// ValidModes is the canonical list of available visualization modes.
var ValidModes = []string{"tree", "smart", "topn", "icicle", "brackets"}

// ModeDescriptions provides help text for each mode.
var ModeDescriptions = map[string]string{
	"tree":     "Indented tree with file stats (default)",
	"smart":    "Depth-aggregated sparkline (--depth=1 for top-level only)",
	"topn":     "Top N files by change size (--count=N, --sort=total|adds|dels)",
	"icicle":   "Horizontal icicle chart (width = magnitude)",
	"brackets": "Nested brackets [dir file... file...] (single-line hierarchy)",
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
