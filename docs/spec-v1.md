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
- Newest episodes first; aggregate new-episode tab; sort by newest, oldest, or
  title. Preserve selection by episode identity as feeds refresh.
- Refresh at startup, every 30 minutes while running, and manually with R.
- Priorities: ease of use, clean visuals, podcast artwork via Kitty graphics
  with an image-derived ASCII fallback. Design reference: sibling mpdtui.

## Layout

```text
+----------------------+---------------------------+
| Podcasts/subscribed  | Queue                     |
+----------------------+---------------------------+
| Episodes             | Details / art / player    |
+----------------------+---------------------------+
```

Compact layout stacks podcasts, episodes, and player controls. Queue/details
remain accessible as overlays; hide artwork and visualization first as space
shrinks. Exact breakpoints are an implementation detail.

Vim-style navigation: j/k move selection, h/l change panes, J/K reorder queue,
/ filters locally, b switches My subscriptions / Apple Discover, f opens search
in the current scope (Tab switches Library/Apple in the dialog), ? shows help. Space starts the selected
episode in Episodes/Queue, or toggles pause if it is already current. In other
panes Space toggles current playback; in text input it types a space. Enter
selects a podcast or expands episode details; a enqueues; s stops playback.
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
- Controls: play, pause, stop. Queue: persistent, reorderable, batch enqueue;
  no duplicate episode entries. Direct playback preserves the remaining queue.
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
- Subscribe/mark-completed bindings. Full layout starts at 100×24; minimum
  terminal size is 30×12.
- Native macOS media-key approach and its packaging/build requirements.
- Desktop notifications and browser/file actions are not yet committed.
- Storage driver, paths, backup policy, and release support matrix.
- Availability of native macOS GitLab runners.

CLI, responsive TUI, profile ownership, cached library, playback/queue controls,
Apple discovery and library search exist. Artwork UI, decorative visualization,
native macOS media controls, release validation and packaging remain.
