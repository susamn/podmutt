// Package profile defines configuration, private paths, and TUI ownership.
// CLI clients resolve paths without preparing directories or acquiring ownership.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RefreshMinutes   int    `toml:"refresh_minutes"`
	MPVPath          string `toml:"mpv_path"`
	Telemetry        bool   `toml:"telemetry"`
	Icons            string `toml:"icons"`
	Theme            string `toml:"theme"`
	PlayerDecoration bool   `toml:"player_decoration"`
	Country          string `toml:"country"`
}

func Defaults() Config {
	return Config{RefreshMinutes: 30, MPVPath: "mpv", Icons: "unicode", Theme: "colors", Country: DetectCountry(os.Getenv)}
}

// Load applies a TOML file over defaults. An absent file uses defaults.
func Load(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("configuration: %w", err)
	}
	if len(md.Undecoded()) != 0 {
		return Config{}, fmt.Errorf("configuration: unknown key %s", md.Undecoded()[0])
	}
	if cfg.RefreshMinutes < 1 || cfg.MPVPath == "" {
		return Config{}, fmt.Errorf("configuration: refresh_minutes must be positive and mpv_path nonempty")
	}
	if cfg.Icons != "unicode" && cfg.Icons != "nerd" && cfg.Icons != "plain" {
		return Config{}, fmt.Errorf("configuration: icons must be unicode, nerd or plain")
	}
	if cfg.Theme == "" {
		cfg.Theme = "colors"
	}
	if filepath.Base(cfg.Theme) != cfg.Theme {
		return Config{}, fmt.Errorf("configuration: theme must be a simple name")
	}
	if cfg.Country == "" {
		cfg.Country = DetectCountry(os.Getenv)
	} else {
		cfg.Country = strings.ToLower(strings.TrimSpace(cfg.Country))
		if len(cfg.Country) != 2 || !isAlpha(cfg.Country) {
			return Config{}, fmt.Errorf("configuration: country must be a 2-letter ISO code")
		}
	}
	if cfg.Telemetry {
		return Config{}, fmt.Errorf("configuration: telemetry is not supported")
	}
	return cfg, nil
}

// SetTheme updates or appends the theme setting in config.toml.
func SetTheme(path, themeName string) error {
	if filepath.Base(themeName) != themeName {
		return fmt.Errorf("configuration: theme must be a simple name")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		content := fmt.Sprintf("theme = %q\n", themeName)
		return os.WriteFile(path, []byte(content), 0600)
	}
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "theme") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "theme" {
				lines[i] = fmt.Sprintf("theme = %q", themeName)
				found = true
				break
			}
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("theme = %q", themeName))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

// DetectCountry infers a 2-letter country code from locale environment variables.
func DetectCountry(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		val := getenv(env)
		if val == "" {
			continue
		}
		part := strings.Split(val, ".")[0]
		sub := strings.Split(part, "_")
		if len(sub) == 2 && len(sub[1]) == 2 && isAlpha(strings.ToLower(sub[1])) {
			return strings.ToLower(sub[1])
		}
		if len(sub) == 1 && len(sub[0]) == 2 && isAlpha(strings.ToLower(sub[0])) {
			return strings.ToLower(sub[0])
		}
	}
	return "us"
}

func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// SetCountry updates or appends the country setting in config.toml.
func SetCountry(path, country string) error {
	country = strings.ToLower(strings.TrimSpace(country))
	if len(country) != 2 || !isAlpha(country) {
		return fmt.Errorf("configuration: country must be a 2-letter ISO code")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		content := fmt.Sprintf("country = %q\n", country)
		return os.WriteFile(path, []byte(content), 0600)
	}
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "country") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "country" {
				lines[i] = fmt.Sprintf("country = %q", country)
				found = true
				break
			}
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = fmt.Sprintf("country = %q", country)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("country = %q", country))
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

type Paths struct{ Config, State, Runtime string }

func (p Paths) ConfigDir() string {
	if p.Config == "" {
		return ""
	}
	return filepath.Dir(p.Config)
}

// Onboarded reports whether the first-run welcome dialog was dismissed.
func (p Paths) Onboarded() bool {
	if p.State == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(p.State, "onboarded"))
	return err == nil
}

// SetOnboarded records that the first-run welcome dialog was dismissed.
func (p Paths) SetOnboarded() error {
	if p.State == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(p.State, "onboarded"), []byte("1\n"), 0600)
}

// Resolve selects one default user profile. Relative XDG values are ignored.
// Supplying the platform and environment explicitly also makes path policy testable.
func Resolve(platform, home string, getenv func(string) string) (Paths, error) {
	if !filepath.IsAbs(home) {
		return Paths{}, fmt.Errorf("profile: home must be absolute")
	}
	switch platform {
	case "linux":
		base := func(key, fallback string) string {
			if p := getenv(key); filepath.IsAbs(p) {
				return p
			}
			return fallback
		}
		state := filepath.Join(base("XDG_STATE_HOME", filepath.Join(home, ".local", "state")), "podmutt")
		runtime := filepath.Join(state, "run")
		if base := getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(base) {
			runtime = filepath.Join(base, "podmutt")
		}
		return Paths{filepath.Join(base("XDG_CONFIG_HOME", filepath.Join(home, ".config")), "podmutt", "config.toml"), state, runtime}, nil
	case "darwin":
		state := filepath.Join(home, "Library", "Application Support", "podmutt")
		return Paths{filepath.Join(state, "config.toml"), state, filepath.Join(state, "run")}, nil
	default:
		return Paths{}, fmt.Errorf("profile: unsupported platform %q", platform)
	}
}

// Prepare creates private state/runtime directories. Existing symlinks and
// directories accessible to other users are rejected instead of silently changed.
func (p Paths) Prepare() error {
	for _, path := range []string{p.State, p.Runtime} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("profile: directory must be absolute: %s", path)
		}
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
		if err := privateDirectory(path); err != nil {
			return err
		}
	}
	return nil
}

// CheckRuntime validates an existing runtime directory without creating it.
func (p Paths) CheckRuntime() error { return privateDirectory(p.Runtime) }

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("profile: directory must be owned by current user and private (0700): %s", path)
	}
	return nil
}
