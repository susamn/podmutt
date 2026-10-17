// Package app owns the foreground session and releases profile ownership after
// the terminal program exits, including initialization and rendering failures.
package app

import (
	"errors"
	"io"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/profile"
	"podmutt/internal/ui"
)

func Run(input io.Reader, output io.Writer) error {
	return withProfile(func(_ profile.Config, _ profile.Paths) error {
		_, err := tea.NewProgram(ui.New(ui.Data{}), tea.WithInput(input), tea.WithOutput(output)).Run()
		return err
	})
}

func withProfile(run func(profile.Config, profile.Paths) error) (err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	paths, err := profile.Resolve(runtime.GOOS, home, os.Getenv)
	if err != nil {
		return err
	}
	cfg, err := profile.Load(paths.Config)
	if err != nil {
		return err
	}
	if err := paths.Prepare(); err != nil {
		return err
	}
	owner, err := paths.Acquire()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owner.Close()) }()
	return run(cfg, paths)
}
