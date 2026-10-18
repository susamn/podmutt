package mpris

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
)

func TestMediaActionsUseSharedDispatcher(t *testing.T) {
	f := fixture()
	s := serviceFor(f)
	player := &playerMethods{s}
	for _, method := range []func() *dbus.Error{
		player.Play, player.Play, player.Pause, player.Pause, player.Next,
		player.PlayPause, player.Next, player.Stop, player.PlayPause, player.PlayPause,
		player.Previous, func() *dbus.Error { return seek(10) }, func() *dbus.Error { return player.SetPosition("/stale", 10) },
	} {
		if err := method(); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(f.calls, ","); got != "play,pause,next,pause,stop,play,pause" {
		t.Fatalf("actions: %s", got)
	}
	f.state.Current = nil
	f.state.Status = playback.Stopped
	if err := player.Play(); err != nil {
		t.Fatal(err)
	}
	if err := player.PlayPause(); err == nil {
		t.Fatal("empty toggle accepted")
	}
	if err := player.OpenUri("file:///tmp/private"); err == nil {
		t.Fatal("arbitrary URI opened")
	}
	root := &rootMethods{}
	if root.Raise() == nil || root.Quit() == nil {
		t.Fatal("unsupported window/quit actions accepted")
	}
}
func TestActionErrorsAreMappedWithoutLeakingBackendDetails(t *testing.T) {
	for _, command := range []string{"play", "pause", "stop", "next", "snapshot"} {
		f := fixture()
		f.fail = command
		if command == "pause" || command == "next" {
			f.state.Status = playback.Playing
		}
		if err := serviceFor(f).act(command); err == nil || !strings.HasSuffix(err.Name, ".Failed") {
			t.Fatalf("%s: %v", command, err)
		}
	}
}
