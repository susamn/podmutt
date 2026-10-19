# ADR 045: Shared episode details card

Status: accepted for #48.

Use one structured card for library, Up next, and RSS preview episodes: title,
show, date/duration/listening state, relevant progress/queue membership, description.
Feed metadata stays canonical; actor/history supplies progress/completion, and
only live pending work supplies queue membership. Resolve cached preview history
by feed plus episode identity, never unrelated current audio or directory fields.

Pin a compact identity header while scrolling the description. Overflow metadata
stays readable in the scroll body; unknown optional fields disappear. Keep a
stable card key so progress/refresh does not reset reading. Explicit inspection
retains its episode until back; playback/selection remain separate.

Reuse existing formatting/sanitization and artwork lifecycle. Card artwork is
small and hidden before constraining short/narrow reading areas. No schema or
dependency changes. Player geometry and contextual actions are separate slices.
