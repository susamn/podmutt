# ADR 048: Obvious library navigation and newest-first playback priority

Status: design accepted; implementation in progress in #51.

## Contract

Library contains All episodes, a muted Subscriptions heading, and shows. Selecting
a scope immediately changes Episodes. Not started and Continue listening are no
longer sidebar destinations. Library episode sorting starts newest first; o keeps
the newest/oldest/title cycle. / searches the focused panel. Selection uses row
backgrounds, never > cursors. Discovery Enter loads a show without leaving Podcasts.

Queue presentation and playback priority are independent. Its default display is
newest first; o switches between that and channel alphabetically + newest first.
It must not write a reordered list back to the actor. No manual J/K moves. Manual
additions append and become selected, without starting playback or subscribing.

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

App opening will explicitly resume its saved unfinished current episode and align
show/episode/queue selections once. Ordinary progress updates must not snap back
browsing selection. Unsubscribed saved playback must never implicitly subscribe.
The actor constructor remains side-effect-free; application startup owns resume.

Current audio must be visible in the queue presentation without storing a duplicate
in the actor's pending queue or making current audio removable by pending edits.
Keep current playback distinct from inspected metadata; preserve pinned reading
across progress/EOF/refresh and resizing. Retain readable errors and transport.

## Staging and verification

Initial implemented slice: newest-priority EOF/Next and two-release arrivals,
including receipts, rollback and unknown-date tests. Library/layout, display-only
sorting, current-row projection, startup resume/alignment and selection/search
presentation remain **unimplemented** at this stage; the MR stays draft.

Before ready: test play old A → visual-sort queue → EOF newest B; manual tail
addition/selection without autoplay; Space independence; cached/unsubscribed
restart and selection alignment; discovery focus; filter/modal/stale-reply guards;
minimum/compact/wide layouts, all themes/icons and existing offline/undo flows.
Use check/race/recovery/private D-Bus/real-mpv CI. Automated fixtures do not count
as human/native/physical validation.
