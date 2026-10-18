// Package profile defines configuration, private paths, and TUI ownership.
// CLI clients resolve paths without preparing directories or acquiring ownership.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/BurntSushi/toml"
)

type Config struct {
	RefreshMinutes int    `toml:"refresh_minutes"`
	MPVPath        string `toml:"mpv_path"`
	Telemetry      bool   `toml:"telemetry"`
	Icons          string `toml:"icons"`
	Theme          string `toml:"theme"`
}

func Defaults() Config {
	return Config{RefreshMinutes: 30, MPVPath: "mpv", Icons: "unicode", Theme: "colors"}
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
	if cfg.Telemetry {
		return Config{}, fmt.Errorf("configuration: telemetry is not supported")
	}
	return cfg, nil
}

type Paths struct{ Config, State, Runtime string }

func (p Paths) ConfigDir() string {
	if p.Config == "" {
		return ""
	}
	return filepath.Dir(p.Config)
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
