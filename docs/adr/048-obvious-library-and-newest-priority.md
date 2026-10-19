# ADR 048: Obvious library navigation and newest-first playback priority

Status: implemented in draft MR !57 / #51; validation in progress.

## Contract

Library contains All episodes, a muted Subscriptions heading, and shows. Selecting
a scope immediately changes Episodes. Not started and Continue listening are no
longer sidebar destinations. Library episode sorting starts newest first; o keeps
the newest/oldest/title cycle. / searches the focused panel. Selection uses row
backgrounds, never > cursors. Discovery Enter loads a show without leaving Library.
Wide layout is Library | Episodes over Details | compact Player over Queue.
Compact layout keeps Player above the focused list; geometry is shared with paging,
reading, artwork and animation.

Queue presentation and playback priority are independent. Its default display is
newest first; o switches between that and channel alphabetically + newest first.
It must not write a reordered list back to the actor. No manual J/K moves. Manual
additions append and become selected/focus Queue, without starting playback or
subscribing. Session-local display-tail overrides preserve explicit append order
until o re-sorts; neither mode writes presentation order back. Input/modals defer
Queue focus until closed. Actor snapshots carry lifetime-local revisions so a late
stream publication cannot replace a newer enqueue/complete reply.

Enter deliberately plays the selection. Space controls current audio only. After
any episode finishes (including a manually chosen older episode), the actor picks
the newest pending publication, not the row following the current selection or
the first row of a channel-grouped view. Next uses the same priority without
marking the skipped episode completed. Equal/unknown publication times retain
persisted queue order as a stable tie-breaker; priority selection never mutates it.

Subscription reconciliation considers exactly the latest two cached releases per
show, then skips completed/current/already-seen episodes. It does not continuously
backfill an older historical episode when one completes. Durable arrival receipts,
manual work, completion and undo semantics remain intact. No schema change.

App opening explicitly resumes its saved unfinished current episode and align
show/episode/queue selections once. Ordinary progress updates must not snap back
browsing selection. Unsubscribed saved playback must never implicitly subscribe.
The actor constructor remains side-effect-free; application startup owns resume.

Current audio must be visible in the queue presentation without storing a duplicate
in the actor's pending queue or making current audio removable by pending edits.
Keep current playback distinct from inspected metadata; preserve pinned reading
across progress/EOF/refresh and resizing. Retain readable errors and transport.

## Staging and verification

Implemented: newest-priority EOF/Next and two-release arrivals (including receipts,
rollback and unknown dates), simplified Library/layout, display-only sorting,
protected current-row projection, explicit enqueue tail/focus, discovery focus,
and one-shot app startup resume/alignment. Progress never continually realigns.
Activation can leave an old show scope to reveal newly current audio, but does not
hijack active discovery or filter input. Existing pinned Details and queue undo
remain separate from transport. No dependency or database schema change.

Verification covers play old A → visual-sort queue → EOF newest B; manual tail
addition/selection without autoplay; Space independence; cached/unsubscribed
restart and selection alignment; discovery focus; filter/modal/stale-reply guards;
minimum/compact/wide layouts, all themes/icons and existing offline/undo flows.
Use check/race/recovery/private D-Bus/real-mpv CI. Automated fixtures do not count
as human/native/physical validation.
