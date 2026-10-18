# ADR 005: Responsive terminal shell

Status: accepted in #8; fixed player layout extended by ADR 014.

Use Bubble Tea v2 with Lip Gloss for layout/composition. Reuse Charm's ANSI
utilities (already in the dependency graph) for display-width truncation and
escape removal. No Bubbles widget dependency is needed for these simple lists.

The model accepts immutable Data messages, not database or player handles.
Stable item IDs preserve selection across asynchronous updates. Rendering
sanitizes feed-derived text. All keys are routed centrally; filter input claims
printable keys before global actions, except Ctrl+C.

At >=100 columns and >=24 rows keep the four-cell grid, with the bottom-right
cell split into fixed Details/Player halves (ADR 014). Compact lists stack above
a persistent player; focused Queue/Details replace the middle list. Esc returns
to the previous library pane. Very short layouts show one list plus Player.
Below 30×12 show resize guidance. Keep focus/selection during resize, remove
artwork/animation before core lists, and test display bounds with wide text.

The app owns config/profile locking around the terminal lifetime. Help/version
skip ownership and setup. This shell starts with honest empty states; library
and playback actions belong to #10/#11, not fixture data in production.
