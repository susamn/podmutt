# Linux media-key smoke check

Automated: `make mpris-test` starts private, temporary dbus-daemon processes and
checks real authentication, discovery/introspection, controls, property/signals,
name collisions, errors, cancellation, and release. It never touches your session
bus. `make check` still passes the 90% gate without dbus-daemon installed.

Manual desktop check (requires a logged-in Linux desktop and current episode):

1. Start podmutt normally. If available, `playerctl --list-all` should include
   `podmutt`; `playerctl --player=podmutt metadata` should show the episode.
2. Press the actual Play/Pause media key: verify mpv and the TUI agree. With a
   queued item, press Next while playing. While paused, Next is intentionally
   disabled and must not resume audio. Test Stop, then Play/resume.
3. Inspect `playerctl --player=podmutt position` and metadata after a track change.
   Seeking is disabled; volume/rate/repeat/shuffle changes are rejected.
4. Quit the TUI: podmutt must disappear from the player list and mpv must exit.
   Restart with `DBUS_SESSION_BUS_ADDRESS=unix:path=/nonexistent`: the TUI/CLI
   must still work and show the nonfatal MPRIS warning.

Record desktop/session type and result on #15. Private-bus tests and cross-builds
are not proof that a desktop routes physical keys to this player. This check is
still pending where no interactive desktop/media keyboard is available.
