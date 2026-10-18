# ADR 026: File-based palette theming and presets

Status: implemented for #29.

Podmutt adopts mpdtui's flat `key = "value"` color scheme file structure to enable
desktop theme integration (e.g. Omarchy, Matugen) and built-in presets.

`theme` in `config.toml` configures the active theme name (default `"colors"` on Linux).
- `"colors"` resolves to `colors.toml` in the user's config directory.
- Other names resolve to `themes/<name>.toml` in the user's config directory, falling back
  to built-in palettes (`solarized-dark`, `catppuccin`, or `Default`).
- On first launch under user profile preparation, podmutt seeds `colors.toml`,
  `themes/solarized-dark.toml`, and `themes/catppuccin.toml` if they are absent.
- The TUI process listens for `SIGUSR1` to hot-reload the active theme file live without
  requiring an application restart.
- Channel names map deterministically to consistent categorical colors from the active
  palette across all panels.

Tests verify palette parsing, serialization roundtrip, fallback behavior, file seeding,
signal reload dispatch, and theme configuration validation.
