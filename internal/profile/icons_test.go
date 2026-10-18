package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguredIconModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := Load(path)
	if err != nil || cfg.Icons != "unicode" {
		t.Fatal("Unicode not default", cfg, err)
	}
	for _, mode := range []string{"unicode", "nerd", "plain", "invalid", ""} {
		if err := os.WriteFile(path, []byte("icons = \""+mode+"\"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err = Load(path)
		if mode == "invalid" || mode == "" {
			if err == nil {
				t.Fatal("invalid icon mode accepted")
			}
		} else if err != nil || cfg.Icons != mode {
			t.Fatal("icon configuration not honored", mode, cfg, err)
		}
	}
}
