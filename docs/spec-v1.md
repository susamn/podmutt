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
- Library contains All episodes, a quiet non-selectable Subscriptions heading,
  and subscribed shows only. Highlighting a scope immediately changes Episodes.
  All includes subscribed history plus saved unfinished/current/pending work without
  implicitly subscribing. Episode sort cycles newest, oldest, title. Selection,
  current playback, visual sort and actor priority are independent (ADR 048).
- Refresh at startup, every 30 minutes while running, and manually with R.
- Priorities: ease of use, clean visuals, podcast artwork via Kitty graphics
  with an image-derived ASCII fallback. Design reference: sibling mpdtui.

## Layout

```text
+----------------+----------------------+----------------------+
| Library (1)    | Episodes (2)         | Queue (3)            |
|                |                      | current + pending    |
| All episodes   +----------------------+                      |
| Subscriptions  | Details (4)          |                      |
| shows          +----------------------+----------------------+
|                | Now playing (~7 rows, spanning both columns) |
+----------------+---------------------------------------------+
```

Player spans the bottom of Details and Queue, borrowing height from Details while
Episodes retains its height. Queue takes 35% of the viewport; Episodes and Details
share an equally reduced width. Queue rows include percentage remaining (unknown
is --); current percentage uses Player's clock and pending work uses saved progress.
Player identity uses one line: `Title · Channel`, with no gap padding between
fields. Prefer the full channel, truncating the title only when needed, and retain
two-cell side padding. Top/bottom blank rows yield
before notices/core content or compact pinned identity. State stays in the border,
not as Episodes/Queue title prefixes. Episode rows use `+ Title` when queued and
`◉ Title` otherwise, with one space after the marker. The accented + marks pending
and visible current queue membership immediately, including the authoritative Enter action reply. Episodes headers/rows reserve Duration first and bound
Channel padding. Queue has pinned Title, Channel, Remains headers. Missing covers
use the bundled application PNG after episode/show art; no runtime asset dependency.
Descriptions never enlarge or displace the player. Compact
layouts retain Player above one focused list; Queue/Details replace that list.
Hide artwork/visualization first as space shrinks. Shared geometry governs rendering,
paging, description reading and animation (ADRs 048–051).

Vim-friendly navigation: j/k move selection, h/Left goes back, l/Right opens shows
or inspects episodes without playback. Queue o toggles newest vs channel/newest
presentation without changing actor order; manual J/K moves are removed. Tab cycles the three
lists; 1–5 directly focuses any pane. Ctrl-d/u and PgUp/PgDn page lists. See ADR 039.
/ filters locally, b switches My subscriptions / Apple Discover, f opens search
in the current scope (Tab switches Library/Apple in the dialog), ? shows help.
Space always pauses/resumes current playback; in text input it types a space.
Enter opens a podcast or plays a selected episode; i opens episode details and
Esc returns to the originating list. In Details, Enter/a/c target the pinned
inspected episode (ADR 047); preview completion remains read-only. Space always
controls current audio. a enqueues; s stops playback. See ADR 037.
1–5 focus Podcasts/Episodes/Queue/Details/Player. In Player, Left/Right selects
Play/Pause, Stop or Next; Enter activates. The position bar is display-only.
`=`/`-` adjust shared playback volume by 5 percentage points, clamped to 0–130%;
volume starts at 100% each launch and is not persisted. Text inputs and modals
retain ownership of these keys. In compact layouts, the active pane appears above
Player; wide layout is unchanged. No configurable bindings or themes in v1. Keep
contextual hints and help short.

## Playback and state

- Exactly one TUI instance per user profile. Another TUI reports already running.
- No daemon, login service, or detached playback. Quit saves state, stops mpv,
  cancels refreshes, and exits. No app-owned child should remain running.
- Persist current episode, position, subscriptions, history, and queue.
- App startup explicitly resumes saved unfinished current audio and aligns selections
  once; ordinary progress must not reset browsing. The actor constructor itself
  remains stopped/side-effect-free. Checkpoint every 5 seconds and on pause, stop,
  episode change, and graceful exit.
- Resume interrupted episodes. Completion means reaching natural EOF or explicit
  user marking, not merely receiving a playback error or pressing Stop.
- EOF/Next chooses the newest pending publication, independently of display or
  insertion order; stable persisted order breaks ties. Next does not complete skipped audio.
  Subscription arrivals consider the two latest cached releases, not historical backfill.
- Controls: play, pause, stop, and shared actor-serialized volume adjustment. Volume changes
  never start playback and are applied to mpv when it is created if set beforehand.
  Queue is persistent and supports batch enqueue without duplicates. Presentation
  projects current audio separately from pending
  work. Manual additions append/select/focus Queue without autoplay or subscription;
  visual sort never writes back. Direct playback preserves pending work. Queue d/Delete
  confirms the captured selected episode before removal, including current audio in any
  playback state. Removing current saves/stops and clears its pointer without completion
  or autoplay; Queue u restores the last removal as pending without restarting audio.
  D clear still requires confirmation and only clears pending audio (ADR 053 supersedes
  the single-row removal rules in ADRs 044 and 048).
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
- Makefile, GitLab CI, automated tests and >=95% aggregate first-party statement
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
