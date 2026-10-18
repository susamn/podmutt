package mpris

import (
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
)

func TestPropertiesReadWriteAndUnsupportedControls(t *testing.T) {
	f := fixture()
	s := serviceFor(f)
	props := &properties{s}
	if _, err := props.GetAll(rootInterface); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		iface, name string
		value       any
		kind        string
	}{
		{playerInterface, "Rate", 1.0, ""}, {playerInterface, "Volume", 1.0, ""}, {playerInterface, "LoopStatus", "None", ""}, {playerInterface, "Shuffle", false, ""},
		{playerInterface, "Rate", 2.0, "NotSupported"}, {playerInterface, "Volume", .5, "NotSupported"}, {playerInterface, "LoopStatus", "Track", "NotSupported"}, {playerInterface, "Shuffle", true, "NotSupported"},
		{playerInterface, "Rate", "wrong", "InvalidArgs"}, {playerInterface, "LoopStatus", 1, "InvalidArgs"}, {playerInterface, "Shuffle", 1, "InvalidArgs"},
		{playerInterface, "Position", int64(10), "PropertyReadOnly"}, {rootInterface, "Identity", "other", "PropertyReadOnly"},
		{playerInterface, "Missing", 1, "UnknownProperty"}, {"missing", "Position", 1, "UnknownInterface"},
	} {
		err := props.Set(test.iface, test.name, dbus.MakeVariant(test.value))
		if (err == nil) != (test.kind == "") || (err != nil && !strings.HasSuffix(err.Name, "."+test.kind)) {
			t.Fatalf("set %s: %v", test.name, err)
		}
	}
	f.state.Status = playback.Playing
	if err := props.Set(playerInterface, "Rate", dbus.MakeVariant(0.0)); err != nil {
		t.Fatal(err)
	}
	state, _ := f.Snapshot(background)
	if state.Status != playback.Paused {
		t.Fatal("rate 0 did not pause")
	}
	if _, err := props.Get("missing", "x"); err == nil {
		t.Fatal("unknown interface accepted")
	}
	descriptions := describe(playerValues(state), false)
	for _, description := range descriptions {
		if description.Name == "Position" && description.Annotations[0].Value != "false" {
			t.Fatal("Position signals advertised")
		}
		if description.Name == "Rate" && description.Access != "readwrite" {
			t.Fatal("Rate not writable")
		}
	}
	for _, description := range describe(rootValues(), true) {
		if description.Annotations[0].Value != "const" {
			t.Fatal("root not constant")
		}
	}
}

func TestPausedDiscontinuitySignalErrors(t *testing.T) {
	f := fixture()
	f.state.Status = playback.Paused
	s := serviceFor(f)
	s.emit = func(string, ...any) error { return failure }
	f.state.Current.PositionSeconds = 20
	if err := s.update(f.state, s.updated.Add(time.Second)); err != failure {
		t.Fatal("paused discontinuity/emit failure ignored")
	}
}

func TestSignalsDistinguishProgressFromDiscontinuities(t *testing.T) {
	f := fixture()
	s := serviceFor(f)
	var signals []string
	s.emit = func(signal string, args ...any) error {
		signals = append(signals, signal)
		if signal == propertiesInterface+".PropertiesChanged" {
			updates := args[1].(values)
			if _, ok := updates["Position"]; ok {
				t.Fatal("position sent as property change")
			}
			if _, ok := updates["CanControl"]; ok {
				t.Fatal("constant capability sent")
			}
		}
		return nil
	}
	now := s.updated
	f.state.Status = playback.Playing
	if err := s.update(f.state, now); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[1] != playerInterface+".Seeked" {
		t.Fatal("resume offset not signaled")
	}
	signals = nil
	f.state.Current.PositionSeconds = 13
	if err := s.update(f.state, now.Add(time.Second)); err != nil || len(signals) != 0 {
		t.Fatal("normal progress emitted a signal")
	}
	f.state.Current.PositionSeconds = 40
	if err := s.update(f.state, now.Add(2*time.Second)); err != nil || len(signals) != 1 {
		t.Fatal("forward jump not signaled")
	}
	signals = nil
	f.state.Current.PositionSeconds = 0
	if err := s.update(f.state, now.Add(3*time.Second)); err != nil || len(signals) != 1 {
		t.Fatal("backward jump not signaled")
	}
	f.state.Current.ID = 2
	signals = nil
	if err := s.update(f.state, now.Add(4*time.Second)); err != nil || len(signals) != 1 {
		t.Fatal("track change emitted unexpected seek")
	}
	f.state.Current.ID = 3
	f.state.Current.PositionSeconds = 10
	signals = nil
	if err := s.update(f.state, now.Add(5*time.Second)); err != nil || len(signals) != 2 {
		t.Fatal("nonzero new track offset not signaled")
	}
	s.emit = func(string, ...any) error { return failure }
	f.state.Status = playback.Paused
	if err := s.update(f.state, now.Add(6*time.Second)); err != failure {
		t.Fatal("signal error ignored")
	}
}
