# ADR 037: Selection-independent playback controls

Status: accepted for #39 (playback slice).

Space pauses/resumes current audio in every pane, never the highlighted episode.
With no current episode, show guidance; do not start the selection or queue.
Enter opens shows and plays episodes (stored, queued, or preview). Repeating
Enter keeps the episode playing instead of toggling pause. Player Enter still
activates its explicitly selected transport button.

`i` opens episode details from Episodes/Queue; Esc restores the originating list
and selection. Selection continues to populate descriptions without extra input.
Editors, dialogs, help, and settings keep ownership of their keys.

Reuse the existing playback actor and preview cache path. No storage migration,
new dependency, autoplay, or change to CLI/media transport semantics is needed.
This supersedes selected-Space and episode-Enter-details behavior in ADRs 011,
018, and 019. Navigation, queue policy, and discovery simplification remain
separate deliveries under #39/#40.
