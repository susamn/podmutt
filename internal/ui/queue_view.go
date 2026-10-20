package ui

import (
	"slices"
	"sort"
	"strings"
)

// Queue order belongs to presentation only. Never send this order to playback.
func (m Model) sortedQueue(source []Item) []Item {
	items := slices.Clone(source)
	tails := make(map[string]int, len(m.queueTail))
	for i, id := range m.queueTail {
		tails[id] = i + 1
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if tails[a.ID] != 0 || tails[b.ID] != 0 {
			if tails[a.ID] == 0 {
				return true
			}
			if tails[b.ID] == 0 {
				return false
			}
			return tails[a.ID] < tails[b.ID]
		}
		if m.queueChannel {
			left, right := strings.ToLower(clean(a.Channel, false)), strings.ToLower(clean(b.Channel, false))
			if left != right {
				return left < right
			}
		}
		return a.PublishedAt.After(b.PublishedAt)
	})
	for i := range items {
		items[i].Index = i + 1
	}
	return items
}

func (m Model) ToggleQueueOrder() Model {
	selected, ok := m.Selected(Queue)
	m.queueChannel = !m.queueChannel
	m.queueTail = nil
	if ok {
		m = m.Select(Queue, selected.ID)
	}
	return m
}

func (m Model) WithQueueTail(ids []string) Model {
	selected, ok := m.Selected(Queue)
	for _, id := range ids {
		if !slices.Contains(m.queueTail, id) {
			m.queueTail = append(slices.Clone(m.queueTail), id)
		}
	}
	if ok {
		m = m.Select(Queue, selected.ID)
	}
	return m
}

func (m Model) queueOrderLabel() string {
	if m.queueChannel {
		return "channel / newest"
	}
	return "newest"
}
