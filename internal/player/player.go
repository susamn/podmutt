// Package player controls an mpv child through a private inherited socketpair.
// Closing the IPC connection makes mpv exit, including when the parent crashes.
package player

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"podmutt/internal/feedback"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrUnavailable = feedback.New(feedback.PlayerMissing, "mpv is unavailable: install mpv using your package manager (macOS: brew install mpv), or set mpv_path")

const shutdownTimeout = 3 * time.Second

var arguments = []string{"--idle=yes", "--no-video", "--no-terminal", "--no-config",
	"--load-scripts=no", "--ytdl=no", "--input-ipc-client=fd://3"}

// The foreground owns macOS commands; mpv must not register a competing remote
// command center. This platform-specific option is not assumed to exist on Linux.
func platformArguments(goos string) []string {
	args := append([]string(nil), arguments...)
	if goos == "darwin" {
		args = append(args, "--input-media-keys=no")
	}
	return args
}

type Player struct {
	*ipc
	cmd       *exec.Cmd
	exited    chan struct{}
	waitErr   error // published by closing exited
	closeOnce sync.Once
	closeErr  error
}

// Start owns the child's lifetime until Close or ctx cancellation. No user mpv
// config/scripts are loaded. Stdout/stderr are discarded, avoiding URL leakage.
func Start(ctx context.Context, path string) (*Player, error) {
	binary, err := exec.LookPath(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	cmd := exec.CommandContext(ctx, binary, platformArguments(runtime.GOOS)...)
	cmd.Env = mediaEnvironment(os.Environ())
	p, err := launch(ctx, cmd, shutdownTimeout)
	if err != nil {
		return nil, feedback.Wrap(feedback.PlayerFailure, err)
	}
	return p, nil
}

// The owned external decoder has no reason to receive directory credentials.
func mediaEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "PODMUTT_PODCAST_INDEX_") {
			result = append(result, entry)
		}
	}
	return result
}

func launch(ctx context.Context, cmd *exec.Cmd, timeout time.Duration) (*Player, error) {
	// Prevent another Go exec from inheriting either endpoint before CLOEXEC.
	syscall.ForkLock.RLock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		for _, fd := range fds {
			syscall.CloseOnExec(fd)
		}
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	parent, child := os.NewFile(uintptr(fds[0]), "mpv-parent"), os.NewFile(uintptr(fds[1]), "mpv-child")
	defer parent.Close()
	defer child.Close()
	conn, err := net.FileConn(parent)
	if err != nil {
		return nil, err
	}
	cmd.ExtraFiles = []*os.File{child} // child receives fd 3; no filesystem socket
	if err := cmd.Start(); err != nil {
		conn.Close()
		return nil, err
	}
	// Close both local originals before handshaking: only conn and the child may
	// own endpoints, otherwise EOF on crash/shutdown could be held open.
	parent.Close()
	child.Close()
	p := &Player{ipc: newIPC(conn, timeout), cmd: cmd, exited: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		conn.Close()
		close(p.exited)
	}()
	if _, err := p.command(ctx, []any{"get_property", "mpv-version"}); err != nil {
		p.Close()
		return nil, err
	}
	for i, property := range []string{"time-pos", "pause", "duration"} {
		if _, err := p.command(ctx, []any{"observe_property", i + 1, property}); err != nil {
			p.Close()
			return nil, err
		}
	}
	return p, nil
}

// Events includes observed properties and end-file events. Only reason=eof is
// natural completion; stop, quit, redirect, and error must remain distinct.
// Consume promptly. Overflow disconnects rather than silently losing EOF.
func (p *Player) Events() <-chan Event { return p.events }

func (p *Player) Load(ctx context.Context, mediaURL string, seconds float64) error {
	if strings.TrimSpace(mediaURL) == "" || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("mpv: media URL and finite nonnegative start position required")
	}
	// Named arguments avoid the loadfile positional-argument change in mpv 0.38.
	_, err := p.command(ctx, map[string]any{"name": "loadfile", "url": mediaURL, "flags": "replace",
		"options": map[string]string{"start": strconv.FormatFloat(seconds, 'f', -1, 64), "pause": "no"}})
	return err
}

func (p *Player) setPaused(ctx context.Context, paused bool) error {
	_, err := p.command(ctx, []any{"set_property", "pause", paused})
	return err
}

func (p *Player) Play(ctx context.Context) error  { return p.setPaused(ctx, false) }
func (p *Player) Pause(ctx context.Context) error { return p.setPaused(ctx, true) }
func (p *Player) Stop(ctx context.Context) error {
	_, err := p.command(ctx, []any{"stop"})
	return err
}

// EntryID identifies the replacement playlist item even before loading finishes.
// End-file events carry this ID, allowing callers to reject stale completions.
func (p *Player) EntryID(ctx context.Context) (int64, error) {
	data, err := p.command(ctx, []any{"get_property", "playlist/0/id"})
	if err != nil {
		return 0, err
	}
	var id int64
	if err := json.Unmarshal(data, &id); err != nil || id < 0 || string(data) == "null" {
		return 0, fmt.Errorf("mpv: invalid playlist entry ID")
	}
	return id, nil
}

// SeekTo uses an absolute exact seek without changing pause state.
func (p *Player) SeekTo(ctx context.Context, seconds float64) error {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("mpv: finite nonnegative seek position required")
	}
	_, err := p.command(ctx, []any{"seek", seconds, "absolute+exact"})
	return err
}

// ChapterList reads embedded media/container chapters for the loaded file.
// A missing property is treated as no chapters, not a playback failure.
func (p *Player) ChapterList(ctx context.Context) ([]byte, error) {
	data, err := p.command(ctx, []any{"get_property", "chapter-list"})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Position returns an error while no file is loaded, rather than inventing zero.
func (p *Player) Position(ctx context.Context) (float64, error) {
	data, err := p.command(ctx, []any{"get_property", "time-pos"})
	if err != nil {
		return 0, err
	}
	position, err := strconv.ParseFloat(string(data), 64)
	if err != nil || position < 0 || math.IsNaN(position) || math.IsInf(position, 0) {
		return 0, fmt.Errorf("mpv: invalid playback position")
	}
	return position, nil
}

// Close disconnects, waits for mpv, and kills an unresponsive child after the
// command timeout. Repeated/concurrent calls are safe and return the same result.
func (p *Player) Close() error {
	p.closeOnce.Do(func() {
		p.fail(ErrDisconnected)
		select {
		case <-p.exited:
			p.closeErr = p.waitErr
		case <-time.After(p.timeout):
			p.cmd.Process.Kill()
			p.closeErr = fmt.Errorf("mpv: shutdown timed out; child killed")
			select {
			case <-p.exited:
			case <-time.After(p.timeout):
			}
		}
	})
	return p.closeErr
}
