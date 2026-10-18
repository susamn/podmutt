# ADR 010: Foreground Linux media controls

Status: proposed in #15.

Export org.mpris.MediaPlayer2.podmutt on an existing Linux session bus. Use
Godbus v5.2.4 (pure Go, BSD-2-Clause): stdlib has no D-Bus authentication,
marshalling, dispatch, or introspection implementation. Do not hand-write that
protocol, autolaunch a daemon, use remote transports, or add CGO. Other platforms
are a no-op; native macOS integration remains #16.

Route play/pause/stop/next through the same dispatcher/actor as TUI and CLI.
Use independent coalesced actor watchers rather than consuming the TUI's stream
or polling. Export current track ID/title/length, position in microseconds, and
honest capabilities. Normal progress never emits Position property changes;
unexpected jumps/resume offsets emit Seeked.

This is a deliberately limited MPRIS surface: no previous, seek, OpenUri, window
raise, remote quit, volume/rate/repeat/shuffle changes. Advertise Next only while
playing with an explicit queued item; it must not resume a paused podcast.
Stop retains podmutt's resume checkpoint rather than rewinding (a deliberate
podcast-policy exception to the MPRIS Stop/restart wording).

Bound connection/authentication startup to 2 seconds and actions to 3 seconds.
A missing, occupied, failed, or disconnected bus is nonfatal to playback. Close
the private connection/subscription before the actor; process death also releases
bus ownership. Do not leak bus addresses or backend errors through D-Bus errors.

References: [MPRIS](https://specifications.freedesktop.org/mpris-spec/latest/),
[Godbus](https://pkg.go.dev/github.com/godbus/dbus/v5).
