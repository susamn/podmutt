# ADR 005: Responsive terminal shell

Status: proposed in #8.

Use Bubble Tea v2 with Lip Gloss for layout/composition. Reuse Charm's ANSI
utilities (already in the dependency graph) for display-width truncation and
escape removal. No Bubbles widget dependency is needed for these simple lists.

The model accepts immutable Data messages, not database or player handles.
Stable item IDs preserve selection across asynchronous updates. Rendering
sanitizes feed-derived text. All keys are routed centrally; filter input claims
printable keys before global actions, except Ctrl+C.

At >=100 columns and >=24 rows show four panes. Otherwise stack podcasts,
episodes, and player; queue/details become centered overlays with Esc return.
Below 30×12 show resize guidance. Keep focus/selection during resize, remove
artwork/animation before core lists, and test display bounds with wide text.

The app owns config/profile locking around the terminal lifetime. Help/version
skip ownership and setup. This shell starts with honest empty states; library
and playback actions belong to #10/#11, not fixture data in production.
