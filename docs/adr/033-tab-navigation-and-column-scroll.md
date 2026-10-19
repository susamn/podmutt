# ADR 033: Dedicated Tab panel navigation, horizontal column scrolling, and player seek

Status: implemented for #36.

Following mpdtui's key navigation conventions:

1. **Panel Navigation**:
   - `Tab` and `Shift+Tab` (as well as `1`–`5`) exclusively control panel focus.
   - `h` and `l` are decoupled from panel cycling.

2. **Horizontal Column Scrolling**:
   - In all list panels (Podcasts, Episodes, Queue), `h` and `l` horizontally pan the
     column view (`colOffset`).
   - Table headers and data rows pan in unison, allowing off-screen columns and wide titles
     to be inspected on narrow terminals.

3. **Player Seeking**:
   - `,` (or `<`) seeks backward 15 seconds.
   - `.` (or `>`) seeks forward 15 seconds.
   - Supported in Player panel and as global transport shortcuts when playback is active.
