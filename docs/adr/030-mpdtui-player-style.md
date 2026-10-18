# ADR 030: mpdtui-style fixed player pane with transport glyphs and bracketed progress

Status: implemented for #33.

The fixed Player pane (Panel 5) reflects mpdtui's Now Playing styling:

1. **Transport Glyphs**:
   - `▶` in bright green (`BrightGreen`) for playing status.
   - `⏸` in bright yellow (`BrightYellow`) for paused status.
   - `■` in bright red (`BrightRed`) for stopped or failed status.

2. **Track and Channel Presentation**:
   - Displays `Title · Channel`, with the channel name colored in the show's consistent
     palette accent color.

3. **Bracketed Progress Bar**:
   - Rendered as `[[████░░░░]]` using cyan foreground (`Cyan`), combining block progress
     with 8-level fractional sub-character resolution.
   - Elapsed and total duration are displayed alongside the bar (`MM:SS / MM:SS`).

4. **Controls and Notices**:
   - Interactive button bar (`[Play/Pause] [Stop] [Next]`) with reverse highlight on selection.
   - Seek shortcut guidance (`h -15s · l +15s`).
   - Warning and persistent playback failure notices clearly prioritized.
