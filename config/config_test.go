package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Errorf("Load empty path: got error %v, want nil", err)
	}
	if cfg != nil {
		t.Errorf("Load empty path: got %+v, want nil", cfg)
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.json")
	if err == nil {
		t.Error("Load nonexistent file: got nil error, want error")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	content := `{
		"defaults": {"width": 80, "depth": 3},
		"modes": {
			"topn": {"n": 15},
			"icicle": {"depth": 6}
		}
	}`

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Defaults.Width == nil || *cfg.Defaults.Width != 80 {
		t.Errorf("Defaults.Width: got %v, want 80", cfg.Defaults.Width)
	}
	if cfg.Defaults.Depth == nil || *cfg.Defaults.Depth != 3 {
		t.Errorf("Defaults.Depth: got %v, want 3", cfg.Defaults.Depth)
	}
	if cfg.Modes["topn"].N == nil || *cfg.Modes["topn"].N != 15 {
		t.Errorf("Modes[topn].N: got %v, want 15", cfg.Modes["topn"].N)
	}
	if cfg.Modes["icicle"].Depth == nil || *cfg.Modes["icicle"].Depth != 6 {
		t.Errorf("Modes[icicle].Depth: got %v, want 6", cfg.Modes["icicle"].Depth)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(cfgPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("Load invalid JSON: got nil error, want error")
	}
}

func TestResolve_Precedence(t *testing.T) {
	// Test the full precedence chain:
	// hardcoded globals < built-in ModeDefaults < config.defaults < config.modes[mode] < CLI flags

	// Hardcoded defaults: width=100, depth=2, expand=-1, n=5
	// ModeDefaults for topn: n=10
	// Config defaults: width=80
	// Config modes[topn]: n=7
	// CLI flags: n=3

	content := `{
		"defaults": {"width": 80},
		"modes": {"topn": {"n": 7}}
	}`

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Test 1: No CLI flags - should use config mode value (n=7)
	resolved := cfg.Resolve("topn", nil)
	if resolved.N != 7 {
		t.Errorf("Resolve topn without CLI: N got %d, want 7", resolved.N)
	}
	if resolved.Width != 80 {
		t.Errorf("Resolve topn without CLI: Width got %d, want 80 (from config defaults)", resolved.Width)
	}
	if resolved.Depth != 2 {
		t.Errorf("Resolve topn without CLI: Depth got %d, want 2 (hardcoded default)", resolved.Depth)
	}

	// Test 2: With CLI flags - should use CLI value (n=3)
	n := 3
	cliFlags := &ModeConfig{N: &n}
	resolved = cfg.Resolve("topn", cliFlags)
	if resolved.N != 3 {
		t.Errorf("Resolve topn with CLI: N got %d, want 3", resolved.N)
	}

	// Test 3: Different mode - should use ModeDefaults for icicle (depth=4)
	resolved = cfg.Resolve("icicle", nil)
	if resolved.Depth != 4 {
		t.Errorf("Resolve icicle: Depth got %d, want 4 (from ModeDefaults)", resolved.Depth)
	}
	if resolved.Width != 80 {
		t.Errorf("Resolve icicle: Width got %d, want 80 (from config defaults)", resolved.Width)
	}

	// Test 4: Mode not in config - should use ModeDefaults
	resolved = cfg.Resolve("smart", nil)
	if resolved.Depth != 3 {
		t.Errorf("Resolve smart: Depth got %d, want 3 (from ModeDefaults)", resolved.Depth)
	}
}

func TestResolve_NilConfig(t *testing.T) {
	// Test that Resolve works with nil config (no config file)
	resolved := Resolve("topn", nil)

	// Should get ModeDefaults for topn: n=10
	if resolved.N != 10 {
		t.Errorf("Resolve nil config topn: N got %d, want 10 (ModeDefaults)", resolved.N)
	}

	// Should get hardcoded defaults for other values
	if resolved.Width != DefaultWidth {
		t.Errorf("Resolve nil config: Width got %d, want %d", resolved.Width, DefaultWidth)
	}

	// Test icicle with nil config - should get depth=4 from ModeDefaults
	resolved = Resolve("icicle", nil)
	if resolved.Depth != 4 {
		t.Errorf("Resolve nil config icicle: Depth got %d, want 4 (ModeDefaults)", resolved.Depth)
	}

	// Also test calling method on nil pointer directly (like main.go does)
	var cfg *Config = nil
	resolved = cfg.Resolve("icicle", nil)
	if resolved.Depth != 4 {
		t.Errorf("nil.Resolve icicle: Depth got %d, want 4 (ModeDefaults)", resolved.Depth)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Width != DefaultWidth {
		t.Errorf("DefaultConfig Width: got %d, want %d", cfg.Width, DefaultWidth)
	}
	if cfg.Depth != DefaultDepth {
		t.Errorf("DefaultConfig Depth: got %d, want %d", cfg.Depth, DefaultDepth)
	}
	if cfg.Expand != DefaultExpand {
		t.Errorf("DefaultConfig Expand: got %d, want %d", cfg.Expand, DefaultExpand)
	}
	if cfg.N != DefaultN {
		t.Errorf("DefaultConfig N: got %d, want %d", cfg.N, DefaultN)
	}
}

func TestDefaultsForMode(t *testing.T) {
	tests := []struct {
		mode  string
		depth int
		n     int
	}{
		{"tree", DefaultDepth, DefaultN},
		{"smart", 3, DefaultN},
		{"topn", DefaultDepth, 10},
		{"icicle", 4, DefaultN},
		{"brackets", DefaultDepth, DefaultN},
		{"unknown", DefaultDepth, DefaultN}, // Unknown mode uses globals
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := DefaultsForMode(tt.mode)
			if cfg.Depth != tt.depth {
				t.Errorf("DefaultsForMode(%s) Depth: got %d, want %d", tt.mode, cfg.Depth, tt.depth)
			}
			if cfg.N != tt.n {
				t.Errorf("DefaultsForMode(%s) N: got %d, want %d", tt.mode, cfg.N, tt.n)
			}
		})
	}
}

func TestDefaultConfigJSON(t *testing.T) {
	cfg := DefaultConfigJSON()

	// Check defaults section
	if cfg.Defaults.Width == nil || *cfg.Defaults.Width != DefaultWidth {
		t.Errorf("DefaultConfigJSON Defaults.Width: got %v, want %d", cfg.Defaults.Width, DefaultWidth)
	}

	// Check that mode-specific overrides are present
	if cfg.Modes["topn"].N == nil || *cfg.Modes["topn"].N != 10 {
		t.Errorf("DefaultConfigJSON Modes[topn].N: got %v, want 10", cfg.Modes["topn"].N)
	}
	if cfg.Modes["icicle"].Depth == nil || *cfg.Modes["icicle"].Depth != 4 {
		t.Errorf("DefaultConfigJSON Modes[icicle].Depth: got %v, want 4", cfg.Modes["icicle"].Depth)
	}
	if cfg.Modes["smart"].Depth == nil || *cfg.Modes["smart"].Depth != 3 {
		t.Errorf("DefaultConfigJSON Modes[smart].Depth: got %v, want 3", cfg.Modes["smart"].Depth)
	}

	// Tree mode should not be in Modes (empty config)
	if _, ok := cfg.Modes["tree"]; ok {
		t.Error("DefaultConfigJSON should not include empty tree mode")
	}
}
