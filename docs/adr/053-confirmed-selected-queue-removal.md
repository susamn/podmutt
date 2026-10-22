# ADR 053: Confirm removal of the selected Queue row, including current audio

Status: implemented; supersedes single-row immediate/pending-only removal in
ADRs 018, 044 and 048. Batch clear remains pending-only.

## Contract

Only in Queue, `d`/Delete opens a y/n confirmation for the selected episode.
Show its title, show, state and saved position/duration; state that progress and
history are retained. Current rows use “Stop & remove?” rather than suggesting
that Stop alone will make them removable. `n`/Esc leaves playback and queue
unchanged; ordinary navigation/edit keys cannot execute a removal behind the
modal. `q`/Ctrl+C still quits. No deletion bindings are added to other panes.

Capture the episode ID when opening the dialog, never an index. While it is
open, progress, EOF/Next, other controls, refreshes and display sorting may change
the stack. Confirmation removes only that captured identity from the live actor:

- Pending: remove it without changing current playback.
- Current (playing, paused, stopped or failed): checkpoint/stop as applicable,
  delete the durable current pointer, clear current presentation, remain stopped.
  Do not mark completion, erase history, remove subscriptions, or start the next
  episode. Delayed EOF for removed audio cannot advance the queue.
- No longer current or pending: report queue-changed; never remove a replacement
  row or overwrite an earlier successful undo.

If a captured pending episode becomes current before confirmation, it is still
that selected identity and may be stopped/removed. If captured current becomes
pending because another episode was started, remove the pending identity and
leave the new current alone.

Undo remains one session-local removal. Undoing current removal inserts it as
pending work, preserving saved history, never restoring playback or replacing a
new current episode. Explicit replay, re-enqueue, batch clear, completion and
restart retain existing undo-invalidation rules.

## Implementation / failure boundaries

Keep the existing pending-only actor API for callers that need pending-only
edits. A separate selected-row operation serializes identity resolution,
checkpoint/stop, durable-pointer clearing and undo recording. Store.ClearCurrent
uses a conditional DELETE by episode ID, without altering listening history or
queue tables; no schema migration is needed. Stop/checkpoint/clear failure must
not claim removal or overwrite the previous undo. If playback was stopped before
a database clear fails, retain the current pointer and stopped state for retry.

Return an authoritative actor snapshot with the action reply so removal does not
wait for a stream update. Existing revision guards reject stale presentation.
Keep the shared activity widget on finite removal work, not on user decision time.

Row-based dialog text truncates each explicit detail separately, keeping y/n and
n/Esc visible even at 30×12 and with long/Unicode episode metadata. This opt-in
mode does not change wrapping/input behavior in other dialogs.

## Verification

Tests cover current removal in all actor states and completion flags; pause/stop,
saved progress/history, retained pending items, no autoplay, undo, delayed EOF,
restart, live identity transitions and failed checkpoint/Stop/clear operations.
App tests cover d/Delete at minimum/compact/wide sizes, cancellation, other-pane
no-ops, modal key guards, stale/sorted selections and authoritative replies.
Store tests verify conditional deletion, preserved queue/history and reopen.
The default make check aggregate first-party coverage gate is raised to 95%.
