package library

import (
	"context"
	"strings"

	"podmutt/internal/store"
)

// Search returns up to 100 cached subscribed-episode matches, including completed
// episodes. Unlike the New tab, it reads the repository's full subscribed scope.
// No network or remote/global episode index is involved; rows keep cache order.
func (s *Service) Search(ctx context.Context, query string) ([]store.Episode, error) {
	query = searchText(query)
	if query == "" || len([]rune(query)) > 128 {
		return nil, nil
	}
	episodes, err := s.repo.Episodes(ctx, 0)
	if err != nil {
		return nil, err
	}
	var matches []store.Episode
	for _, episode := range episodes {
		if strings.Contains(searchText(PlainText(episode.Title)), query) || strings.Contains(searchText(PlainText(episode.Description)), query) {
			matches = append(matches, episode)
			if len(matches) == 100 {
				break
			}
		}
	}
	return matches, nil
}
func searchText(text string) string { return strings.ToLower(strings.Join(strings.Fields(text), " ")) }
