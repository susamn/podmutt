# ADR 025: Cell-safe icons are an optional presentation vocabulary

Status: implemented for #26.

One UI vocabulary supplies show, episode, search/discovery, RSS, queue and
playing/paused/stopped/failed glyphs. Standard Unicode is the default without a
patched font. `icons = "nerd"` explicitly selects Nerd Font glyphs; use a Nerd Font
Mono family for single-cell terminals. `icons = "plain"` removes icon prefixes but
keeps all readable labels (it is not a promise to remove existing Unicode borders
or the progress bar). Unknown internal glyphs/modes fall back to text; unknown
configured modes fail with a clear configuration error.

Apply icons during rendering, never to stable item IDs or stored metadata.
Selection, filtering, queue/playback and exposed control labels remain unchanged.
Plain-text buttons retain their compact width; icons accompany state/title rather
than displacing controls. Every icon is supplementary to a textual label.
Untrusted metadata is sanitized before decoration, then clipped with the existing
ANSI/cell-width-aware renderer. No terminal escape or Nerd Font autodetection and
no added dependency. The existing macOS media metadata remains undecorated.

Tests cover all vocabulary entries/modes, configuration/default/error handling,
unknown-mode text fallback, Unicode/CJK/escape sanitization, stable selection and
widths at wide/compact/minimum sizes. Real terminal/font rendering still requires
manual checks; numerical width tests cannot prove every user's font metrics.
