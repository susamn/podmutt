# ADR 027: Interactive Settings overlay and live configuration persistence

Status: implemented for #30.

Following mpdtui's pattern, podmutt provides a keyboard-driven Settings overlay
accessed via `e` (closed via `e` or `Esc`).

The overlay presents:
1. An interactive theme selector listing available themes (`colors`, `solarized-dark`,
   `catppuccin`, and custom `.toml` themes from `themes/`).
   - `j/k` or arrow keys navigate choices.
   - `Enter` immediately applies the selected theme live and persists the choice to
     `config.toml` via `profile.SetTheme`.
2. A configuration summary showing:
   - Active theme file path (`colors.toml` or `themes/<name>.toml`).
   - Configuration file path.
   - Background refresh interval.
   - Icon vocabulary mode.
   - External `mpv` binary path.

The modal is centered and composited over the library without interrupting background
refresh or ongoing playback.
