# ADR 015: Decorative, cancellation-bound player animation

Status: accepted

A small ASCII pattern scrolls at about 5.5 frames/second in spare rows of the
wide Player pane. It is labelled **Decorative**, not an audio spectrum. There is
no capture, FFT, player polling, subprocess or new dependency. Progress and
controls retain their fixed rows; metadata/errors take priority over decoration.

The foreground context owns every timer. Only one timer is outstanding; its
generation rejects late/duplicate messages after transport or layout changes.
Pause freezes the pattern. Stop/failure clears it. Compact or crowded panes
cancel ticking and hide decoration; a sufficiently tall wide pane resumes it.
Quit cancels the foreground context, including a timer waiting for its deadline.
The waiting clock is injectable for deterministic pause/resize/stale-tick tests.

This is deliberately not a measured visualization. It conveys activity only;
progress/elapsed time remain the playback actor's authoritative values.
