# ADR 029: Columnar layouts, queued indicators, and channel-consistent colors

Status: implemented for #32.

Episodes (Panel 2) and Queue (Panel 3) present structured columnar data with dedicated
status gutters and consistent channel color accents:

1. **Episodes (Panel 2)**:
   - Gutter: `[+]` when queued, `[✓]` when completed, `[ ]` uncompleted.
   - Title: Episode title, truncated adaptively.
   - Channel: Podcast title, styled with its deterministic palette channel color.
   - Date: Publication date (`YYYY-MM-DD`).
   - Duration: Episode duration (`MM:SS` or `H:MM:SS`).

2. **Queue (Panel 3)**:
   - Gutter: Transport indicator (`▶` bright green playing, `⏸` bright yellow paused,
     `■` bright red stopped) for current track; empty for queued tracks.
   - Index: Queue item number (`#1`, `#2`, ...).
   - Title: Episode title.
   - Channel: Podcast name with consistent channel color.
   - Time: `position/duration`.

3. **Channel Colors**:
   - Every podcast channel maps deterministically to an accent color from the active
     theme palette.
   - The same channel color is used across Podcasts, Episodes, Queue, and Player.

4. **Responsive Bounds**:
   - Layouts under 70 columns omit secondary columns (channel, date) and prioritize title
     and duration, ensuring no text overflows terminal cells.
