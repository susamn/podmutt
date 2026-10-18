package mpris

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
	"podmutt/internal/store"
)

func stateFixture() playback.State {
	return playback.State{Status: playback.Stopped,
		Current: &store.Episode{ID: 1, Title: "Episode", DurationSeconds: 100, PositionSeconds: 12, AudioURL: "https://secret.invalid/audio"},
		Queue:   []store.Episode{{ID: 2, Title: "Queued episode"}}}
}

func TestMetadataPositionAndCapabilities(t *testing.T) {
	state := stateFixture()
	state.Current.Title = "Title\x00\xff"
	state.Current.PositionSeconds = 200
	values := playerValues(state)
	metadata := values["Metadata"].Value().(map[string]dbus.Variant)
	if metadata["mpris:trackid"].Signature().String() != "o" || metadata["mpris:length"].Value() != int64(100_000_000) || values["Position"].Value() != int64(100_000_000) {
		t.Fatal("metadata signatures/position incorrect")
	}
	if strings.Contains(metadata["xesam:title"].Value().(string), "\x00") || len(metadata) != 3 {
		t.Fatal("unsafe/unnecessary metadata exposed")
	}
	if values["CanGoNext"].Value() != false || values["CanPlay"].Value() != true || values["CanSeek"].Value() != false {
		t.Fatal("dishonest capabilities")
	}
	state.Status = playback.Playing
	if playerValues(state)["CanGoNext"].Value() != true {
		t.Fatal("queued next not advertised")
	}
	state.Current = nil
	state.Status = playback.Failed
	empty := playerValues(state)
	if empty["PlaybackStatus"].Value() != "Stopped" || empty["Position"].Value() != int64(0) || empty["CanPlay"].Value() != false {
		t.Fatal("failed/empty state incorrect")
	}
	for input, want := range map[float64]int64{-1: 0, 1.5: 1_500_000, math.Inf(1): math.MaxInt64} {
		if microseconds(input) != want {
			t.Fatal("microsecond overflow/rounding")
		}
	}
	if microseconds(math.NaN()) != 0 {
		t.Fatal("NaN position")
	}
}
func TestPropertyDeltaExcludesOrdinaryPositionUpdates(t *testing.T) {
	state := stateFixture()
	state.Status = playback.Playing
	state.Current.PositionSeconds = 1
	before := playerValues(state)
	state.Current.PositionSeconds = 2
	after := playerValues(state)
	if len(changed(before, after)) != 0 || jumped(before, after, time.Second) {
		t.Fatal("normal progress emitted changes")
	}
	state.Status = playback.Paused
	after = playerValues(state)
	if changed(before, after)["PlaybackStatus"].Value() != "Paused" {
		t.Fatal("state change omitted")
	}
	if rootValues()["CanQuit"].Value() != false {
		t.Fatal("remote quit advertised")
	}
}
