# podmutt v1

Status: scope agreed in discussion; open decisions below block implementation.

## Experience

- Linux and macOS; Go + Bubble Tea; mpv streams public podcast audio.
- Local-first: show cached subscriptions/episodes immediately. Refresh feeds at
  startup and periodically while the TUI runs; update lists incrementally.
- Discover shows by name, inspect episodes before subscribing, add RSS and
  ordinary podcast website URLs. Global search includes episodes and shows;
  exact local/remote scope remains open.
- Newest episodes first; aggregate new-episode tab; changeable sort order.
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

Compact layout retains podcasts, episodes, and player controls. Exact breakpoints
and access to hidden queue/details are pending.

Vim-style navigation; j/k move selection, h/l change panes. Space plays/resumes;
'a' enqueues an episode. Space selection vs global pause priority is pending.
No configurable bindings or themes in v1. Keep contextual hints and help short.

## Playback and state

- Exactly one TUI instance per user profile. Another TUI reports already running.
- No daemon, login service, or detached playback. Quit saves state, stops mpv,
  cancels refreshes, and exits. No app-owned child should remain running.
- Persist current episode, position, subscriptions, history, and queue.
- Reopening restores state; automatic playback on startup is not decided.
- Resume interrupted episodes. Completion means reaching natural EOF or explicit
  user marking, not merely receiving a playback error or pressing Stop.
- At natural EOF advance through the explicit queue; stop when it is empty.
- Controls: play, pause, stop. Queue: persistent, reorderable, batch enqueue.
- Playback failure: visible notification and stop; retry policy pending.
- CLI controls/status target the running TUI for use in external keybindings;
  they must not create a background player when no TUI is running.
- Simple scrolling spectrum requested; genuine audio analysis vs decorative
  animation, extraction method, dependencies, and fallback remain open.

## Deliberately excluded

Downloads/offline audio, OPML, private/authenticated feeds, SSH support,
accounts/cloud sync, recommendations, transcripts, video, chapters, bookmarks,
speed/seek controls, silence skipping, intro/outro skipping. Offline browsing
of cached metadata is supported. Later answers about download policies are
superseded by the explicit streaming-only scope.

## Engineering constraints

- Small focused commits; concise specs and ADRs. Reuse cohesive code, not
  duplicated logic or speculative abstraction frameworks.
- Prefer Go standard library; justify necessary third-party dependencies.
- One executable with TUI and CLI commands; separate mpv runtime dependency.
- TOML configuration; no telemetry by default; redact secrets from logs.
- Makefile, GitLab CI, automated tests and >=90% statement coverage target.
  Exact coverage scope and native platform CI availability remain open.
- Desired distribution: binaries, source, Homebrew, AUR, Nix. Architecture,
  compatibility, packaging ownership, and signing matrix remain open.
- Database migrations, backups/export, and Linux/macOS testing are desired;
  formats and release acceptance criteria remain open.

## Decisions still needed

1. Search: cached episodes vs arbitrary internet-wide episode text search;
   provider/access terms, credentials, geography, and rate limits.
2. External mpv installation and real spectrum source; optional dependencies.
3. Space behavior, Enter behavior, subscribe/complete actions, compact navigation.
4. Queue duplicates/direct-play semantics; startup resume without autoplay;
   refresh interval, sort choices, and cursor stability during refresh.
5. Linux media keys/MPRIS, macOS Now Playing/media keys, notifications, and
   browser/file actions: which are launch requirements?
6. Storage driver, paths, backup policy; license; release support matrix.

No executable, Makefile, tests, or CI pipeline exists yet.
