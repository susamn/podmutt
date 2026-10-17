//go:build linux || darwin

package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrAlreadyRunning = errors.New("podmutt: already running for this profile")

// Lock holds ownership until Close or process exit. Never unlink its file:
// replacing a locked inode would allow two owners of the same profile.
type Lock struct{ file *os.File }

// Acquire is for the foreground TUI only, after Prepare. CLI clients must never
// call it; they contact the running TUI through its private control endpoint.
func (p Paths) Acquire() (*Lock, error) {
	if err := privateDirectory(p.Runtime); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(filepath.Join(p.Runtime, "tui.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "tui.lock")
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		file.Close()
		return nil, fmt.Errorf("profile: lock must be a private regular file")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return &Lock{file: file}, nil
}

func (l *Lock) Close() error { return l.file.Close() }
