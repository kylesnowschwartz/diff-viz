package render

// ValidModes is the canonical list of available visualization modes.
var ValidModes = []string{"tree", "smart", "sparkline-tree", "icicle", "brackets", "gauge", "depth", "heatmap"}

// ModeDescriptions provides help text for each mode.
var ModeDescriptions = map[string]string{
	"tree":           "Indented tree with file stats (default)",
	"smart":          "Depth-aggregated sparkline (--depth=1 collapsed, 2 subdirs)",
	"sparkline-tree": "Rainbow sidebar tree with sparkline bars (--files for flat list)",
	"icicle":         "Horizontal icicle chart (width = magnitude)",
	"brackets":       "Nested brackets [dir file... file...] (single-line hierarchy)",
	"gauge":          "Progress gauge showing change magnitude",
	"depth":          "Nested gauges showing change distribution by depth",
	"heatmap":        "Heatmap matrix (rows=dirs, cols=depth levels)",
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
