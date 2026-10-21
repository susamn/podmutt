# ADR 052: Shared asynchronous activity/status row

Status: implemented; visual acceptance by the user remains pending.

Reserve two footer rows at every supported terminal size: activity/status above
key hints. Keep pane geometry independent of whether a task is running; modals
must not obscure either footer. The minimum remains 30×12. The reserved row costs
one content row; minimum Player padding yields before controls/identity, and a
media warning that cannot fit in Player uses the idle status row.

A per-session `internal/activity.Tracker`, carried by context, owns finite-work
lifetimes. Begin/finish are concurrency-safe; cancellation removes a task even
when its worker is still unwinding. Nested operations share their parent's token
instead of double counting (preview → website → RSS, or batch refresh → feed).
Snapshots have monotonic revisions and owned slices. One coalesced wakeup channel
feeds the terminal model; it never blocks workers or starts a polling goroutine.
Application-authored labels only: no query, URL, API credential or error content
is added to the activity tracker.

Foreground work wins over background work, with stable start-order ties. The
widget shows a spinner and label, plus counts for additional operations. Feed
batches expose completed/total counts, including failed attempts; unknown work
never gets a fabricated percentage. Spinner appearance is delayed 150 ms to
avoid flicker, with one context-bound 100 ms presentation timer only while work
exists. Plain icons use ASCII spinner frames. Completion/cancellation clears the
spinner; existing application notices use the same row when idle, including at
compact widths. Existing error translation remains the separate scope of #8.

## Finite-work inventory

| Entry point | Activity ownership |
| --- | --- |
| Apple charts and search; Podcast Index search | App command plus service boundary; includes gate/backoff and decoding |
| RSS/website preview, add feed and subscribe | App command; Resolve/Fetch service fallback for direct callers |
| Startup/periodic/manual feed refresh | Scheduler cycle, one measurable batch; direct Refresh also tracked |
| Podcast ratings | Background command and service fallback |
| Artwork loading/decoding/logo fallback | Background command; Image service fallback |
| Local library reads/search, subscription/history writes | Foreground finite app command |
| Autoqueue reconciliation after refresh | Background finite app command |
| Preview caching and play/enqueue | Foreground finite app command |
| Play/pause/stop/next/seek, queue edits/undo/clear, startup resume | Foreground app command; actor submission also covers CLI/desktop actions |
| Theme/country changes and signal-triggered theme reload | Foreground finite app command |

The nested token prevents command + service instrumentation from producing two
entries. Direct service callers without a session tracker remain unchanged.

Do **not** track idle subscriptions (playback updates, refresh-result waits,
activity updates, signal waits), presentation/progress clocks, terminal capability
probes/raw delivery acknowledgements, or steady-state mpv audio streaming. These
would make the app appear perpetually busy. Periodic checkpointing is internal
playback maintenance, not a separate foreground action. The signal subscription
itself stays idle; only the actual reload is tracked. The pre-TUI startup/cleanup and standalone OPML import have no live TUI in which
to place this widget; CLI import keeps its existing per-feed output.

No network cache policy change (#9), new dependencies, global HTTP interception,
telemetry or runtime configuration changes. Labels describe operations, not proof
of an uncached HTTP request: some operations can complete from local cache and
finish before the spinner appears.

## Verification

Deterministic tests cover each HTTP client, success/cancellation, nested work,
concurrent progress/finish, external playback control, stale snapshots/ticks,
foreground/background priority, delayed animation, compact/wide layouts, modal
visibility, label sanitization, fixed geometry and plain/unicode/nerd icons.

For a visual review, build with `make build` and run `./bin/podmutt` (quit any
other instance using that profile first). Search with `f`, preview with Enter,
refresh with `R`, and switch shows to load artwork/ratings. Inspect the row above
key hints while requests run; try a compact terminal and open Settings/help during
an update. Cached/very fast responses may not display a spinner by design.
Physical/native platform validation remains distinct from automated tests.
