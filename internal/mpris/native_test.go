package mpris

import (
	"errors"
	"testing"
)

func TestNativePlatformAndSessionFallback(t *testing.T) {
	getenv := func(string) string { return "" }
	if s, err := startOn(background, "darwin", getenv, nil); s != nil || err != nil {
		t.Fatal("non-Linux MPRIS activated")
	}
	if _, err := startOn(background, "linux", getenv, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := startOn(background, "linux", func(key string) string {
		if key == "XDG_RUNTIME_DIR" {
			return t.TempDir()
		}
		return ""
	}, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := startOn(background, "linux", func(string) string { return "autolaunch:" }, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
