# ADR 051: Immediate queue markers and responsive padded Player

Status: accepted; implemented for #54.

Episodes must represent visible Queue membership, not only actor pending storage.
The existing actor keeps current audio out of pending Queue, so pending-only marker
maps omitted newly played episodes until a second activation pushed them to pending.
Use the same visible-current predicate for Queue, Episodes, preview and Details.
A completed stopped current is not visible/queued; completion alone does not stop
active audio. Rebuild markers on current-visibility changes as well as IDs, history
and pending membership. Retain lifetime revision guards and current edit protection.

Enter returns an authoritative actor snapshot with the action reply (including
preview play) rather than relying on a later stream publication. Apply membership
without extra polling, timers, persistence or subscriptions. Explicit enqueue keeps
its existing authoritative reply. Queued Episodes use an accented plain + in the
existing gutter; remove Playing/Paused prefixes from Episodes/Queue while preserving
current identity and full-row selection styling.

Player identity prioritizes the full channel, then allocates all leftover width to
the title, with two cells of padding on each side. Remove fixed 50/20 caps. When the
channel cannot physically fit, truncate safely while retaining a small readable title.
Normal Player is seven rows: borders, one identity line, timeline, transport and
one blank row at each end. Padding yields before notices/core content or minimum
compact Details identity. Decoration, paging, artwork, scrolling and animation keep
the shared geometry; Episodes height and the 35% Queue width do not change.

Regression coverage includes action replies before any stream/navigation, previous
and current membership, stale updates, preview caching without subscription,
completion/stop, filtering, pinned reading, EOF, resize, full-width Unicode identity,
plain markers, themes and padding under failure/minimum-size budgets. Human/native
and real Kitty verification remain separate from automated fixtures.
