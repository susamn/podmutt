package mpris

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
)

func privateBus(t *testing.T) string {
	t.Helper()
	if os.Getenv("PODMUTT_TEST_DBUS") == "0" {
		t.Skip("private-bus integration disabled")
	}
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("PODMUTT_TEST_DBUS") == "1" {
			t.Fatal("dbus-daemon required")
		}
		t.Skip("dbus-daemon not installed")
	}
	ctx, cancel := context.WithTimeout(background, 20*time.Second)
	command := exec.CommandContext(ctx, daemon, "--session", "--nofork", "--nopidfile", "--print-address=1")
	output, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); command.Wait() })
	address, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(address)
}
func client(t *testing.T, address string) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
func listen(t *testing.T, address string, f *fakeBackend) *Service {
	t.Helper()
	s, err := Listen(background, address, f)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func call(t *testing.T, object dbus.BusObject, method string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(background, time.Second)
	defer cancel()
	if err := object.CallWithContext(ctx, playerInterface+"."+method, 0, args...).Err; err != nil {
		t.Fatal(err)
	}
}
func owns(t *testing.T, conn *dbus.Conn) bool {
	t.Helper()
	var owner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, Name).Store(&owner); err != nil {
		t.Fatal(err)
	}
	return owner
}

func TestPrivateBusExportControlsAndRelease(t *testing.T) {
	address := privateBus(t)
	f := fixture()
	service := listen(t, "unix:path=/missing/fallback;"+address, f)
	conn := client(t, address)
	object := conn.Object(Name, Path)
	if !owns(t, conn) || property(t, object, "Position") != int64(12_000_000) {
		t.Fatal("service not ready")
	}
	if property(t, object, "CanGoNext") != false || property(t, object, "CanSeek") != false {
		t.Fatal("unsupported controls advertised")
	}
	var xml string
	if err := object.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml); err != nil || !strings.Contains(xml, "PlayPause") || !strings.Contains(xml, "PropertiesChanged") {
		t.Fatalf("introspection: %s %v", xml, err)
	}
	call(t, object, "Play")
	eventually(t, func() bool { return property(t, object, "PlaybackStatus") == "Playing" })
	call(t, object, "PlayPause")
	eventually(t, func() bool { return property(t, object, "PlaybackStatus") == "Paused" })
	call(t, object, "Next")
	state, _ := f.Snapshot(background)
	if state.Current.ID != 1 || state.Status != playback.Paused {
		t.Fatal("Next resumed paused playback")
	}
	call(t, object, "PlayPause")
	call(t, object, "Next")
	eventually(t, func() bool {
		metadata := property(t, object, "Metadata").(map[string]dbus.Variant)
		return metadata["xesam:title"].Value() == "Queued episode"
	})
	call(t, object, "Stop")
	eventually(t, func() bool { return property(t, object, "PlaybackStatus") == "Stopped" })
	if err := object.Call(playerInterface+".OpenUri", 0, "file:///private").Err; err == nil {
		t.Fatal("OpenUri accepted")
	}
	if err := object.SetProperty(playerInterface+".Volume", .2); err == nil {
		t.Fatal("unsupported volume accepted")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return !owns(t, conn) })
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	listen(t, address, fixture()) // the well-known name can be reused after teardown
}

func TestPrivateBusSignalsAndNameCollision(t *testing.T) {
	address := privateBus(t)
	f := fixture()
	f.state.Status = playback.Playing
	f.state.Current.PositionSeconds = 0
	service := listen(t, address, f)
	if _, err := Listen(background, address, fixture()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("name collision displaced owner")
	}
	conn := client(t, address)
	object := conn.Object(Name, Path)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(Path)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	state, _ := f.Snapshot(background)
	state.Current.PositionSeconds = 1
	f.set(state)
	eventually(t, func() bool { return property(t, object, "Position") == int64(1_000_000) })
	select {
	case signal := <-signals:
		t.Fatalf("ordinary progress signaled: %s", signal.Name)
	case <-time.After(20 * time.Millisecond):
	}
	call(t, object, "Pause")
	var signal *dbus.Signal
	select {
	case signal = <-signals:
	case <-time.After(time.Second):
		t.Fatal("no PropertiesChanged")
	}
	if signal.Name != propertiesInterface+".PropertiesChanged" {
		t.Fatalf("wrong signal: %s", signal.Name)
	}
	updates := signal.Body[1].(map[string]dbus.Variant)
	if updates["PlaybackStatus"].Value() != "Paused" {
		t.Fatalf("status change: %+v", updates)
	}
	if _, ok := updates["Position"]; ok {
		t.Fatal("position emitted as property change")
	}
	conn.Close() // a client disappearing must not stop the foreground service
	if service.Err() != nil {
		t.Fatal(service.Err())
	}
	service.conn.Close()
	eventually(t, func() bool { return errors.Is(service.Err(), ErrUnavailable) })
}

func TestPrivateBusFailureAndCancellationLifetimes(t *testing.T) {
	address := privateBus(t)
	bad := fixture()
	bad.fail = "watch"
	if _, err := Listen(background, address, bad); !errors.Is(err, ErrUnavailable) {
		t.Fatal("failed subscription accepted")
	}
	f := fixture()
	ctx, cancel := context.WithCancel(background)
	service, err := Listen(ctx, address, f)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-service.done:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked bus service")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	conn := client(t, address)
	eventually(t, func() bool { return !owns(t, conn) })
	f = fixture()
	service = listen(t, address, f)
	f.mu.Lock()
	for stream := range f.streams {
		delete(f.streams, stream)
		close(stream)
	}
	f.mu.Unlock()
	select {
	case <-service.done:
	case <-time.After(time.Second):
		t.Fatal("closed actor leaked service")
	}
}
