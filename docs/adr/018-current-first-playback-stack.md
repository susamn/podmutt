# ADR 018: Current-first playback stack

Status: accepted for post-v1 #20.

The Queue pane includes current playback, not just up-next items. Explicitly
playing another episode checkpoints the interrupted current episode and puts it
at the front of pending playback. Selecting an existing row promotes it without
duplicating it or sorting other rows. Next skips the current episode without
marking completion; EOF completes it and resumes the first pending episode.

The actor retains `State.Current` plus `State.Queue` (pending only), keeping
existing CLI/storage structure. Presentation composes the current, noncompleted
row with pending rows. Current stays pinned at the front; J/K/d edit pending
rows, and clear removes pending rows without stopping current playback. Queue
Enter plays/promotes; j/k only selects. Episode Enter still opens Details.

An atomic Store.Activate transaction updates the current pointer, clears replay
completion and replaces pending identities together after a successful player
load. The interrupted episode's resume point is checkpointed before replacement.
Failure rolls back activation and closes the player without consuming pending
work. No schema change: startup removes legacy current/pending duplicates,
restores stopped, and never starts mpv. A completed last-current pointer remains
available for replay/history but is absent from the queue presentation.

Rows include episode, show, state and position/duration. Feed titles are read with
episode metadata rather than synchronously querying feeds on every UI frame.
Player Next availability uses pending items, not the visible row count. Linux
MPRIS follows that same eligibility while paused/stopped; native macOS alignment
is the independent #19 patch. Clock smoothing/seeking and subscription-driven
arrivals remain separate work items. No dependency added.

Regression evidence: interruption/promotion/Next/EOF positions, atomic rollback,
legacy normalization, stopped restore, current-only Next, queue navigation,
pending-only edits, completed history and display formatting. Native macOS and
physical media verification remain separate from Linux tests/cross-builds.
