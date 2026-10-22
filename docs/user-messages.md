# User-facing message contract and inventory (#8)

This is the maintained presentation-boundary inventory. The fixed catalog is
`internal/feedback/feedback.go`; producers retain errors/causes internally and
translate them at UI/CLI boundaries. Do not render arbitrary `err.Error()` text.
Podcast titles/descriptions are content, not application copy; existing terminal
escape/control-character sanitization still applies to them.

## Lifecycle and severity

- Foreground errors stay visible in the reserved status row, with `Error · ! details`.
  `!` opens a scrollable explanation; j/k scroll, Esc dismisses. Text input and
  other modals own their keys; close them before opening message details.
- Messages have operation scopes. Successful retry clears that operation only;
  background completion or unrelated success cannot erase a failed user action.
- Feed refresh, local reload and arrival reconciliation failures are background
  warnings. Aggregate feed warnings identify affected shows and retain all feed
  failures; cancellation does not replace a useful prior failure.
- Player failure revisions distinguish a genuinely new failure from repeated
  progress/queue snapshots. Repeated snapshots must not replay a dismissed error
  or replace a more recent action's message.
- Superseded/shutdown cancellation is silent. Explicit Escape cancellation is a
  neutral acknowledgement. Timeouts are not proof that the user is offline.
- Severity is textual, not color-only. Long details remain reachable at 30×12;
  input/cancel keys cannot be displaced by explanatory text. Media warnings also
  expose `!` through their player label without hiding ongoing network activity.

## Inventory

`Code` is the stable error category; `Scope` identifies the attempted operation.
Recovery strings are fixed copy unless composed from bounded, sanitized metadata.
Internal/third-party/OS errors are open-ended, so inventory their boundaries, not
an invented exhaustive list of all possible raw strings.

| Producer / trigger | Category or scope | Presentation and recovery |
| --- | --- | --- |
| `app/discover.go`: Apple/Index search and charts | Provider + search/chart; timeout, network, HTTP, rate_limited, invalid_response | Name the operation, distinguish timeout/connectivity/provider rejection; R retries that operation, f changes search, b opens saved Library. No raw context or response body. |
| Same: Library search | Library search | Searches subscribed show names/cached episode text offline; f retries. Count is actual returned rows; only show the 100-result cap when reached. |
| Same: feed/website preview | Feed preview | Identify preview, not directory search; R retries the selected feed, including direct URLs; U permits a new URL. It must not fetch audio or silently select another provider. |
| `discovery/*`, `feed/fetch.go`, `httpurl/url.go`, `library/preview.go` | input, invalid_url, credentials, authentication, HTTP, invalid_response | Preserve categories, HTTP status and causes. Explain 1–128-character query limits, public HTTP(S) URL requirements, missing Index access, denied/missing source or unreadable publisher data. Input/setup failures do not suggest blind retry. |
| Search/URL/Add dialogs | input validation | Keep invalid input editable; show short, safe inline validation with visible submit/cancel keys. Cancel clears obsolete validation feedback. Do not echo credentials in explanatory text or send invalid URLs to directories. |
| `app/library.go`: cached reads / arrival reconciliation | Load library / Update latest episodes | Background warnings; retain current foreground error and cached view. A successful read/reconciliation clears its corresponding warning. |
| Same: subscription/history changes | Library update; partial | Omit vague “Library updated” noise. If subscription change saved but queue reconciliation failed, say so and reload saved state. Do not report total failure or claim the queue update succeeded. |
| Same: scheduled/manual feed refresh | Feed refresh warning | Name all affected shows and count failures against known subscriptions when meaningful. Cached episodes remain browsable. In Discover, direct user to b then R; in Library, R retries. A successful feed result clears obsolete cycle-start failure. |
| `app/player.go`, `playback/queue*` | Queue edit; queue_changed, no_undo | Confirm captured episode identity, retain current removal/history semantics; stale edits ask to review Queue. Successful removal/undo clears its own error. No undo is neutral. D still only clears pending items. |
| `playback/actions.go`, `seek.go`, `player/*` | Playback/Seek; player_missing, player_failure, seek_unavailable, session_closed | Explain missing mpv/install or mpv_path, source/player failure, loaded-seek readiness or closed session. Do not expose IPC identifiers or falsely claim saved position/stopped audio. Successful transport/seek actions clear resolved errors. |
| `app/controls.go`: optional media backend errors | Media warning | Desktop media keys unavailable; use in-app controls. Nonfatal, separate from playback failure; full warning readable with !. Native/physical backend verification remains separate from fixtures. |
| Same / `app/library.go`: theme and country save | Save theme / Save country | Apply UI-owned settings only after persistence succeeds. On failure, give config-file/disk/permission guidance and retain the prior active setting. Prevent duplicate pending writes; reject stale signal reloads. |
| `profile/config.go`: load/validation | configuration, input | Identify config.toml, supported keys/value types and examples; never echo raw TOML, unknown key names or values. Missing HOME/unsupported platform gets explicit setup guidance. |
| `profile/lock.go`, `app/app.go`, `store/store.go` | already_running, profile, database | Existing instance: use/close that window. Private-file/permission errors: check current-user ownership and private directories, never sudo or weaker privacy checks. Other DB errors get disk/profile guidance, not a universal upgrade instruction. |
| `store/store.go`: schema newer than binary | newer_database | Upgrade podmutt or run the newer binary used previously. Include known DB/supported schema numbers, not a guessed minimum app version. Never advise deleting DB, editing user_version or downgrading data. |
| `cmd/podmutt/main.go`: usage/startup/import/controls | classified CLI boundaries | Safe explanations; discard flag parser output that could echo argument values. Preserve help/version behavior, existing exit codes and successful status JSON. |
| `control/errors.go`, `protocol.go`, `socket.go` | not_running, protocol; remote categories | Start podmutt for absent instance; same-version/restart guidance for incompatible responses; preserve timeout/cancellation. Existing error-string field carries a versioned category trailer. Old strict JSON decoders still accept it; new clients do not English-match or echo untrusted legacy errors. |
| `app/import.go`, `opml/*` | import and per-feed categories | Bounded sanitized names, ordinal fallback instead of raw feed URL; safe per-feed failures. “Processed successfully” includes already-subscribed feeds, not a claim of newly created subscriptions. Receipt failure reports saved subscriptions plus failed bookkeeping. |
| `ui/view.go`, `details.go`, `app/library.go` | empty states / metadata fallbacks | Distinguish empty selection/library/queue from no filter matches. Queue empty points to Episodes, not an ineffective a action in the empty Queue. Preserve Untitled episode, Unknown show, No description and playback/history state labels as useful information, not failures. |
| Discovery/help/onboarding/settings hints | informational copy | Actual Top 100 and selected storefront, filter vs remote search, accurate current panel numbers and context-owned keys. This issue does not add future Favorites/chapters/volume controls or renumber panes. |
| Artwork/ratings/theme fallback | intentionally quiet | Missing artwork uses image/ASCII/logo fallbacks; missing ratings are optional. Built-in/default palette fallback and optional preset-file seeding failure do not block playback/startup. Do not convert harmless degraded decoration into repeated errors. |

## Privacy and diagnostics

- Causes remain unwrap-able for tests/internal diagnosis, not printed as user copy.
  Never add raw config/parser content, auth headers, API key/secret values or URL
  credentials/query tokens to messages, protocol errors or logs. No new telemetry.
- Feedback labels strip terminal escapes/control characters, bound length, and
  replace embedded links. Source attribution strips URL userinfo/query/fragment
  while retaining normal public source identity. Typed input is editable input,
  not diagnostic text; invalid credentials are never dispatched.
- Keep provider errors provider-specific and local failures local. Unknown errors
  get a safe operation-specific fallback plus the project issue URL/version hint;
  do not guess that every failure is offline, an API error, or a DB downgrade.

## Regression gates

`make check` enforces >=95% aggregate first-party statement coverage. Run
`make race`, recovery, real-mpv/null-audio and private-D-Bus tests when touching
these paths. Deterministic fixtures cover HTTP/auth/limits/malformed data,
request gate/backoff/header/body deadlines, cancellation/staleness, redaction,
scoped recovery, settings persistence failures and compact message visibility.
Native macOS and physical media-key acceptance are not implied by Linux CI.

This work is #8 only. Discover stale-while-revalidate (#9), volume (#12), chapters
(#13), relative dates (#14), Favorites (#15), compact Player placement (#16) and
the post-milestone screenshot refresh (#17) are separate work.
