# ADR 014: Fixed details viewport and independent player controls

Status: accepted

## Decision

Keep the wide 2×2 grid. Split its bottom-right cell vertically: upper 50% is
Details, lower 50% is Player (the odd extra row goes to Player). Descriptions are
wrapped, clipped and scrolled only inside Details. PgUp/PgDn move one visible
page; resize clamps the scroll offset. Player updates never append description
rows or reset description scroll.

Lip Gloss v2 Width/Height include borders: pass outer pane dimensions to the
style, reserving two border rows and one title row in the content budget. Both
child panes have explicit heights, independent of metadata length.

Use typed, presentation-only player data: status, current title, elapsed/total,
progress fraction, persistent playback error and optional media warning. Reserve
progress and control rows first; truncate metadata rather than move controls.
Errors take priority over titles; media warnings use extra rows when available.
The progress bar is read-only; this does not add seeking, volume or rate controls.

Pane `5` focuses Player. Left/Right selects Play/Pause, Stop or Next; Enter
activates through the existing shared action callback/playback actor. Disabled
buttons do not dispatch. Space/s/n keep their current keyboard semantics; text
inputs/help/dialogs do not activate controls. Tab/h/l cycle five focus targets.
No second player or new dependency is introduced.

Compact layouts keep a fixed-height player at the bottom. Queue/Details replace
the middle list while focused, never overlay the player. Below 14 terminal rows,
show one focused library pane plus Player instead of squeezing both lists in.
At the 30×12 minimum, omit the progress row before hiding controls. Esc returns
to the last visible library pane. Only explicit help/input dialogs overlay the
layout. This supersedes the compact Queue/Details overlays in ADRs 005/011.
