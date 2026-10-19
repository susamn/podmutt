# ADR 035: Modern Cyberpunk Player HUD, full-length slider bar, and CLIAMP visualizer

Status: implemented for #38.

The Player pane (Panel 5) is redesigned with a modern Cyberpunk HUD layout:

1. **Top Row**:
   - Status pill: `● PLAYING` (bright green), `⏸ PAUSED` (bright yellow),
     `■ STOPPED` (muted), or `! FAILED` (bright red).
   - Episode title, truncated adaptively.
   - Channel pill badge: `[ Channel Name ]` styled with the show's consistent palette color.

2. **Full-Length Progress Slider Bar**:
   - Spans the full width of the pane:
     `pos  ──────●──────  pos / dur [-rem]`
   - Features an illuminated playhead knob (`●`) in `BrightCyan`, elapsed track in `Cyan`,
     remaining track in `Muted`, and real-time countdown `[-MM:SS]`.

3. **Directional Transport Chips**:
   - Formatted as modern action chips:
     `[◀◀ ,] 15s      [Play]  [Stop]  [Next]      [.] 15s ▶▶`
   - Active buttons highlight when Player is focused, preserving Left/Right/Enter navigation
     and global shortcuts (`Space`, `s`, `n`, `,`, `.`).

4. **Multi-Band CLIAMP Visualizer**:
   - Multi-band frequency bars using Unicode eighth-blocks (` `, `▂`, `▃`, `▄`, `▅`, `▆`, `▇`, `█`)
     and floating gravity peak caps (`▔`).
   - Vertical color gradient from `Green` (low levels) through `Yellow` (mid) to `Red` (high).
   - Active when playing, frozen when paused, and resting flat when stopped.
   - Compresses smoothly down to compact 3-row layout on short terminals.
