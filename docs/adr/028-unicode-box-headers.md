# ADR 028: Unicode box-drawing panel headers and version display

Status: implemented for #31.

Panels feature Unicode box-drawing headers (`╭──Podcasts──────────────────────[1]─╮`)
with integrated panel number indicators (`[1]` to `[5]`) and version stamping on the
top-right panel (Queue).

Details:
- Top border lines embed the panel title on the left and the panel shortcut tag on
  the right (`[1]`–`[5]`).
- The upper-right Queue panel embeds the application release/dev version on the right
  edge (`╭──Queue─────────────────[3]──dev─╮`).
- Focused panels highlight the entire border (corners, horizontal/vertical lines,
  title, and tag) in the active theme's accent color.
- Unfocused panels use the theme's muted color.
- Narrow terminal layouts truncate title text gracefully and omit the secondary version
  string when space is constrained, preserving strict column bounds.
