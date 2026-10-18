package player

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func integrationBinary(t *testing.T) string {
	t.Helper()
	path := os.Getenv("PODMUTT_TEST_MPV")
	if path == "" {
		t.Skip("set PODMUTT_TEST_MPV or run make mpv-test for real-mpv checks")
	}
	binary, err := exec.LookPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return binary
}

func awaitEvent(t *testing.T, p *Player, name, reason string) Event {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-p.Events():
			if !ok {
				t.Fatalf("mpv disconnected: %v", p.Err())
			}
			if event.Name == name && (reason == "" || event.Reason == reason) {
				return event
			}
		case <-timer.C:
			t.Fatalf("no %s/%s event from mpv", name, reason)
		}
	}
}

func silenceWAV(t *testing.T, seconds int) string {
	t.Helper()
	const rate = 8000
	data := make([]byte, 44+seconds*rate*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(data)-44))
	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRealMPVControlsAndEndReasons(t *testing.T) {
	binary := integrationBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append(append([]string{}, arguments...), "--ao=null")
	p, err := launch(ctx, exec.CommandContext(ctx, binary, args...), shutdownTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Load(ctx, silenceWAV(t, 5), 1.25); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, p, "file-loaded", "")
	entryID, err := p.EntryID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	// file-loaded precedes playback initialization; wait for a numeric position.
	deadline := time.Now().Add(2 * time.Second)
	for {
		position, err := p.Position(ctx)
		if err == nil && position >= 1.2 && position < 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("resume position = %v, %v", position, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, target := range []float64{3, 1} {
		if err := p.SeekTo(ctx, target); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(2 * time.Second)
		for {
			position, err := p.Position(ctx)
			if err == nil && position >= target-.2 && position <= target+.2 {
				break
			}
			if time.Now().After(until) {
				t.Fatalf("seek target %v: position=%v err=%v", target, position, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		paused, err := p.command(ctx, []any{"get_property", "pause"})
		if err != nil || string(paused) != "true" {
			t.Fatal("seek resumed paused audio", string(paused), err)
		}
	}
	if err := p.Play(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	stopped := awaitEvent(t, p, "end-file", "stop")
	if stopped.PlaylistEntryID == nil || *stopped.PlaylistEntryID != entryID {
		t.Fatalf("end-file entry ID = %v, expected %d", stopped.PlaylistEntryID, entryID)
	}
	if err := p.Load(ctx, silenceWAV(t, 1), 0); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, p, "end-file", "eof")
	if err := p.Load(ctx, filepath.Join(t.TempDir(), "absent.mp3"), 0); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, p, "end-file", "error")
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRealMPVOwnerProbe(t *testing.T) {
	if os.Getenv("PODMUTT_OWNER_PROBE") != "1" {
		return
	}
	p, err := Start(context.Background(), os.Getenv("PODMUTT_TEST_MPV"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(p.cmd.Process.Pid)
	time.Sleep(time.Hour) // test parent kills this owner without running cleanup
}

func TestRealMPVExitsAfterOwnerKilled(t *testing.T) {
	integrationBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRealMPVOwnerProbe$")
	owner.Env = append(os.Environ(), "PODMUTT_OWNER_PROBE=1")
	stdout, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { owner.Process.Kill(); owner.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("owner did not report mpv PID")
	}
	pid, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL) // cleanup if the assertion fails
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	owner.Wait()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		// Containers may not reap orphan zombies promptly; a zombie has exited
		// and cannot keep audio or IPC alive. macOS init normally reaps it.
		if stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			fields := strings.Fields(string(stat))
			if len(fields) > 2 && fields[2] == "Z" {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("mpv survived abrupt owner death")
}
