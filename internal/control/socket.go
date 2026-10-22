package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"podmutt/internal/feedback"
	"sync"
	"syscall"

	"podmutt/internal/profile"
)

func endpoint(paths profile.Paths) string { return filepath.Join(paths.Runtime, "control.sock") }
func privateSocket(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, feedback.New(feedback.Profile, "control endpoint must be a private socket owned by current user")
	}
	return info, nil
}

type Server struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

// Listen requires the caller to hold the profile's TUI lock for its lifetime.
// It removes only a verified stale socket, never files, symlinks or live servers.
func Listen(ctx context.Context, paths profile.Paths, actions Actions) (*Server, error) {
	if err := paths.CheckRuntime(); err != nil {
		return nil, err
	}
	path := endpoint(paths)
	if _, err := privateSocket(path); err == nil {
		probe, err := net.DialTimeout("unix", path, timeout)
		if err == nil {
			probe.Close()
			return nil, feedback.New(feedback.AlreadyRunning, "control endpoint already active")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return nil, err
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0600); err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	info, err := privateSocket(path)
	if err != nil {
		listener.Close()
		os.Remove(path)
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	server := &Server{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(server.done)
		stop := context.AfterFunc(lifetime, func() { listener.Close() })
		defer stop()
		var workers sync.WaitGroup
		slots := make(chan struct{}, 16)
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				if lifetime.Err() == nil {
					server.err = err
				}
				break
			}
			select {
			case slots <- struct{}{}:
				workers.Go(func() { defer func() { <-slots }(); serve(lifetime, conn, actions) })
			default:
				conn.Close()
			}
		}
		cancel()
		listener.Close()
		workers.Wait()
		if current, err := os.Lstat(path); err == nil && os.SameFile(info, current) {
			server.err = errors.Join(server.err, os.Remove(path))
		}
	}()
	return server, nil
}
func (s *Server) Close() error { s.cancel(); <-s.done; return s.err }

func serve(parent context.Context, conn net.Conn, actions Actions) {
	defer conn.Close()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	var request request
	err := readFrame(conn, maxRequest, &request)
	var result Status
	if err == nil {
		result, err = Execute(ctx, actions, request.Command)
	}
	reply := response{State: &result}
	if err != nil {
		reply = response{Error: encodeError(err)}
	}
	if err := writeFrame(conn, maxResponse, reply); err != nil {
		writeFrame(conn, maxResponse, response{Error: "control response could not be encoded"})
	}
}

// Call never creates profile directories, opens SQLite, acquires ownership, or
// starts a player. Missing/refused endpoints are reported as ErrNotRunning.
func Call(ctx context.Context, paths profile.Paths, command string) (Status, error) {
	if !Valid(command) {
		return Status{}, fmt.Errorf("unknown control command")
	}
	if err := paths.CheckRuntime(); err != nil {
		if os.IsNotExist(err) {
			return Status{}, ErrNotRunning
		}
		return Status{}, feedback.Wrap(feedback.Profile, err)
	}
	if _, err := privateSocket(endpoint(paths)); err != nil {
		if os.IsNotExist(err) {
			err = ErrNotRunning
		}
		return Status{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", endpoint(paths))
	if err != nil {
		if ctx.Err() != nil {
			return Status{}, ctx.Err()
		}
		if errors.Is(err, syscall.ECONNREFUSED) || os.IsNotExist(err) {
			err = ErrNotRunning
		}
		return Status{}, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	if err := writeFrame(conn, maxRequest, request{command}); err != nil {
		if ctx.Err() != nil {
			return Status{}, ctx.Err()
		}
		return Status{}, feedback.Wrap(feedback.Protocol, err)
	}
	var reply response
	if err := readFrame(conn, maxResponse, &reply); err != nil {
		if ctx.Err() != nil {
			return Status{}, ctx.Err()
		}
		return Status{}, feedback.Wrap(feedback.Protocol, err)
	}
	if reply.Error != "" {
		return Status{}, decodeError(reply.Error)
	}
	if reply.State == nil {
		return Status{}, feedback.New(feedback.Protocol, "missing control state")
	}
	return *reply.State, nil
}
