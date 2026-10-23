package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func pipeIPC(t *testing.T, timeout time.Duration) (*ipc, net.Conn) {
	t.Helper()
	local, remote := net.Pipe()
	client := newIPC(local, timeout)
	t.Cleanup(func() { remote.Close(); client.fail(ErrDisconnected); <-client.done })
	return client, remote
}

func TestSetVolumeReadsReportedValueAndValidatesRange(t *testing.T) {
	client, remote := pipeIPC(t, time.Second)
	go func() {
		decoder, encoder := json.NewDecoder(remote), json.NewEncoder(remote)
		for _, data := range []any{nil, 74.7} {
			var frame map[string]any
			if decoder.Decode(&frame) != nil {
				return
			}
			command := frame["command"].([]any)
			if command[0] == "set_property" && (command[1] != "volume" || command[2] != float64(75)) {
				return
			}
			encoder.Encode(map[string]any{"request_id": frame["request_id"], "error": "success", "data": data})
		}
	}()
	actual, err := (&Player{ipc: client}).SetVolume(context.Background(), 75)
	if err != nil || actual != 75 {
		t.Fatalf("SetVolume = %d, %v", actual, err)
	}
	if _, err := (&Player{ipc: client}).SetVolume(context.Background(), 131); err == nil {
		t.Fatal("out-of-range volume accepted")
	}
}

func TestVolumeRejectsBadPlayerRepliesAndChapterErrors(t *testing.T) {
	for _, reply := range []any{"invalid", -1, 131, nil} {
		t.Run(fmt.Sprint(reply), func(t *testing.T) {
			client, remote := pipeIPC(t, time.Second)
			go func() {
				decoder, encoder := json.NewDecoder(remote), json.NewEncoder(remote)
				for _, data := range []any{nil, reply} {
					var frame map[string]any
					if decoder.Decode(&frame) != nil {
						return
					}
					encoder.Encode(map[string]any{"request_id": frame["request_id"], "error": "success", "data": data})
				}
			}()
			if _, err := (&Player{ipc: client}).SetVolume(context.Background(), 65); err == nil {
				t.Fatal("invalid reported volume accepted")
			}
		})
	}
	client, remote := pipeIPC(t, time.Second)
	go func() {
		var frame map[string]any
		json.NewDecoder(remote).Decode(&frame)
		json.NewEncoder(remote).Encode(map[string]any{"request_id": frame["request_id"], "error": "property unavailable"})
	}()
	if _, err := (&Player{ipc: client}).ChapterList(context.Background()); err == nil {
		t.Fatal("missing chapter-list property accepted as valid chapters")
	}
}

func TestConcurrentRequestsMatchOutOfOrderReplies(t *testing.T) {
	client, remote := pipeIPC(t, time.Second)
	const count = 16
	go func() {
		decoder, encoder := json.NewDecoder(remote), json.NewEncoder(remote)
		var frames []map[string]any
		for range count {
			var frame map[string]any
			if decoder.Decode(&frame) != nil {
				return
			}
			frames = append(frames, frame)
		}
		for i := len(frames) - 1; i >= 0; i-- {
			frame := frames[i]
			encoder.Encode(map[string]any{"request_id": frame["request_id"], "error": "success", "data": frame["command"]})
		}
	}()
	var wg sync.WaitGroup
	for i := range count {
		wg.Go(func() {
			data, err := client.command(context.Background(), i)
			if err != nil || string(data) != fmt.Sprint(i) {
				t.Errorf("request %d received %s, %v", i, data, err)
			}
		})
	}
	wg.Wait()
}

func TestTimeoutCancellationAndLateReply(t *testing.T) {
	client, remote := pipeIPC(t, 30*time.Millisecond)
	go func() {
		var frame map[string]any
		json.NewDecoder(remote).Decode(&frame)
		time.Sleep(60 * time.Millisecond)
		json.NewEncoder(remote).Encode(map[string]any{"request_id": frame["request_id"], "error": "success"})
	}()
	if _, err := client.command(context.Background(), []any{"silent"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.command(cancelled, []any{"cancelled"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled command = %v", err)
	}
}

func TestCommandErrorAndEvents(t *testing.T) {
	client, remote := pipeIPC(t, time.Second)
	go func() {
		var frame map[string]any
		json.NewDecoder(remote).Decode(&frame)
		encoder := json.NewEncoder(remote)
		encoder.Encode(map[string]any{"event": "property-change", "name": "time-pos", "data": 12.5})
		encoder.Encode(map[string]any{"event": "end-file", "reason": "error", "file_error": "loading failed"})
		encoder.Encode(map[string]any{"request_id": frame["request_id"], "error": "property unavailable"})
	}()
	if _, err := client.command(context.Background(), []any{"get_property", "missing"}); err == nil {
		t.Fatal("mpv error ignored")
	}
	property, ended := <-client.events, <-client.events
	if property.Property != "time-pos" || string(property.Data) != "12.5" || ended.Reason != "error" || string(ended.FileError) != `"loading failed"` {
		t.Fatalf("events: %+v, %+v", property, ended)
	}
}

func TestDisconnectAndInvalidMessage(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			client, remote := pipeIPC(t, time.Second)
			go func() {
				var frame map[string]any
				json.NewDecoder(remote).Decode(&frame)
				if invalid {
					fmt.Fprintln(remote, "invalid JSON")
				} else {
					remote.Close()
				}
			}()
			if _, err := client.command(context.Background(), "test"); err == nil {
				t.Fatal("broken IPC accepted")
			}
			<-client.done
			if client.Err() == nil {
				t.Fatal("terminal error missing")
			}
			if _, err := client.command(context.Background(), "test"); err == nil {
				t.Fatal("disconnected command accepted")
			}
		})
	}
}

func TestEventOverflowFailsExplicitly(t *testing.T) {
	client, remote := pipeIPC(t, time.Second)
	go func() {
		encoder := json.NewEncoder(remote)
		for range cap(client.events) + 1 {
			if encoder.Encode(map[string]any{"event": "end-file", "reason": "eof"}) != nil {
				return
			}
		}
	}()
	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("overflow did not terminate IPC")
	}
	if !errors.Is(client.Err(), ErrEventOverflow) {
		t.Fatalf("overflow error = %v", client.Err())
	}
}

func TestBlockedWriteIsBounded(t *testing.T) {
	client, _ := pipeIPC(t, 20*time.Millisecond)
	if _, err := client.command(context.Background(), "no reader"); err == nil {
		t.Fatal("blocked write succeeded")
	}
}
