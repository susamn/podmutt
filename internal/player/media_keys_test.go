package player

import (
	"slices"
	"testing"
)

func TestForegroundOwnsMacMediaKeysWithoutChangingLinuxArguments(t *testing.T) {
	mac := platformArguments("darwin")
	linux := platformArguments("linux")
	if !slices.Contains(mac, "--input-media-keys=no") || slices.Contains(linux, "--input-media-keys=no") {
		t.Fatal(mac, linux)
	}
	mac[0] = "mutated"
	if arguments[0] == "mutated" || linux[0] == "mutated" {
		t.Fatal("shared mutable argument slices")
	}
}
