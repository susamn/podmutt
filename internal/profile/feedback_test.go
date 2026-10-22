package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"podmutt/internal/feedback"
)

func TestConfigurationErrorsExplainValidInputWithoutEchoingValues(t *testing.T) {
	for _, data := range []string{`theme = "private-secret`, `private_secret_key = "private-secret"`, `theme = "../private-secret"`, `country = "private-secret"`, `mpv_path = 123`} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil {
			t.Fatal("invalid configuration accepted")
		}
		message := feedback.For("Start podmutt", err)
		if message.Code != feedback.Configuration || !strings.Contains(message.Text(), "config.toml") || strings.Contains(message.Text(), "private-secret") || strings.Contains(message.Text(), "private_secret_key") {
			t.Fatal(message)
		}
	}
	_, err := Load(t.TempDir())
	if message := feedback.For("Start podmutt", err); message.Code != feedback.Configuration || !strings.Contains(message.Text(), "readable file") {
		t.Fatal(message)
	}
}
