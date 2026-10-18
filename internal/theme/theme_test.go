package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	p := Default()
	if p.Mode != "dark" || p.Accent != "#00FF00" || p.Red != "#DC3545" {
		t.Fatalf("unexpected default palette: %+v", p)
	}
}

func TestSolarizedDark(t *testing.T) {
	p := SolarizedDark()
	if p.Mode != "dark" || p.Background != "#002b36" || p.Accent != "#268bd2" {
		t.Fatalf("unexpected solarized-dark palette: %+v", p)
	}
}

func TestCatppuccin(t *testing.T) {
	p := Catppuccin()
	if p.Mode != "dark" || p.Background != "#1e1e2e" || p.Accent != "#a6e3a1" {
		t.Fatalf("unexpected catppuccin palette: %+v", p)
	}
}

func TestSerializeAndParse(t *testing.T) {
	orig := Catppuccin()
	serialized := Serialize(orig)
	if !strings.Contains(serialized, `mode = "dark"`) || !strings.Contains(serialized, `accent = "#a6e3a1"`) {
		t.Fatalf("unexpected serialization: %s", serialized)
	}
	parsed := Parse([]byte(serialized))
	if parsed.Accent != orig.Accent || parsed.Background != orig.Background || parsed.Red != orig.Red {
		t.Fatalf("roundtrip mismatch: orig=%+v, parsed=%+v", orig, parsed)
	}
}

func TestParseCommentsAndQuotes(t *testing.T) {
	input := `
# Full-line comment
mode = "dark"
accent = '#ff00ff' # single quotes with inline comment
red = #123456 # unquoted with comment
green = "#234567"
invalid_line_without_equals
`
	p := Parse([]byte(input))
	if p.Mode != "dark" || p.Accent != "#ff00ff" || p.Red != "#123456" || p.Green != "#234567" {
		t.Fatalf("unexpected parse output: %+v", p)
	}
}

func TestPreset(t *testing.T) {
	if Preset("solarized-dark").Background != SolarizedDark().Background {
		t.Fatal("solarized-dark preset failed")
	}
	if Preset("catppuccin").Background != Catppuccin().Background {
		t.Fatal("catppuccin preset failed")
	}
	if Preset("unknown").Background != Default().Background {
		t.Fatal("unknown preset fallback failed")
	}
}

func TestChannelColor(t *testing.T) {
	p := Default()
	c1 := p.ChannelColor("Podcast A")
	c2 := p.ChannelColor("Podcast A")
	if c1 != c2 {
		t.Fatalf("channel color not deterministic: %v vs %v", c1, c2)
	}
	c3 := p.ChannelColor("Podcast B")
	_ = c3 // Just ensure it runs without crashing
}

func TestEnsureFilesAndLoad(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureFiles(dir); err != nil {
		t.Fatalf("EnsureFiles failed: %v", err)
	}

	// Verify files were created
	colorsPath := filepath.Join(dir, "colors.toml")
	solPath := filepath.Join(dir, "themes", "solarized-dark.toml")
	catPath := filepath.Join(dir, "themes", "catppuccin.toml")

	for _, path := range []string{colorsPath, solPath, catPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s to exist: %v", path, err)
		}
	}

	// Loading colors loads from colors.toml
	palColors, path := Load(dir, "colors")
	if path != colorsPath || palColors.Accent != Default().Accent {
		t.Fatalf("expected colors palette: %+v at %s", palColors, path)
	}

	// Loading solarized-dark loads from themes/solarized-dark.toml
	palSol, path := Load(dir, "solarized-dark")
	if path != solPath || palSol.Background != SolarizedDark().Background {
		t.Fatalf("expected solarized-dark palette: %+v at %s", palSol, path)
	}

	// Available themes
	avail := Available(dir)
	if len(avail) < 3 || avail[0] != "colors" {
		t.Fatalf("unexpected available themes: %v", avail)
	}

	// EnsureFiles should be idempotent
	if err := EnsureFiles(dir); err != nil {
		t.Fatalf("idempotent EnsureFiles failed: %v", err)
	}

	// Empty dir handling
	if err := EnsureFiles(""); err != nil {
		t.Fatal("EnsureFiles with empty dir failed")
	}
	if p, path := Load("", "colors"); path != "" || p.Accent != Default().Accent {
		t.Fatal("Load with empty dir failed")
	}
	if a := Available(""); len(a) != 3 {
		t.Fatal("Available with empty dir failed")
	}
}

func TestNotifyReload(t *testing.T) {
	ch := make(chan os.Signal, 1)
	NotifyReload(ch)
}
