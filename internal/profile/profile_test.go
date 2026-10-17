package profile

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	p := Paths{State: filepath.Join(root, "state"), Runtime: filepath.Join(root, "runtime")}
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPrepare(t *testing.T) {
	p := testPaths(t)
	if err := p.Prepare(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{p.State, p.Runtime} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("%v %v", info, err)
		}
	}
	if err := os.Chmod(p.Runtime, 0755); err != nil {
		t.Fatal(err)
	}
	if err := p.Prepare(); err == nil {
		t.Fatal("public directory accepted")
	}
	if err := (Paths{State: "relative"}).Prepare(); err == nil {
		t.Fatal("relative path accepted")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := (Paths{State: file}).Prepare(); err == nil {
		t.Fatal("file accepted")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(p.State, link); err != nil {
		t.Fatal(err)
	}
	if err := (Paths{State: link}).Prepare(); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestConcurrentOwnership(t *testing.T) {
	p := testPaths(t)
	var wg sync.WaitGroup
	locks := make(chan *Lock, 16)
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			l, err := p.Acquire()
			if err != nil {
				errs <- err
			} else {
				locks <- l
			}
		})
	}
	wg.Wait()
	close(locks)
	close(errs)
	if len(locks) != 1 || len(errs) != 15 {
		t.Fatalf("owners=%d errors=%d", len(locks), len(errs))
	}
	for err := range errs {
		if !errors.Is(err, ErrAlreadyRunning) {
			t.Fatal(err)
		}
	}
	for lock := range locks {
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := p.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	// Loading configuration and resolving CLI paths never takes TUI ownership.
	lock, err = p.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := Load(filepath.Join(p.State, "missing.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestLockFileValidation(t *testing.T) {
	p := testPaths(t)
	path := filepath.Join(p.Runtime, "tui.lock")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Acquire(); err == nil {
		t.Fatal("public lock accepted")
	}
	q := testPaths(t)
	if err := os.Symlink(path, filepath.Join(q.Runtime, "tui.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Acquire(); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := (Paths{Runtime: filepath.Join(t.TempDir(), "missing")}).Acquire(); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestCrashRecovery(t *testing.T) {
	if path := os.Getenv("PODMUTT_LOCK_HELPER"); path != "" {
		lock, err := (Paths{Runtime: path}).Acquire()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout.WriteString("locked\n")
		var b [1]byte
		os.Stdin.Read(b[:])
		runtime.KeepAlive(lock)
		os.Exit(0)
	}
	p := testPaths(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCrashRecovery$")
	cmd.Env = append(os.Environ(), "PODMUTT_LOCK_HELPER="+p.Runtime)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper: %q %v", line, err)
	}
	if _, err := p.Acquire(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second process: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	lock, err := p.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
}
