# ADR 043: Distinct Not started and Continue listening views

Status: accepted from user review; supersedes ADR 040's New definition.

Rename New episodes to Not started. Include only subscribed episodes with zero
position and no completion mark. Continue listening remains positive-position,
unfinished history, including unsubscribed explicit playback. All episodes still
contains all cached subscribed episodes, including completed history.

Overlay actor-owned current progress on copied snapshots so the views stay
separate before checkpoints and across stale cached reads. Completion replies
carry an authoritative actor snapshot before history reload, avoiding stale live
state resurrecting completed episodes. Preserve internal view IDs/selections;
no release-date cutoff, schema change, dependency, or autoplay.
