## Highlights

- **Persistent Favorites** — a dedicated Favorites pane independent of the playback queue; mark queued episodes from the actions menu and remove favorites directly.
- **Chapter navigation** — chapter indicators plus a `C` picker for the current playback, with jumps that preserve paused/playing state. Supports Podcasting 2.0, Podlove, and embedded mpv chapters.
- **Apple Discover restoration** — cached chart/search results and view selection appear immediately, then refresh asynchronously; failed refreshes retain saved Apple results and can be retried.
- **Volume hotkeys** — `=` / `-` adjust volume in 5% steps, from 0–130%. Volume starts at 100% on each launch and is not persisted.
- **Relative publication dates** — recent episodes show readable labels such as “Today” and “3 days back.”
- **Compact Player layout** — Player stays below the active pane in compact terminals; the wide layout is unchanged.
- **Clearer dialogs and recovery feedback** — unified Settings, Help, confirmation, and input layouts, with actionable failure guidance.

### Other improvements

- Confirmed Queue episode removal, including current playback, with single-step undo.
- Favorites is pane 5 and Player is pane 6; Help and README keybindings reflect the current layout.
- Discover provider caching follows provider policy; Podcast Index search results are not durably cached.

Audio streaming requires `mpv`.
