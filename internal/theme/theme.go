// Package theme resolves the color palette podmutt renders with, matching
// mpdtui's colors.toml structure, preset themes, and live SIGUSR1 reloads.
package theme

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Color is a "#RRGGBB" hex color string.
type Color string

// Palette is the semantic color set podmutt derives its widget styling from.
// Field names mirror mpdtui's colors.toml.
type Palette struct {
	Mode string

	Accent    Color
	Selection Color
	Muted     Color

	Background        Color
	DarkBackground    Color
	DarkerBackground  Color
	LighterBackground Color

	Foreground       Color
	DarkForeground   Color
	LightForeground  Color
	BrightForeground Color

	Red, Yellow, Orange, Green, Cyan, Blue, Magenta, Brown Color

	BrightRed, BrightYellow, BrightGreen, BrightCyan, BrightBlue, BrightMagenta Color
}

// Default is mpdtui's built-in default dark palette.
func Default() Palette {
	return Palette{
		Mode:              "dark",
		Accent:            "#00FF00",
		Selection:         "#0000FF",
		Muted:             "#808080",
		Background:        "#000000",
		DarkBackground:    "#000000",
		DarkerBackground:  "#000000",
		LighterBackground: "#FFFFFF",
		Foreground:        "#FFFFFF",
		DarkForeground:    "#808080",
		LightForeground:   "#D3D3D3",
		BrightForeground:  "#FFFFFF",
		Red:               "#DC3545",
		Yellow:            "#FFD700",
		Orange:            "#FFA500",
		Green:             "#25D366",
		Cyan:              "#00E5E5",
		Blue:              "#87CEEB",
		Magenta:           "#DDA0DD",
		Brown:             "#935E6D",
		BrightRed:         "#FF0000",
		BrightYellow:      "#FFFF00",
		BrightGreen:       "#00FF00",
		BrightCyan:        "#00E5E5",
		BrightBlue:        "#87CEEB",
		BrightMagenta:     "#DDA0DD",
	}
}

// SolarizedDark returns the canonical Solarized Dark palette.
func SolarizedDark() Palette {
	return Palette{
		Mode:              "dark",
		Accent:            "#268bd2",
		Selection:         "#073642",
		Muted:             "#586e75",
		Background:        "#002b36",
		DarkBackground:    "#00212b",
		DarkerBackground:  "#00171f",
		LighterBackground: "#073642",
		Foreground:        "#839496",
		DarkForeground:    "#657b83",
		LightForeground:   "#93a1a1",
		BrightForeground:  "#fdf6e3",
		Red:               "#dc322f",
		Yellow:            "#b58900",
		Orange:            "#cb4b16",
		Green:             "#859900",
		Cyan:              "#2aa198",
		Blue:              "#268bd2",
		Magenta:           "#d33682",
		Brown:             "#6c71c4",
		BrightRed:         "#e24845",
		BrightYellow:      "#c99900",
		BrightGreen:       "#97ad00",
		BrightCyan:        "#36c1b5",
		BrightBlue:        "#3ea3eb",
		BrightMagenta:     "#de4e96",
	}
}

// Catppuccin returns the Catppuccin Mocha dark palette.
func Catppuccin() Palette {
	return Palette{
		Mode:              "dark",
		Accent:            "#a6e3a1",
		Selection:         "#45475a",
		Muted:             "#585b70",
		Background:        "#1e1e2e",
		DarkBackground:    "#181825",
		DarkerBackground:  "#11111b",
		LighterBackground: "#313244",
		Foreground:        "#cdd6f4",
		DarkForeground:    "#a6adc8",
		LightForeground:   "#bac2de",
		BrightForeground:  "#ffffff",
		Red:               "#f38ba8",
		Yellow:            "#f9e2af",
		Orange:            "#fab387",
		Green:             "#a6e3a1",
		Cyan:              "#89dceb",
		Blue:              "#89b4fa",
		Magenta:           "#cba6f7",
		Brown:             "#f2cdcd",
		BrightRed:         "#f38ba8",
		BrightYellow:      "#f9e2af",
		BrightGreen:       "#a6e3a1",
		BrightCyan:        "#89dceb",
		BrightBlue:        "#89b4fa",
		BrightMagenta:     "#cba6f7",
	}
}

// ChannelColor returns a consistent color from the palette for the given channel name.
func (p Palette) ChannelColor(name string) Color {
	palette := []Color{p.Cyan, p.Blue, p.Magenta, p.Yellow, p.Green, p.Orange}
	if len(palette) == 0 {
		return p.Cyan
	}
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return palette[int(h)%len(palette)]
}

// Serialize encodes a Palette into flat key = "value" text with descriptive comments.
func Serialize(p Palette) string {
	var b strings.Builder
	writeField := func(comment, key string, value Color) {
		if comment != "" {
			fmt.Fprintf(&b, "# %s\n", comment)
		}
		fmt.Fprintf(&b, "%s = %q\n\n", key, value)
	}

	fmt.Fprintf(&b, "mode = %q\n\n", p.Mode)

	writeField("Accent color -- focused panel borders, selected-row highlight.", "accent", p.Accent)
	writeField("Selected-row background.", "selection", p.Selection)
	writeField("Header rows, secondary borders.", "muted", p.Muted)

	writeField("Base background.", "background", p.Background)
	writeField("A darker background shade.", "dark_background", p.DarkBackground)
	writeField("The darkest background shade.", "darker_background", p.DarkerBackground)
	writeField("A lighter background shade.", "lighter_background", p.LighterBackground)

	writeField("Base foreground/text color.", "foreground", p.Foreground)
	writeField("A dimmer foreground shade.", "dark_foreground", p.DarkForeground)
	writeField("A lighter foreground shade.", "light_foreground", p.LightForeground)
	writeField("The brightest foreground shade.", "bright_foreground", p.BrightForeground)

	writeField("Categorical colors.", "red", p.Red)
	writeField("", "yellow", p.Yellow)
	writeField("", "orange", p.Orange)
	writeField("", "green", p.Green)
	writeField("", "cyan", p.Cyan)
	writeField("", "blue", p.Blue)
	writeField("", "magenta", p.Magenta)
	writeField("", "brown", p.Brown)

	writeField("Brighter variants.", "bright_red", p.BrightRed)
	writeField("", "bright_yellow", p.BrightYellow)
	writeField("", "bright_green", p.BrightGreen)
	writeField("", "bright_cyan", p.BrightCyan)
	writeField("", "bright_blue", p.BrightBlue)
	fmt.Fprintf(&b, "bright_magenta = %q\n", p.BrightMagenta)

	return b.String()
}

// Parse extracts key = "value" pairs, merging onto Default().
func Parse(data []byte) Palette {
	p := Default()
	scanner := bufio.NewScanner(bytes.NewReader(data))
	fields := make(map[string]string)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if strings.HasPrefix(val, `"`) {
			val = strings.TrimPrefix(val, `"`)
			if idx := strings.Index(val, `"`); idx >= 0 {
				val = val[:idx]
			}
		} else if strings.HasPrefix(val, `'`) {
			val = strings.TrimPrefix(val, `'`)
			if idx := strings.Index(val, `'`); idx >= 0 {
				val = val[:idx]
			}
		} else {
			if idx := strings.Index(val, " #"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
			} else if idx := strings.Index(val, "\t#"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
			}
		}
		fields[key] = val
	}
	set := func(dst *Color, key string) {
		if v, ok := fields[key]; ok && v != "" {
			*dst = Color(v)
		}
	}
	if v, ok := fields["mode"]; ok && v != "" {
		p.Mode = v
	}
	set(&p.Accent, "accent")
	set(&p.Selection, "selection")
	set(&p.Muted, "muted")
	set(&p.Background, "background")
	set(&p.DarkBackground, "dark_background")
	set(&p.DarkerBackground, "darker_background")
	set(&p.LighterBackground, "lighter_background")
	set(&p.Foreground, "foreground")
	set(&p.DarkForeground, "dark_foreground")
	set(&p.LightForeground, "light_foreground")
	set(&p.BrightForeground, "bright_foreground")
	set(&p.Red, "red")
	set(&p.Yellow, "yellow")
	set(&p.Orange, "orange")
	set(&p.Green, "green")
	set(&p.Cyan, "cyan")
	set(&p.Blue, "blue")
	set(&p.Magenta, "magenta")
	set(&p.Brown, "brown")
	set(&p.BrightRed, "bright_red")
	set(&p.BrightYellow, "bright_yellow")
	set(&p.BrightGreen, "bright_green")
	set(&p.BrightCyan, "bright_cyan")
	set(&p.BrightBlue, "bright_blue")
	set(&p.BrightMagenta, "bright_magenta")
	return p
}

// Preset returns a known built-in palette by name, or Default() if unrecognized.
func Preset(name string) Palette {
	switch name {
	case "solarized-dark":
		return SolarizedDark()
	case "catppuccin":
		return Catppuccin()
	default:
		return Default()
	}
}

// FilePath resolves the expected path for a theme in configDir.
func FilePath(configDir, name string) string {
	if configDir == "" {
		return ""
	}
	if name == "" || name == "colors" {
		return filepath.Join(configDir, "colors.toml")
	}
	return filepath.Join(configDir, "themes", name+".toml")
}

// Load loads the requested theme from configDir, falling back to preset or default.
func Load(configDir, name string) (Palette, string) {
	if name == "" {
		name = "colors"
	}
	path := FilePath(configDir, name)
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return Parse(data), path
		}
	}
	return Preset(name), path
}

// EnsureFiles seeds colors.toml and theme presets into configDir if missing.
func EnsureFiles(configDir string) error {
	if configDir == "" {
		return nil
	}
	themesDir := filepath.Join(configDir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return err
	}
	colorsPath := filepath.Join(configDir, "colors.toml")
	if _, err := os.Stat(colorsPath); os.IsNotExist(err) {
		_ = os.WriteFile(colorsPath, []byte(Serialize(Default())), 0644)
	}
	solPath := filepath.Join(themesDir, "solarized-dark.toml")
	if _, err := os.Stat(solPath); os.IsNotExist(err) {
		_ = os.WriteFile(solPath, []byte(Serialize(SolarizedDark())), 0644)
	}
	catPath := filepath.Join(themesDir, "catppuccin.toml")
	if _, err := os.Stat(catPath); os.IsNotExist(err) {
		_ = os.WriteFile(catPath, []byte(Serialize(Catppuccin())), 0644)
	}
	return nil
}

// Available returns all selectable theme names in configDir.
func Available(configDir string) []string {
	seen := map[string]bool{"colors": true, "solarized-dark": true, "catppuccin": true}
	themes := []string{"colors", "solarized-dark", "catppuccin"}
	if configDir == "" {
		return themes
	}
	entries, err := os.ReadDir(filepath.Join(configDir, "themes"))
	if err != nil {
		return themes
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			name := strings.TrimSuffix(entry.Name(), ".toml")
			if !seen[name] {
				seen[name] = true
				themes = append(themes, name)
			}
		}
	}
	sort.Strings(themes[1:]) // Keep colors first, sort the rest
	return themes
}
