# ADR 046: Compact player and shared reading geometry

Status: accepted for #49; supersedes the equal split and default HUD decoration.

Reserve six total rows for Now playing; Details gets the rest of the wide lower
cell. Share geometry with scroll/artwork/animation calculations. In compact or
error-constrained layouts, retain identity, timeline and controls before show
metadata; wrap bounded errors/warnings and collapse the extra library pane first.

Order current title, show/state, one elapsed/total/optional-remaining timeline,
then stable transport. Global transport and button capabilities remain unchanged.
Persistent failures belong to current audio, not the inspected description.

Decoration defaults off. Optional `player_decoration = true` reserves two rows
only when spacious, reusing the existing labelled pattern/hairline visualizer.
Hidden or disabled animation cannot keep scheduling ticks. No measured-audio
claims, extra polling, schema change, or dependency. Details actions stay separate.
