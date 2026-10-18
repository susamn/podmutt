package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"podmutt/internal/profile"
)

func isolatedProfile(t *testing.T) profile.Paths {
	t.Helper()
	home, err := os.MkdirTemp("", "pma-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS"} {
		t.Setenv(name, "")
	}
	paths, err := profile.Resolve(runtime.GOOS, home, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestSessionHoldsAndReleasesOwnership(t *testing.T) {
	paths := isolatedProfile(t)
	sentinel := errors.New("program failure")
	err := withProfile(func(cfg profile.Config, got profile.Paths) error {
		if cfg != profile.Defaults() || got != paths {
			t.Fatalf("session defaults: %+v, %+v", cfg, got)
		}
		if err := withProfile(func(profile.Config, profile.Paths) error {
			t.Fatal("second owner entered session")
			return nil
		}); !errors.Is(err, profile.ErrAlreadyRunning) {
			t.Fatalf("second owner: %v", err)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("program error lost: %v", err)
	}
	if err := withProfile(func(profile.Config, profile.Paths) error { return nil }); err != nil {
		t.Fatalf("ownership not released after failure: %v", err)
	}
}

func TestSessionStartupFailures(t *testing.T) {
	never := func(profile.Config, profile.Paths) error {
		t.Fatal("program started after setup failure")
		return nil
	}
	t.Run("missing home", func(t *testing.T) {
		isolatedProfile(t)
		t.Setenv("HOME", "")
		if err := withProfile(never); err == nil {
			t.Fatal("missing home accepted")
		}
	})
	t.Run("relative home", func(t *testing.T) {
		isolatedProfile(t)
		t.Setenv("HOME", "relative")
		if err := withProfile(never); err == nil {
			t.Fatal("relative home accepted")
		}
	})
	t.Run("invalid TOML", func(t *testing.T) {
		paths := isolatedProfile(t)
		if err := os.MkdirAll(filepath.Dir(paths.Config), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.Config, []byte("refresh_minutes = -1"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := withProfile(never); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	})
	t.Run("public state directory", func(t *testing.T) {
		paths := isolatedProfile(t)
		if err := os.MkdirAll(paths.State, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(paths.State, 0755); err != nil {
			t.Fatal(err)
		}
		if err := withProfile(never); err == nil {
			t.Fatal("public directory accepted")
		}
	})
}

func TestProgramQuitsAndReleasesProfile(t *testing.T) {
	isolatedProfile(t)
	var output bytes.Buffer
	if err := Run(strings.NewReader("q"), &output); err != nil {
		t.Fatal(err)
	}
	if err := withProfile(func(profile.Config, profile.Paths) error { return nil }); err != nil {
		t.Fatalf("program exit retained lock: %v", err)
	}
}
