# podmutt v1

Status: core v1 scope agreed; remaining engineering details listed below.

## Experience

- Linux and macOS; Go + Bubble Tea; mpv streams public podcast audio.
- Local-first: show cached subscriptions/episodes immediately. Refresh feeds at
  startup and periodically while the TUI runs; update lists incrementally.
- Discover Apple Podcasts shows online; an empty profile automatically shows
  the non-personalized US top-25 chart. Inspect episodes before subscribing, or
  add RSS/ordinary podcast website URLs. My subscriptions lists subscribed shows
  only; library search covers their names and cached episode titles/descriptions
  (including completed history), never a remote episode index.
- Newest episodes first; explicit New episodes, Continue listening, All episodes
  (subscribed), and show destinations. A quiet, non-selectable Subscriptions
  divider separates views from shows (ADR 042). Highlighting a show always scopes its
  episodes. Continue includes saved unfinished history even after unsubscribe.
  Sort by newest, oldest, or title; preserve episode identity on refresh (ADR 040).
- Refresh at startup, every 30 minutes while running, and manually with R.
- Priorities: ease of use, clean visuals, podcast artwork via Kitty graphics
  with an image-derived ASCII fallback. Design reference: sibling mpdtui.

## Layout

```text
+----------------------+---------------------------+
| Podcasts/subscribed  | Up next (pending only)    |
+----------------------+---------------------------+
| Episodes             | Details (upper 50%)       |
|                      +---------------------------+
|                      | Now playing (lower 50%)   |
+----------------------+---------------------------+
```

The bottom-right cell splits 50/50 between a clipped, scrollable description and
an independent player. Descriptions never enlarge or displace the player.
Compact layouts retain bottom player controls; Queue/Details replace the middle
list while focused. Very short layouts show one library pane plus Player. Hide
artwork/visualization first as space shrinks. See ADR 014.

Vim-friendly navigation: j/k move selection, h/Left goes back, l/Right opens shows
or inspects episodes without playback; J/K reorders queue. Tab cycles the three
lists; 1–5 directly focuses any pane. Ctrl-d/u and PgUp/PgDn page lists. See ADR 039.
/ filters locally, b switches My subscriptions / Apple Discover, f opens search
in the current scope (Tab switches Library/Apple in the dialog), ? shows help.
Space always pauses/resumes current playback; in text input it types a space.
Enter opens a podcast or plays a selected episode; i opens episode details and
Esc returns to the originating list. a enqueues; s stops playback. See ADR 037.
1–5 focus Podcasts/Episodes/Queue/Details/Player. In Player, Left/Right selects
Play/Pause, Stop or Next; Enter activates. The position bar is display-only.
No configurable bindings or themes in v1. Keep contextual hints and help short.

## Playback and state

- Exactly one TUI instance per user profile. Another TUI reports already running.
- No daemon, login service, or detached playback. Quit saves state, stops mpv,
  cancels refreshes, and exits. No app-owned child should remain running.
- Persist current episode, position, subscriptions, history, and queue.
- Reopening restores state without autoplay. Checkpoint position every 5 seconds
  and on pause, stop, episode change, and graceful exit.
- Resume interrupted episodes. Completion means reaching natural EOF or explicit
  user marking, not merely receiving a playback error or pressing Stop.
- At natural EOF advance through the explicit queue; stop when it is empty.
- Controls: play, pause, stop. Up next is persistent, reorderable, batch enqueue;
  no duplicate entries. Current playback stays in the separate Now playing pane,
  never an editable queue row (ADR 041). Direct playback preserves pending work.
  d/Delete removes immediately; u in Up next undoes one session-local removal
  by merging into the live queue. D clear still requires confirmation (ADR 044).
- Playback failure: persistent notification and stop; no automatic retry or skip.
- CLI controls/status target the running TUI for use in external keybindings;
  they must not create a background player when no TUI is running.
- Simple scrolling decorative visualization; real audio analysis is not required
  for v1. No additional audio-analysis dependency. Do not present animation as
  a measured spectrum; freeze it when paused and clear it when stopped.

## Deliberately excluded

Downloads/offline audio, OPML, private/authenticated feeds, SSH support,
accounts/cloud sync, personalized recommendations, transcripts, video, chapters, bookmarks,
speed/seek controls, silence skipping, intro/outro skipping. Offline browsing
of cached metadata is supported. Later answers about download policies are
superseded by the explicit streaming-only scope.

## Engineering constraints

- Small focused commits; concise specs and ADRs. Reuse cohesive code, not
  duplicated logic or speculative abstraction frameworks.
- Prefer Go standard library; justify necessary third-party dependencies.
- One executable with TUI and CLI commands; separate mpv runtime dependency.
  On launch, detect missing mpv and prompt the user to install it with appropriate
  instructions; do not silently install software or request elevated privileges.
- Linux MPRIS/media keys and macOS media keys are required in v1. macOS Now
  Playing metadata beyond what media-key integration needs is not yet committed.
- License: MIT.
- TOML configuration; no telemetry by default; redact secrets from logs.
- Makefile, GitLab CI, automated tests and >=90% aggregate first-party statement
  coverage, including command code. Native macOS CI availability remains open.
- Desired distribution: binaries, source, Homebrew, AUR, Nix. Architecture,
  compatibility, packaging ownership, and signing matrix remain open.
- Database migrations, backups/export, and Linux/macOS testing are desired;
  formats and release acceptance criteria remain open.

## Remaining engineering details

- Apple discovery defaults to the US chart; other-country configuration is deferred.
  See ADR 012 for provider/access terms, limits, and website resolution.
- Current bindings are documented in README (u subscribe, c completion, 1–5
  panes). Full layout starts at 100×24; minimum terminal size is 30×12.
- Native macOS public MediaPlayer bridge is a prototype (ADR 016); its SDK/CGO,
  bundle/signing and physical-key requirements still need native verification.
- Desktop notifications and browser/file actions are not yet committed.
- modernc SQLite/private profile paths are accepted in ADRs 002/003. Consistent
  backup/restore is documented in docs/recovery.md; release support is not approved.
- Native macOS runner recipes are opt-in; host availability and both architecture
  runtime results remain unresolved.

CLI, responsive TUI, profile ownership, cached library, playback/queue controls,
Apple discovery and library search exist. Separate MRs implement artwork UI and
decorative animation; real Kitty verification remains pending. Native macOS controls
are a prototype, not a verified runtime claim. Local/CI workflow and recovery gates
exist, including ordered final checkpoint/reap (ADR 017), but the physical/native
rows in docs/validation-v1.md and release packaging still gate v1 completion.
