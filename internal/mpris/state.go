// Package mpris adapts foreground playback to the MPRIS 2 D-Bus interfaces.
package mpris

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
)

const Name = "org.mpris.MediaPlayer2.podmutt"
const Path dbus.ObjectPath = "/org/mpris/MediaPlayer2"
const rootInterface = "org.mpris.MediaPlayer2"
const playerInterface = rootInterface + ".Player"
const propertiesInterface = "org.freedesktop.DBus.Properties"

type values map[string]dbus.Variant

func rootValues() values {
	return variants(map[string]any{
		"CanQuit": false, "CanRaise": false, "HasTrackList": false, "Identity": "podmutt",
		"SupportedUriSchemes": []string{}, "SupportedMimeTypes": []string{},
	})
}
func playerValues(state playback.State) values {
	status := "Stopped"
	switch state.Status {
	case playback.Playing:
		status = "Playing"
	case playback.Paused:
		status = "Paused"
	}
	metadata := map[string]dbus.Variant{}
	position := int64(0)
	if episode := state.Current; episode != nil {
		metadata["mpris:trackid"] = dbus.MakeVariant(dbus.ObjectPath("/org/podmutt/episode/" + strconv.FormatInt(episode.ID, 10)))
		metadata["xesam:title"] = dbus.MakeVariant(strings.ReplaceAll(strings.ToValidUTF8(episode.Title, "�"), "\x00", ""))
		if episode.DurationSeconds > 0 {
			metadata["mpris:length"] = dbus.MakeVariant(microseconds(float64(episode.DurationSeconds)))
		}
		position = microseconds(episode.PositionSeconds)
		if length, ok := metadata["mpris:length"]; ok {
			position = min(position, length.Value().(int64))
		}
	}
	return variants(map[string]any{
		"PlaybackStatus": status, "Metadata": metadata, "Position": position,
		"Rate": 1.0, "MinimumRate": 1.0, "MaximumRate": 1.0, "Volume": 1.0, "LoopStatus": "None", "Shuffle": false,
		"CanControl": true, "CanPlay": state.Current != nil, "CanPause": state.Current != nil,
		"CanGoNext": state.Status == playback.Playing && len(state.Queue) > 0, "CanGoPrevious": false, "CanSeek": false,
	})
}
func variants(source map[string]any) values {
	result := make(values, len(source))
	for name, value := range source {
		result[name] = dbus.MakeVariant(value)
	}
	return result
}
func microseconds(seconds float64) int64 {
	if math.IsNaN(seconds) || seconds <= 0 {
		return 0
	}
	if seconds >= float64(math.MaxInt64)/1e6 {
		return math.MaxInt64
	}
	return int64(seconds * 1e6)
}
func changed(before, after values) values {
	result := values{}
	for name, value := range after {
		if name != "Position" && name != "CanControl" && !reflect.DeepEqual(before[name], value) {
			result[name] = value
		}
	}
	return result
}

// Seeked is for discontinuities, not normal progress. Starting/resuming at a
// saved nonzero offset is explicit; clients otherwise assume Stopped starts at 0.
func jumped(before, after values, elapsed time.Duration) bool {
	oldPosition := before["Position"].Value().(int64)
	position := after["Position"].Value().(int64)
	oldMeta := before["Metadata"].Value().(map[string]dbus.Variant)
	metadata := after["Metadata"].Value().(map[string]dbus.Variant)
	if !reflect.DeepEqual(oldMeta["mpris:trackid"], metadata["mpris:trackid"]) {
		return position != 0
	}
	if before["PlaybackStatus"].Value() == "Stopped" && after["PlaybackStatus"].Value() == "Playing" {
		return position != 0
	}
	if before["PlaybackStatus"].Value() == "Playing" {
		delta := position - oldPosition
		return delta < -1_000_000 || delta > elapsed.Microseconds()+2_000_000
	}
	return position != oldPosition
}
