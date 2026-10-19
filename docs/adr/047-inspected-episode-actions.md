# ADR 047: Actions target the inspected episode

Status: accepted for #50; builds on ADR 045.

Enter plays/resumes, a queues, and c toggles completion on the stable inspected
episode through the same handlers as Episodes. Show shared contextual labels,
including already queued/current and preview completion's subscription requirement.
No permanent action box or new accelerator. Space/global transport still controls
current audio, independently of the inspection target.

Both i and direct 4 pin identity; back restores that identity when still visible.
Preview actions capture its feed/episode identity, never a later row/index. They
may cache/play/queue explicitly but cannot silently subscribe or alter preview
completion. Canceled/replaced/missing targets never fall back to another episode.

Versioned cached reads refresh pinned history even after completion removes an
unsubscribed episode from Continue. Correlate replies by inspection key and keep
RSS metadata/scroll intact; no render-frame database/network reads. Existing
input/modal guards and actor transport ownership remain. General menu/global
shortcut reassignment stay separate. No schema or dependency changes.
