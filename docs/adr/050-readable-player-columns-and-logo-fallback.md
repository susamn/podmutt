# ADR 050: One-line player identity, aligned tables and bundled cover fallback

Status: implemented in !61 / #53; identity budgets, padding and markers amended by ADR 051.

Player identity is a single line: episode title (at most 50 display cells), ` · `,
and channel (at most 20). Narrow layouts shrink both budgets without splitting
Unicode or wrapping. Full RSS metadata stays unchanged in Details. State moves to
the player border; identity, timeline and transport normally need five total rows.
Notices and opt-in decoration keep their shared geometry and priority budgets.

Episodes headers and rows use identical separator/column budgets, reserving Duration
at the right edge first. Channel padding is capped at 20 cells and surplus width
goes to Title; optional Channel/Date fields disappear before duration on narrow
views. Queue gets pinned Title, Channel, Remains headers with the same row geometry,
including multi-digit indices. Header rows are not selection stops; paging reserves
their physical space. Percentage, current-row protection and playback policy do not
change.

Missing/unavailable cover artwork tries episode then show images, then a fresh
bounded decode of the existing `assets/logo.png`. Embed the PNG in standalone builds;
do not alter it or depend on a runtime working directory. Fetch/decode stays async.
Cancellation/deadlines do not become successful fallback replies, and existing
source generations prevent stale logos replacing newer covers. ASCII and probed
Kitty use the same image path. Empty/compact/modal Details hide artwork.

Tests cover identity caps/state/transport, Unicode/sanitization, complete duration
and header alignment, queue scrolling/index gutters, embedded-byte/pixel isolation,
o-network fallback, stale/canceled loads and the existing combined lifecycle gates.
Automated fixtures do not establish native, human or real Kitty usability.
