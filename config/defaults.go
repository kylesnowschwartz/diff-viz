// Package config provides configuration types and loading for diff-viz renderers.
package config

// Default global values.
const (
	DefaultWidth  = 100
	DefaultDepth  = 2
	DefaultExpand = -1 // auto
	DefaultN      = 5
	DefaultMode   = "tree"
)

// ModeDefaults provides optimized defaults for each render mode.
// These are applied after global defaults but before config file values.
var ModeDefaults = map[string]ModeConfig{
	"tree":     {},                   // uses global defaults
	"smart":    {Depth: intPtr(3)},   // show individual files by default
	"topn":     {N: intPtr(10)},      // show more files
	"icicle":   {Depth: intPtr(4)},   // deeper hierarchy
	"brackets": {Expand: intPtr(-1)}, // auto
}

// DefaultConfig returns the hardcoded global default configuration.
func DefaultConfig() ResolvedConfig {
	return ResolvedConfig{
		Width:  DefaultWidth,
		Depth:  DefaultDepth,
		Expand: DefaultExpand,
		N:      DefaultN,
	}
}

// DefaultsForMode returns resolved defaults for a specific mode,
// applying ModeDefaults on top of global defaults.
func DefaultsForMode(mode string) ResolvedConfig {
	result := DefaultConfig()
	if modeConfig, ok := ModeDefaults[mode]; ok {
		result = mergeConfig(result, modeConfig)
	}
	return result
}

// DefaultConfigJSON returns a Config struct suitable for serializing
// as a starting template. Includes all built-in mode defaults.
func DefaultConfigJSON() Config {
	width := DefaultWidth
	depth := DefaultDepth
	expand := DefaultExpand

	return Config{
		Defaults: ModeConfig{
			Width:  &width,
			Depth:  &depth,
			Expand: &expand,
		},
		Modes: copyModeDefaults(),
	}
}

// copyModeDefaults returns a deep copy of ModeDefaults for safe mutation.
func copyModeDefaults() map[string]ModeConfig {
	result := make(map[string]ModeConfig, len(ModeDefaults))
	for k, v := range ModeDefaults {
		// Skip empty configs
		if v.Width == nil && v.Depth == nil && v.Expand == nil && v.N == nil {
			continue
		}
		result[k] = ModeConfig{
			Width:  copyIntPtr(v.Width),
			Depth:  copyIntPtr(v.Depth),
			Expand: copyIntPtr(v.Expand),
			N:      copyIntPtr(v.N),
		}
	}
	return result
}

func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func intPtr(i int) *int {
	return &i
}
