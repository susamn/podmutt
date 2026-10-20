# ADR 049: Shared bottom player and queue percentages

Status: accepted; implemented for #52.

## Decision

On wide terminals, Now playing spans the bottom of the Details and Queue columns.
It borrows only the bottom of Details; Episodes keeps its existing height and
Library keeps its full-height column. Player normally remains six rows but gains
substantially more horizontal space. Shared geometry budgets notices using this
full width, reserves at least two interior Details rows for pinned identity and
reading, and enables optional decoration only when Details has room to spare.
Rendering, paging, artwork, description scrolling and animation use that geometry.
Compact layouts retain Player above their focused pane unchanged.

Queue increases from 30% to 35% of viewport width. Episodes and Details retain one
shared middle-column width, so each loses exactly the width transferred to Queue.

Queue rows reserve a right-aligned percentage-remaining column before truncating
title/show text. A shared formatter clamps valid samples to 0–100%; unknown duration
or non-finite samples display `--`, never a fabricated 100%. The current row derives
its percentage from the existing bounded Player presentation clock; pending work
uses saved progress. No extra timers, polling, checkpoints, elapsed clocks or queue
mutation. Selection, visual sorting, newest-publication priority, current-row edit
protections, and pinned Details identity remain unchanged.

## Verification

Geometry/identity/transport regressions cover minimum-wide through spacious layouts,
all bundled themes/icon modes, decoration and long warnings/errors. Percentage tests
cover truncation, Unicode/escape safety, invalid/unknown samples, interpolation,
pause, pending-row isolation and resizing. Retain check, race, recovery/private
D-Bus and real-mpv CI gates. Automated fixtures are not human/native validation.
