# ADR 041: Separate Now playing from editable Up next

Status: accepted for #39 (queue presentation slice).

Panel 3 becomes Up next and contains only the actor's pending queue, numbered
from 1. Its header shows pending count, or matching/total under a local filter.
Panel 5 becomes Now playing and retains current metadata, errors and transport.
Current is never selectable by queue navigation, removal, reorder, or clear.
Empty/filter-empty pending lists explain their state instead of showing current.

Keep `a`/`A`, `J/K`, `d`/`D`, Enter, global transport and direct 1–5 focus.
Details and selection identity still work for pending entries. Remove obsolete
current-row formatting and UI guard messages rather than duplicating state.

No actor, storage, autoqueue, or interruption semantics change: explicit playback
still preserves interrupted work; EOF completes/advances; restart stays stopped.
This supersedes the combined current-first Queue presentation in ADR 018, not
its persistence/playback-stack rules. Undo/action menus and autoqueue policy
remain separate deliveries.
