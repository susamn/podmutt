package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"podmutt/internal/feedback"
	"strings"
	"sync"
	"testing"
	"time"

	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/store"
)

var background = context.Background()
var failed = errors.New("action failed")

type fakeActions struct {
	mu    sync.Mutex
	calls []string
	fail  string
}

func (f *fakeActions) action(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
	if f.fail == name {
		return failed
	}
	return nil
}
func (f *fakeActions) Play(context.Context) error  { return f.action("play") }
func (f *fakeActions) Pause(context.Context) error { return f.action("pause") }
func (f *fakeActions) Stop(context.Context) error  { return f.action("stop") }
func (f *fakeActions) Next(context.Context) error  { return f.action("next") }
func (f *fakeActions) Snapshot(context.Context) (playback.State, error) {
	if err := f.action("status"); err != nil {
		return playback.State{}, err
	}
	item := store.Episode{ID: 1, Title: "Episode", AudioURL: "https://example.test/secret", PositionSeconds: 12}
	return playback.State{Status: playback.Stopped, Current: &item, Queue: []store.Episode{item}}, nil
}
func paths(t *testing.T) profile.Paths {
	t.Helper()
	dir, err := os.MkdirTemp("", "pmc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return profile.Paths{Runtime: dir}
}
func start(t *testing.T, p profile.Paths, actions Actions) *Server {
	t.Helper()
	s, err := Listen(background, p, actions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestCommandsAndConcurrentClients(t *testing.T) {
	p := paths(t)
	f := &fakeActions{}
	start(t, p, f)
	info, err := os.Lstat(endpoint(p))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("endpoint mode: %v, %v", info, err)
	}
	var clients sync.WaitGroup
	for range 3 {
		for _, command := range []string{"play", "pause", "stop", "next", "status"} {
			clients.Go(func() {
				state, err := Call(background, p, command)
				if err != nil {
					t.Error(err)
					return
				}
				if state.Current == nil || state.Current.Position != 12 || len(state.Queue) != 1 {
					t.Errorf("status: %+v", state)
				}
				data, _ := json.Marshal(state)
				if bytes.Contains(data, []byte("secret")) {
					t.Error("status disclosed stream URL")
				}
			})
		}
	}
	clients.Wait()
	if _, err := Call(background, p, "invalid"); err == nil {
		t.Fatal("invalid command accepted")
	}
}

func TestActionAndSnapshotErrors(t *testing.T) {
	for _, command := range []string{"play", "pause", "stop", "next", "status"} {
		p := paths(t)
		start(t, p, &fakeActions{fail: command})
		if _, err := Call(background, p, command); err == nil || feedback.For("Playback command", err).Code != feedback.Unknown {
			t.Fatalf("%s: %v", command, err)
		}
	}
	if _, err := Execute(background, &fakeActions{}, "invalid"); err == nil {
		t.Fatal("invalid action accepted")
	}
}

func TestInvalidRequests(t *testing.T) {
	p := paths(t)
	start(t, p, &fakeActions{})
	for _, frame := range []string{"not-json\n", "{}\n", "{\"command\":\"status\",\"extra\":true}\n", "{\"command\":\"status\"} {}\n", strings.Repeat("x", maxRequest) + "\n"} {
		conn, err := net.Dial("unix", endpoint(p))
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(time.Second))
		conn.Write([]byte(frame))
		var reply response
		if err := readFrame(conn, maxResponse, &reply); err != nil || reply.Error == "" {
			t.Fatalf("invalid request: %+v, %v", reply, err)
		}
		conn.Close()
	}
}

func TestStaleActiveAndUnsafeEndpoints(t *testing.T) {
	p := paths(t)
	if _, err := Call(background, p, "status"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("absent endpoint: %v", err)
	}
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint(p), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	os.Chmod(endpoint(p), 0600)
	stale.Close()
	if _, err := Call(background, p, "status"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stale endpoint: %v", err)
	}
	s := start(t, p, &fakeActions{})
	if _, err := Listen(background, p, &fakeActions{}); err == nil {
		t.Fatal("active endpoint replaced")
	}
	s.Close()
	if _, err := os.Lstat(endpoint(p)); !os.IsNotExist(err) {
		t.Fatal("endpoint not removed")
	}
	for _, kind := range []string{"file", "symlink", "public-socket"} {
		q := paths(t)
		path := endpoint(q)
		switch kind {
		case "file":
			os.WriteFile(path, nil, 0600)
		case "symlink":
			os.Symlink(filepath.Join(q.Runtime, "other"), path)
		case "public-socket":
			l, _ := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			l.SetUnlinkOnClose(false)
			os.Chmod(path, 0666)
			l.Close()
		}
		if _, err := Listen(background, q, &fakeActions{}); err == nil {
			t.Fatalf("accepted %s", kind)
		}
		if _, err := Call(background, q, "status"); err == nil {
			t.Fatalf("connected to %s", kind)
		}
	}
	os.Chmod(p.Runtime, 0755)
	if _, err := Listen(background, p, &fakeActions{}); err == nil {
		t.Fatal("public runtime accepted")
	}
	if _, err := Call(background, p, "status"); err == nil {
		t.Fatal("public runtime used")
	}
	missing := profile.Paths{Runtime: filepath.Join(paths(t).Runtime, "absent")}
	if _, err := Call(background, missing, "status"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("missing runtime: %v", err)
	}
	if _, err := os.Stat(missing.Runtime); !os.IsNotExist(err) {
		t.Fatal("client created runtime")
	}
}

func TestCloseCancelsBlockedClientsAndPreservesReplacement(t *testing.T) {
	p := paths(t)
	s := start(t, p, &fakeActions{})
	conn, err := net.Dial("unix", endpoint(p))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := os.Remove(endpoint(p)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(endpoint(p), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on client")
	}
	data, err := os.ReadFile(endpoint(p))
	if err != nil || string(data) != "replacement" {
		t.Fatal("replacement was deleted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFramesAndCanceledClient(t *testing.T) {
	for _, raw := range []string{"{}", "{} {}\n", "{bad}\n"} {
		if err := readFrame(strings.NewReader(raw), maxResponse, new(response)); err == nil {
			t.Fatal("invalid frame accepted")
		}
	}
	if err := writeFrame(new(bytes.Buffer), 2, response{Error: "long"}); err == nil {
		t.Fatal("oversized output accepted")
	}
	p := paths(t)
	start(t, p, &fakeActions{})
	ctx, cancel := context.WithCancel(background)
	cancel()
	if _, err := Call(ctx, p, "status"); err == nil {
		t.Fatal("canceled request succeeded")
	}
}
