// Package app owns the foreground session and releases profile ownership after
// the terminal program exits, including initialization and rendering failures.
package app

import (
	"errors"
	"io"

	"podmutt/internal/profile"
	"podmutt/internal/theme"
)

var Version = "dev"

func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

func Run(input io.Reader, output io.Writer) error {
	return withProfile(func(cfg profile.Config, paths profile.Paths) error {
		return withSession(cfg, paths, func(s *session) error {
			return runLibrary(s, input, output)
		})
	})
}

func withProfile(run func(profile.Config, profile.Paths) error) (err error) {
	paths, err := userPaths()
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
	_ = theme.EnsureFiles(paths.ConfigDir())
	return run(cfg, paths)
}
