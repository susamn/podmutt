package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlayerDecorationDefaultsOffAndAcceptsExplicitBoolean(t *testing.T) {
	if Defaults().PlayerDecoration {
		t.Fatal("decoration is not opt-in")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("player_decoration = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if cfg, err := Load(path); err != nil || !cfg.PlayerDecoration {
		t.Fatal(cfg, err)
	}
	if err := os.WriteFile(path, []byte("player_decoration = 'yes'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid decoration type accepted")
	}
}
