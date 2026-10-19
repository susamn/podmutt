# ADR 044: Immediate per-item queue removal with safe undo

Status: accepted for #39; supersedes single-removal confirmation in ADR 011.

d or Delete removes the selected Up next episode immediately. u in Up next
undoes the last successful removal, including on an empty list. Keep u for
subscription actions in Podcasts; advertise contextual d/Del and u first in the
queue footer, including narrow terminals. D clear retains stale-safe confirmation.

The playback actor owns one session-local receipt: removed ID and neighbor IDs,
never a saved queue snapshot. Undo merges into the live queue before a surviving
next neighbor, after a surviving previous neighbor, or at the end. Reorder/new
work and EOF/history survive. Failed edits preserve the receipt. Successful undo,
play/re-add/completion of that item, or explicit clear/replacement invalidates it.
Restore queue state stopped on restart, without restoring the ephemeral receipt.

Serialize UI removal/undo requests; remember u pressed during removal rather
than lose the intent or race it ahead. No new dependencies or schema changes.
A general action menu and opt-in autoqueue policy remain separate work.
