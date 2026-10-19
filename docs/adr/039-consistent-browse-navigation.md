# ADR 039: Browse with open/back, not horizontal scrolling

Status: accepted for #39 (navigation slice).

`h`/Left goes back; `l`/Right opens shows or inspects episode details without
starting audio. Reuse Enter's show-opening and `i`'s details paths. Player arrows
remain explicit transport-button selection; Player `h` leaves and never seeks.
Only global `,/.` (and existing </> aliases) seek.

Tab/Shift-Tab cycles Podcasts, Episodes, Queue, skipping supporting panes.
Keep direct 1–5 focus and `i` details. Esc clears an applied list filter before
leaving; details return to their originating list/selection. Leaving Queue must
not discard a discovery preview. Editors and overlays retain key ownership.

Add half/full-page list movement using viewport capacity, shared with rendering.
Remove horizontal offset state and duplicate slicing; full metadata remains in
Details. No dependency, persistence, queue-policy, or layout redesign is needed.
This supersedes scrolling/Player-seek and all-pane Tab behavior in ADRs 021/033.
Explicit library destinations remain a separate #39 delivery.
