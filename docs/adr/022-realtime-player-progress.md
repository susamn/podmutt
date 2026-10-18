# ADR 022: Real-time progress is a presentation clock

Status: implemented for post-v1 #28.

The actor attaches ClockAt to moving playback samples. Current-entry, ready,
transport and actual position changes re-anchor the clock; queue/metadata-only
publications preserve it so adding a queue item cannot reset elapsed time to an
old sample. Loading/stopped/paused playback has no moving anchor.

The UI derives elapsed/total/remaining time and a fractional Unicode progress bar
from numeric playback state. It interpolates at four frames/second for at most
two seconds beyond the last moving sample, then freezes until a fresh correction.
Unknown duration shows elapsed/--:-- without inventing a percentage/remaining time.
Known duration clamps the bar and countdown; only real EOF completes an episode.
Accepted seeks re-anchor via their published position change; actual observations
still correct the clock. This is not seeking, a waveform or audio analysis.

One injected-clock, foreground-context-owned progress timer is pending at a time.
Generation guards reject stale ticks; pause/stop, stale/end position, small/hidden
layouts, prompts/modals and cancellation stop timers. No per-frame mpv calls,
SQLite writes, selection changes, Details-scroll resets or control movement.
The separately labelled existing Decorative pattern is still decorative; its
independent timer does not provide playback position or alter this progress bar.

Tests cover interpolation/corrections, stale-source bounds, unknown duration,
pause/resume/cancellation, timer coalescing/stale generations, resize/compact
rendering and control/Details stability. No dependencies added. Real device/native
macOS checks are distinct from Linux unit tests and fallback cross-builds.
