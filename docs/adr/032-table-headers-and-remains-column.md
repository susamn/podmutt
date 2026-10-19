# ADR 032: Pinned table headers and percentage remains in Queue and Episodes

Status: implemented for #35.

Following mpdtui's table design, Episodes (Panel 2) and Queue (Panel 3) feature
distinct column header rows and structured progress reporting:

1. **Table Headers**:
   - Pinned at row 0 of the table data viewport.
   - Rendered with bold font in `LightForeground`.
   - Episodes header: `Title`, `Channel`, `Date`, `Duration`.
   - Queue header: `#` (Index), `Remains`, `Title`, `Channel`, `Time`.

2. **Remains Column**:
   - Placed as the 3rd column in Queue: `[Gutter] [#] [Remains] [Title] [Channel] [Time]`.
   - Displays percentage remaining (e.g. `60%`, unplayed tracks show `100%`).
   - Styled in theme `Cyan`.
   - Title column dynamically shrinks to preserve `Remains` visibility.

3. **Numeric Index**:
   - Queue index uses clean numeric formatting (`1`, `2`, `3`...) instead of `#` prefixes.
