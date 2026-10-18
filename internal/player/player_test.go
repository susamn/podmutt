package player

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func helperPlayer(t *testing.T, mode string, timeout time.Duration) (*Player, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMPVHelper$")
	cmd.Env = append(os.Environ(), "PODMUTT_MPV_HELPER="+mode, "GORACE=atexit_sleep_ms=0")
	return launch(context.Background(), cmd, timeout)
}

// TestMPVHelper is a disposable child process, not an mpv substitute in production.
func TestMPVHelper(t *testing.T) {
	mode := os.Getenv("PODMUTT_MPV_HELPER")
	if mode == "" {
		return
	}
	file := os.NewFile(3, "ipc")
	defer file.Close()
	decoder, encoder := json.NewDecoder(file), json.NewEncoder(file)
	for {
		var request struct {
			Command   json.RawMessage `json:"command"`
			RequestID uint64          `json:"request_id"`
		}
		if decoder.Decode(&request) != nil {
			if mode == "hang" {
				time.Sleep(time.Hour)
			}
			os.Exit(0)
		}
		if mode == "silent" {
			continue
		}
		result := map[string]any{"request_id": request.RequestID, "error": "success"}
		var args []any
		if json.Unmarshal(request.Command, &args) == nil {
			if args[0] == "get_property" {
				result["data"] = 12.5
				if args[1] == "mpv-version" {
					result["data"] = "test-mpv"
				} else if args[1] == "playlist/0/id" && mode != "bad-position" {
					result["data"] = 7
				} else if mode == "bad-position" {
					result["data"] = "unknown"
				}
			}
			if args[0] == "seek" && (len(args) != 3 || args[1] != float64(15) || args[2] != "absolute+exact") {
				result["error"] = "unexpected seek arguments"
			}
			if mode == "reject-observe" && args[0] == "observe_property" {
				result["error"] = "test rejection"
			}
			if args[0] == "stop" {
				encoder.Encode(map[string]any{"event": "end-file", "reason": "stop"})
			}
		} else {
			var command map[string]any
			json.Unmarshal(request.Command, &command)
			options, _ := command["options"].(map[string]any)
			if command["name"] != "loadfile" || command["flags"] != "replace" || options["start"] != "42.25" || options["pause"] != "no" {
				result["error"] = "unexpected load options"
			}
			encoder.Encode(map[string]any{"event": "end-file", "reason": "eof"})
		}
		encoder.Encode(result)
	}
}

func TestPlayerControlsAndClose(t *testing.T) {
	p, err := helperPlayer(t, "normal", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	for _, action := range []func() error{
		func() error { return p.Load(ctx, "https://example.test/audio.mp3", 42.25) },
		func() error { return p.Pause(ctx) },
		func() error { return p.Play(ctx) },
		func() error { return p.Stop(ctx) },
		func() error { return p.SeekTo(ctx, 15) },
	} {
		if err := action(); err != nil {
			t.Fatal(err)
		}
	}
	if id, err := p.EntryID(ctx); err != nil || id != 7 {
		t.Fatalf("entry ID = %v, %v", id, err)
	}
	if position, err := p.Position(ctx); err != nil || position != 12.5 {
		t.Fatalf("position = %v, %v", position, err)
	}
	if ended, stopped := <-p.Events(), <-p.Events(); ended.Reason != "eof" || stopped.Reason != "stop" {
		t.Fatalf("end reasons lost: %+v, %+v", ended, stopped)
	}
	for _, position := range []float64{-1, math.NaN(), math.Inf(1)} {
		if err := p.SeekTo(ctx, position); err == nil {
			t.Fatal("invalid seek position accepted")
		}
		if err := p.Load(ctx, "url", position); err == nil {
			t.Fatal("invalid start position accepted")
		}
	}
	if err := p.Load(ctx, " ", 0); err == nil {
		t.Fatal("empty media URL accepted")
	}
	for range 2 {
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.EntryID(ctx); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("closed player entry ID: %v", err)
	}
	if _, err := p.Position(ctx); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("closed player command: %v", err)
	}
}

func TestPlayerStartupFailures(t *testing.T) {
	if _, err := Start(context.Background(), "/no/such/mpv"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing mpv: %v", err)
	}
	if _, err := launch(context.Background(), exec.Command("/no/such/mpv"), time.Second); err == nil {
		t.Fatal("invalid executable launched")
	}
	for _, mode := range []string{"silent", "reject-observe"} {
		t.Run(mode, func(t *testing.T) {
			if p, err := helperPlayer(t, mode, 200*time.Millisecond); err == nil {
				p.Close()
				t.Fatal("startup failure ignored")
			}
		})
	}
}

func TestPlayerBadPositionAndForcedShutdown(t *testing.T) {
	p, err := helperPlayer(t, "bad-position", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.EntryID(context.Background()); err == nil {
		t.Fatal("invalid playlist entry ID accepted")
	}
	if _, err := p.Position(context.Background()); err == nil {
		t.Fatal("invalid position accepted")
	}
	p.Close()
	p, err = helperPlayer(t, "hang", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected forced shutdown: %v", err)
	}
	select {
	case <-p.exited:
	default:
		t.Fatal("killed child not reaped")
	}
}
