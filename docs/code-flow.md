# Code Flow Architecture Guide

This document provides a map of `podmutt`'s control and data flow, tracing how user actions travel from the terminal through session services, state engines, and back into the UI projection.

---

## 1. System Topology & Layering

```text
[ Terminal / User Input / CLI ]
               │
               ▼
   cmd/podmutt/main.go (CLI Router)
               │
               ▼
   internal/app/app.go (Profile Resolution & Lock)
               │
               ▼
   internal/app/session.go (Service Lifetimes & Resources)
               │
               ▼
  ┌─────────────────────────┐
  │  internal/app/library.go│◄─── [Asynchronous Workers & Signals]
  │  (Bubble Tea Coordinator│
  └────────────┬────────────┘
               │
  ┌────────────┴────────────┬───────────────────────┐
  │                         │                       │
  ▼                         ▼                       ▼
Feature Handlers      Service Layer           UI Projection
(Settings, Artwork,   (Playback Engine,       (internal/ui/model.go,
 Discovery, Playback)  Store, Discovery)       internal/ui/view.go)
```

---

## 2. Core Execution Phases

### A. Bootstrapping & Session Lifetime
1. **Entry Point** (`cmd/podmutt/main.go`):
   - Dispatches `--version`, `--help`, subcommands (`import-opml`), or CLI control RPCs (`podmutt play`, `pause`, `status`).
   - If no arguments are provided, invokes `app.Run(os.Stdin, os.Stdout)`.
2. **Profile & Process Ownership** (`internal/app/app.go` & `internal/profile`):
   - Resolves system paths (`~/.config/podmutt`, `~/.local/state/podmutt`).
   - Acquires the exclusive session lock file `podmutt.lock`.
3. **Session Initialization** (`internal/app/session.go`):
   - Opens the SQLite database (`internal/store/store.go`).
   - Initializes the serialized audio playback engine (`internal/playback/engine.go`) backed by `mpv` (`internal/player/player.go`).
   - Starts the local Unix domain socket control server (`internal/control/socket.go`) and desktop media integration (MPRIS on Linux / MediaPlayer on macOS).

### B. The Bubble Tea Update Loop
`internal/app/library.go` acts as the single application-level Bubble Tea coordinator. Its top-level `Update(msg tea.Msg)` dispatches messages through focused, feature-oriented handlers:

```go
func (m libraryShell) Update(msg tea.Msg) (result tea.Model, command tea.Cmd) {
    defer func() {
        if updated, ok := result.(libraryShell); ok {
            result, command = updated.postUpdate(command)
        }
    }()

    if updated, cmd, handled := m.updateSettings(msg); handled {
        return updated, cmd
    }
    if updated, cmd, handled := m.updateArtwork(msg); handled {
        return updated, cmd
    }
    if updated, cmd, handled := m.updateDiscovery(msg); handled {
        return updated, cmd
    }
    if updated, cmd, handled := m.updatePlaybackEvents(msg); handled {
        return updated, cmd
    }
    if updated, cmd, handled := m.updateLibraryEvents(msg); handled {
        return updated, cmd
    }
    if updated, cmd, handled := m.updateInput(msg); handled {
        return updated, cmd
    }
    return m.delegate(msg)
}
```

### C. Keyboard Routing & Modal Precedence
Keyboard input (`tea.KeyPressMsg`) follows a strict 4-layer precedence model in `handleKey`:
1. **Application Modals & Text Inputs** (`tryModalKey`):
   - URL Inspector (`I`), Chapters Picker (`C`), First-run Onboarding, Queue Removal Confirmation, Add Feed URL (`N`), and Directory Search (`f`/`U`).
   - If active, the modal or text field consumes the keystroke exclusively; global hotkeys (like Space to play/pause) are blocked from mutating playback.
2. **UI Presentation Modals** (`Model.Busy()`):
   - Column filter (`/`), In-app Settings Theme Picker (`e`), Keyboard Help (`?`), and Error Details Viewer (`!`).
   - If active, the key is delegated directly to `ui.Model` to handle text filtering or modal dismissal.
3. **Application Actions** (`handleActionKey`):
   - Global transport controls (`Space` toggle, `,`/`.` seek, `=` / `-` volume).
   - Navigation & mode toggles (`b` browse, `t` top 100, `f` search, `R` refresh).
   - Focused panel operations (`Enter` play/inspect, `a`/`A` queue, `d` delete/unfavorite, `m` actions, `u` subscribe/undo).
4. **Default Presentation Navigation** (`delegate`):
   - Vim directional navigation (`j`/`k`, `h`/`l`), list paging, Tab panel cycling, and numeric panel jumps (`1`–`6`).

### D. Typed UI-to-App Action Contract
Instead of loosely typed magic strings, presentation events flow to the application session via strongly typed `ui.Action` structures (`internal/ui/action.go`):
- `ActionToggle`: Transport play/pause toggle.
- `ActionStop`: Transport stop.
- `ActionNext`: Transport advance to next queued track.
- `ActionSeekForward` / `ActionSeekBackward`: ±15s seeking.
- `ActionSetTheme`: Theme change with theme name payload.
- `ActionCycleCountry`: Discovery chart country cycling.

### E. Post-Update Synchronization (`postUpdate`)
Regardless of which branch, handler, or delegated path runs, the deferred `postUpdate` stage guarantees:
1. **Chapter Guard**: Automatically closes the chapter picker if current playback changed or stopped.
2. **Queue Focus**: Preserves or shifts list focus when queue removals or selections take place.
3. **Artwork Sync** (`syncArt`): Loads cover art asynchronously via Kitty graphics protocol or ASCII fallback.
4. **Presentation Timers**: Batches smooth 1-second position progress ticks and visualizer animation frames without polling when audio is idle.

---

## 3. End-to-End Walkthroughs

### Example 1: Playing an Episode
1. **User Input**: User presses `Enter` on an episode in the Episodes list (`ui.Episodes`).
2. **Key Dispatch**:
   - `Update` routes the key to `updateInput` -> `handleKey`.
   - `handleKey` matches `case "enter": m.playSelected()`.
3. **Asynchronous Command**:
   - `playSelected` schedules background work (`work(ctx, "Starting audio…", ...)`).
   - Calls `engine.PlayEpisode(ctx, id)`.
4. **Playback Engine Execution** (`internal/playback/engine.go`):
   - The engine serializes the request through its request channel.
   - Spawns `mpv` with `--idle=yes` via domain socket JSON IPC (`internal/player/ipc.go`).
   - Loads the audio URL, checkpoints previous listening state in SQLite, and publishes an updated `playback.State`.
5. **State Notification & UI Projection**:
   - The engine sends the updated snapshot through `Updates()` channel.
   - `shell.wait()` receives the snapshot and emits `playbackUpdate`.
   - `updatePlaybackEvents` updates `m.shell` with `withPlayback(state)`, re-renders the library and details, and updates the Player HUD (`internal/ui/player.go`).

### Example 2: Discovering & Refreshing Podcasts
1. **User Action**: User presses `b` (toggle Discover) or `t` (Top 100).
2. **Key Dispatch**:
   - `handleKey` executes `m.searchDirectory("")`.
3. **Cache & Network Fetch** (`internal/app/discover.go`):
   - `searchDirectory` checks SQLite for a saved snapshot (`store.LoadDiscoverSnapshot`).
   - If found, immediately presents saved results (stale-while-revalidate).
   - Issues background work (`trackedWork(ctx, "Updating Apple charts…", ...)`) to query remote Apple or Podcast Index directory.
4. **Result Application**:
   - Directory worker returns `directoryLoaded`.
   - `updateDiscovery` receives `directoryLoaded` and calls `applyDirectory`.
   - Saves results into the local database (`SaveDiscoverSnapshot`) and updates `m.browse.shows`.
   - Triggers customer star rating lookup (`ensureShowRating`).
   - Calls `renderLibrary()` to project shows into `ui.Podcasts`.
