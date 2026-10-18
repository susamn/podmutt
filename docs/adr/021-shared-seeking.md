# ADR 021: Explicit actor-owned seeks

Status: implemented Go path; native macOS skip-command extension needs native check.

Player h/l dispatch -15/+15 seconds; Tab/Shift-Tab or numeric focus always change
panes. Outside Player, h/l keep focus navigation. Fixed Player buttons remain
Play/Pause, Stop, Next; seek hints appear when space permits and in focused help.

The actor offers SeekBy and SeekTo. A seek requires loaded/ready playback and an
optional SeekTo-capable player. mpv uses public JSON IPC absolute+exact seek and
preserves pause state. Relative seeks poll actual position, accumulate consecutive
pending seeks, clamp zero/known duration, checkpoint accepted targets and publish
shared snapshots. Unknown duration is allowed; invalid/nonfinite input is rejected.
Seek rejection leaves playback/queue intact with a visible error; persistence
failure follows normal fail/cleanup policy. EOF/queued observations are drained
in a bounded pass before seeking; a changed entry rejects the request.

mpv acknowledges before all observed positions catch up. A two-second correction
window ignores observations far from the accepted target; matching observations
or timeout restore normal clock correction. Pause/quit polling does not overwrite
the accepted target with pre-seek positions in this window. Track loads reset the
window. This is bounded reconciliation, not a fabricated audio clock.

macOS registers public MPSkipIntervalCommand forward/backward handlers with a
preferred 15-second interval, routed through the existing bounded actor worker.
Continuous seeking and absolute system scrubbing remain disabled; do not claim
those controls work. Linux TUI seeking works; MPRIS still truthfully advertises
CanSeek=false because its separate absolute/relative wire API is not implemented.
No private API/global key hook/new dependency. Nativecheck expands actual registered
Objective-C-block routing/capability/cleanup tests; Linux cross-builds cannot run it.

Tests cover clamps, pause/queue/resume checkpoints, rapid seeks, stale position
reconciliation, unsupported/unready playback, EOF identity, rejected seeks,
keyboard/input guards and real-mpv paused forward/backward seek (null audio in CI).
