# ADR 011: Selected playback and identity-based queue editing

Status: accepted for v1

## Decision

Space in Episodes/Queue plays the selected identity; if it is current, toggle
play/pause. Other panes toggle current playback. Enter opens show/episode details
and never starts audio. All text inputs and help consume transport keys first.

`a` enqueues one selected Episodes/Queue item; `A` enqueues visible (locally
filtered) Episodes in display order. Existing actor deduplication applies.
`J`/`K` move a queued identity one step. `d` removes one captured identity and
`D` clears a captured queue, both after explicit `y`; Esc/n cancels. Filters do
not change the meaning of clear: it applies to the full queue. Confirmation does
not suppress asynchronous state/refresh updates. A changed queue invalidates
clear; removal affects only captured IDs. Queue edits execute atomically in the
playback actor, not by submitting an old UI snapshot with SetQueue.

Queue snapshots render through the same ID-preserving UI update path as library
rows. ADR 014 replaces the compact Queue/Details overlays with an inline focused
pane and persistent player. The player shows
status, position, optional feed duration, and persistent actor errors. Failed
operations produce a notice; they do not erase the actor's persistent failure.

## Consequences

No new dependencies or second player owner. Concurrent EOF/enqueues cannot be
undone by stale reorder/clear commands. Current transport/history are independent
of queue removal/clear. Existing actor shutdown checkpoints and reaps the player.
Batch selection is explicitly the visible list, not an implicit whole show.
