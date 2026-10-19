package ui

// EpisodeActions describes eligibility, not transport. The app binds these
// hints to its shared episode handlers and a stable inspection target.
type EpisodeActions struct {
	Available, Resume, CanComplete bool
	Completed, Queued, Current     bool
}

func (m Model) WithEpisodeActions(actions EpisodeActions) Model {
	m.episodeOps = actions
	return m
}

func (m Model) episodeHints() []hint {
	if !m.episodeOps.Available {
		return nil
	}
	play, queue, complete := "play", "queue", "complete"
	if m.episodeOps.Resume {
		play = "resume"
	}
	if m.episodeOps.Queued {
		queue = "queued"
	}
	if m.episodeOps.Current {
		queue = "current"
	}
	if m.episodeOps.Completed {
		complete = "unfinished"
	}
	if !m.episodeOps.CanComplete {
		complete = "subscribe first"
	}
	return []hint{{"Enter", play}, {"a", queue}, {"c", complete}}
}
