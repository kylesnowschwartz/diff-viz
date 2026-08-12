package render

// ValidModes is the canonical list of available visualization modes.
var ValidModes = []string{"tree", "smart", "sparkline-tree", "hotpath", "icicle", "brackets", "gauge", "depth", "stat", "plain"}

// ModeDescriptions provides help text for each mode.
var ModeDescriptions = map[string]string{
	"tree":           "Indented tree with file stats (default)",
	"smart":          "Multi-column table sorted by magnitude",
	"sparkline-tree": "Rainbow sidebar tree with sparkline bars (--files for flat list)",
	"hotpath":        "Hot trail view (follows largest child at each level)",
	"icicle":         "Horizontal icicle chart (width = magnitude)",
	"brackets":       "Nested brackets [dir file... file...] (single-line hierarchy)",
	"gauge":          "Progress gauge showing change magnitude",
	"depth":          "Nested gauges showing change distribution by depth",
	"stat":           "Native git diff --stat output (unchanged)",
	"plain":          "Model-facing plain text: additions-ranked, full paths, no ANSI",
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
