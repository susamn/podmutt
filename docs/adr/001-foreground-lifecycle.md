# ADR 001: Foreground-owned playback

Status: accepted from product discussion.

## Decision

The Bubble Tea application owns playback state, queue advancement, refresh jobs,
and a child mpv process. It uses a private local IPC endpoint to control mpv.
There is no daemon or service, and quitting stops all app-owned work.

Persist position periodically and on pause, stop, episode change, and graceful
exit. Restore the current episode and queue on startup without requiring mpv to
have survived. A crash may lose the last checkpoint; exact interval is pending.

A profile lock admits one TUI. Short-lived CLI invocations contact a separate
private application control endpoint, so actions use the same state transitions
as keyboard input. They do not write playback state independently or launch a
player when no TUI is running. Endpoint permissions and stale-lock recovery
must be tested.

## Consequences

Simple cross-platform lifecycle; no systemd/launchd setup. Commands work only
while the TUI is alive. No playback, refresh, or queue advancement after quit.

Graceful cleanup alone does not cover SIGKILL or crashes. Verify an mpv
parent-liveness mechanism on both platforms so the child also exits after an
unexpected parent death; this is a required lifecycle prototype, not an assumed
property of os/exec.

Audio spectrum extraction is independent of JSON playback control and needs a
separate validated approach.
