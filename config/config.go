package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the full configuration file structure.
type Config struct {
	Defaults ModeConfig            `json:"defaults,omitempty"`
	Modes    map[string]ModeConfig `json:"modes,omitempty"`
}

// ModeConfig holds configuration for a single mode or defaults.
// All fields are pointers to distinguish "not set" from "set to zero".
type ModeConfig struct {
	Width  *int `json:"width,omitempty"`
	Depth  *int `json:"depth,omitempty"`
	Expand *int `json:"expand,omitempty"`
	N      *int `json:"n,omitempty"` // TopN-specific
}

// ResolvedConfig holds the final resolved values (no pointers, always has values).
type ResolvedConfig struct {
	Width  int
	Depth  int
	Expand int
	N      int
}

// Load reads and parses a config file from the given path.
// Returns nil config (not error) if path is empty.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// Resolve combines defaults, config file, and CLI flags for a specific mode.
// Precedence: global defaults < mode defaults < config.defaults < config.modes[mode] < CLI flags.
func (c *Config) Resolve(mode string, cliFlags *ModeConfig) ResolvedConfig {
	// Start with hardcoded global defaults
	result := DefaultConfig()

	// Apply built-in mode-specific defaults
	if modeConfig, ok := ModeDefaults[mode]; ok {
		result = mergeConfig(result, modeConfig)
	}

	if c != nil {
		// Apply config file defaults
		result = mergeConfig(result, c.Defaults)

		// Apply mode-specific config
		if modeConfig, ok := c.Modes[mode]; ok {
			result = mergeConfig(result, modeConfig)
		}
	}

	// Apply CLI flags (if provided)
	if cliFlags != nil {
		result = mergeConfig(result, *cliFlags)
	}

	return result
}

// Resolve without a config file - uses only defaults and CLI flags.
func Resolve(mode string, cliFlags *ModeConfig) ResolvedConfig {
	var nilConfig *Config
	return nilConfig.Resolve(mode, cliFlags)
}

// mergeConfig overlays src onto base, only replacing non-nil values.
func mergeConfig(base ResolvedConfig, src ModeConfig) ResolvedConfig {
	if src.Width != nil {
		base.Width = *src.Width
	}
	if src.Depth != nil {
		base.Depth = *src.Depth
	}
	if src.Expand != nil {
		base.Expand = *src.Expand
	}
	if src.N != nil {
		base.N = *src.N
	}
	return base
}
