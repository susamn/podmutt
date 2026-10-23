package ui

import (
	"strings"
	"testing"
	"time"

	"podmutt/internal/chapters"
)

func TestChapterProgressMarkersDenseAndUnknownDuration(t *testing.T) {
	list := []chapters.Chapter{
		{Index: 1, Start: 0, Title: "Intro"},
		{Index: 2, Start: 25, Title: "Middle"},
		{Index: 3, Start: 25.1, Title: "Extra"},
		{Index: 4, Start: 50, Title: "Final"},
	}
	m := New(Data{Player: Player{Title: "Current episode", Channel: "Show", HasCurrent: true, Status: "paused", PositionSeconds: 26, DurationSeconds: 100, Chapters: list}})
	m.width, m.height = 100, 25
	player := m.displayPlayer()
	if !strings.Contains(player.ChapterLabel, "Chapter 3/4 · Extra") {
		t.Fatal(player.ChapterLabel)
	}
	line := m.renderSliderLine(player, 80)
	if strings.Count(line, "│") != 2 {
		t.Fatal("dense boundaries not merged", line)
	}
	if !strings.Contains(m.renderPlayerTopLine(player, 100), "Chapter 3/4") {
		t.Fatal("current chapter hidden")
	}
	player.DurationSeconds = 0
	if strings.Contains(m.renderSliderLine(player, 80), "│") {
		t.Fatal("unknown duration produced markers")
	}
	player = m.displayPlayer()
	if player.ChapterLabel == "" {
		t.Fatal("unknown duration lost chapter name")
	}
}

func TestChapterChangesOnAuthoritativePositionAndClock(t *testing.T) {
	list := []chapters.Chapter{{Index: 1, Start: 0, Title: "Intro"}, {Index: 2, Start: 60, Title: "Discussion"}}
	m := New(Data{Player: Player{HasCurrent: true, Status: "playing", PositionSeconds: 59, DurationSeconds: 100, ClockAt: time.Unix(10, 0), Chapters: list}})
	m.progress.at = time.Unix(11, 0)
	if got := m.displayPlayer().ChapterLabel; !strings.Contains(got, "Discussion") {
		t.Fatal(got)
	}
	m.data.Player.PositionSeconds = 15
	m.data.Player.Status = "paused"
	if got := m.displayPlayer().ChapterLabel; !strings.Contains(got, "Intro") {
		t.Fatal(got)
	}
}
