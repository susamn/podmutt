package store

import "strings"

// EpisodeIdentity is the per-feed key used by episode upserts. Preview actions
// use the same rule to resolve a captured selection to its stable stored ID.
func EpisodeIdentity(episode Episode) string {
	if guid := strings.TrimSpace(episode.GUID); guid != "" {
		return "guid:" + guid
	}
	return "url:" + episode.AudioURL
}
