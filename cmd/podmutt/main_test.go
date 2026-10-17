package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{name: "version", args: []string{"--version"}, stdout: "podmutt dev\n"},
		{name: "help", args: []string{"--help"}, stderr: "Usage: podmutt"},
		{name: "unknown flag", args: []string{"--unknown"}, code: 2, stderr: "flag provided but not defined"},
		{name: "unexpected argument", args: []string{"play"}, code: 2, stderr: "unexpected arguments"},
		{name: "version with argument", args: []string{"--version", "extra"}, code: 2, stderr: "unexpected arguments"},
		{name: "not implemented", code: 1, stderr: "TUI not implemented yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.code {
				t.Fatalf("exit code = %d, want %d", got, tt.code)
			}
			if got := stdout.String(); got != tt.stdout {
				t.Errorf("stdout = %q, want %q", got, tt.stdout)
			}
			if got := stderr.String(); (tt.stderr == "" && got != "") || !strings.Contains(got, tt.stderr) {
				t.Errorf("stderr = %q, want containing %q", got, tt.stderr)
			}
		})
	}
}
