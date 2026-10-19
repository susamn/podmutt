package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func fixture() Data {
	return Data{
		Podcasts: []Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}, {ID: "c", Title: "Gamma"}},
		Episodes: []Item{{ID: "1", Title: "First episode"}, {ID: "2", Title: "Second episode"}},
		Queue:    []Item{{ID: "2", Title: "Queued episode"}}, Details: "Episode details", Player: Player{Status: "stopped"},
	}
}

func resize(m Model, width, height int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(Model)
}

func keyMsg(key string) tea.KeyPressMsg {
	keys := map[string]tea.Key{
		"tab": {Code: tea.KeyTab}, "shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
		"esc": {Code: tea.KeyEscape}, "enter": {Code: tea.KeyEnter},
		"backspace": {Code: tea.KeyBackspace}, "down": {Code: tea.KeyDown}, "up": {Code: tea.KeyUp},
		"home": {Code: tea.KeyHome}, "end": {Code: tea.KeyEnd},
		"ctrl+u": {Code: 'u', Mod: tea.ModCtrl}, "ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	}
	if k, ok := keys[key]; ok {
		return tea.KeyPressMsg(k)
	}
	return tea.KeyPressMsg(tea.Key{Code: []rune(key)[0], Text: key})
}

func press(m Model, keys ...string) Model {
	for _, key := range keys {
		next, _ := m.Update(keyMsg(key))
		m = next.(Model)
	}
	return m
}

func TestNavigationAndBounds(t *testing.T) {
	m := resize(New(fixture()), 120, 32)
	if m.Init() != nil {
		t.Fatal("unexpected startup command")
	}
	m = press(m, "k", "up", "j", "down", "j")
	if m.selected[Podcasts] != 2 {
		t.Fatal("selection did not clamp at end")
	}
	m = press(m, "g", "G", "home", "end", "k")
	if m.selected[Podcasts] != 1 {
		t.Fatal("first/last navigation failed")
	}
	m = press(m, "l", "tab", "shift+tab", "h", "h")
	if m.focus != PlayerPanel {
		t.Fatalf("focus = %v", m.focus)
	}
	m = press(m, "j", "G", "/", "4", "1", "2", "3")
	if m.focus != Queue {
		t.Fatal("numbered pane navigation failed")
	}
	m = press(m, "l", "l", "tab")
	if m.focus != Podcasts {
		t.Fatal("focus did not wrap")
	}
	empty := press(New(Data{}), "j", "G", "k")
	if empty.selected[Podcasts] != 0 {
		t.Fatal("empty selection is invalid")
	}
}

func TestFilterInputIsIsolatedAndCancelable(t *testing.T) {
	m := resize(New(fixture()), 120, 32)
	m = press(m, "/", "q", " ", "j", "h", "?", "β", "backspace")
	if !m.editing || m.focus != Podcasts || m.filters[Podcasts] != "q jh?" || m.help {
		t.Fatalf("input escaped edit mode: %+v", m)
	}
	m = press(m, "ctrl+u", "backspace", "b", "enter")
	if m.editing || len(m.items(Podcasts)) != 1 || m.items(Podcasts)[0].ID != "b" {
		t.Fatal("filter did not apply")
	}
	m = press(m, "/", "x", "esc")
	if m.filters[Podcasts] != "b" {
		t.Fatal("cancel did not restore previous filter")
	}
	m = press(m, "esc")
	if m.filters[Podcasts] != "" {
		t.Fatal("Esc did not clear applied filter")
	}
}

func TestAsynchronousDataPreservesIdentity(t *testing.T) {
	m := press(New(fixture()), "j")
	data := fixture()
	data.Podcasts = []Item{{ID: "new", Title: "New"}, {ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}
	next, _ := m.Update(data)
	m = next.(Model)
	if m.selected[Podcasts] != 2 {
		t.Fatal("refresh lost selected identity")
	}
	next, _ = m.Update(Data{})
	m = next.(Model)
	if m.selected[Podcasts] != 0 || len(m.items(Podcasts)) != 0 {
		t.Fatal("removed selection did not clamp")
	}
	next, _ = m.Update(fixture()) // restore from empty lists
	if next.(Model).selected[Podcasts] != 0 {
		t.Fatal("restore from empty failed")
	}
	if _, cmd := m.Update(struct{}{}); cmd != nil {
		t.Fatal("unknown message produced command")
	}
}

func TestCompactOverlaysAndHelp(t *testing.T) {
	m := resize(New(fixture()), 70, 20)
	m = press(m, "2", "3")
	if !strings.Contains(ansi.Strip(m.View().Content), "Queued episode") {
		t.Fatal("queue overlay missing")
	}
	m = press(m, "esc")
	if m.focus != Episodes {
		t.Fatal("overlay did not restore visible focus")
	}
	m = press(m, "4")
	if !strings.Contains(ansi.Strip(m.View().Content), "Episode details") {
		t.Fatal("details overlay missing")
	}
	m = press(m, "?", "j", "?")
	if m.help || m.focus != Details {
		t.Fatal("help changed underlying focus")
	}
	m = press(m, "?", "esc")
	if m.help {
		t.Fatal("help did not dismiss")
	}
	m = resize(m, 120, 32)
	if !strings.Contains(ansi.Strip(m.View().Content), "Queue") {
		t.Fatal("expanding did not restore full layout")
	}
}

func TestQuitInEveryMode(t *testing.T) {
	for _, keys := range [][]string{{"q"}, {"?", "q"}, {"/", "ctrl+c"}} {
		m := New(fixture())
		for _, key := range keys[:len(keys)-1] {
			m = press(m, key)
		}
		_, cmd := m.Update(keyMsg(keys[len(keys)-1]))
		if cmd == nil {
			t.Fatal("quit did not return command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("not a quit command")
		}
	}
}

func TestViewsFitTerminal(t *testing.T) {
	data := fixture()
	for i := range 40 {
		data.Podcasts = append(data.Podcasts, Item{ID: fmt.Sprint(i), Title: strings.Repeat("界", 100)})
	}
	for _, size := range [][2]int{{0, 0}, {-1, 2}, {1, 1}, {29, 11}, {30, 12}, {80, 23}, {99, 40}, {100, 24}, {160, 50}} {
		for _, keys := range [][]string{{"G"}, {"?"}, {"3"}, {"4"}, {"/", "a"}} {
			m := press(resize(New(data), size[0], size[1]), keys...)
			view := m.View()
			if !view.AltScreen {
				t.Fatal("alternate screen disabled")
			}
			if view.Content == "" {
				continue
			}
			lines := strings.Split(view.Content, "\n")
			if len(lines) > size[1] {
				t.Fatalf("%v: %d lines exceed height", size, len(lines))
			}
			for _, line := range lines {
				if width := ansi.StringWidth(line); width > size[0] {
					t.Fatalf("%v: line width %d exceeds terminal: %q", size, width, line)
				}
			}
		}
	}
	m := resize(New(Data{}), 120, 32)
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "No items") || !strings.Contains(text, "No episode selected") {
		t.Fatal("empty-state placeholders missing")
	}
	m = resize(New(Data{}), 50, 15)
	if !strings.Contains(ansi.Strip(m.View().Content), "No episode selected") {
		t.Fatal("compact empty player missing")
	}
}

func TestUntrustedTextCannotControlTerminal(t *testing.T) {
	if got := clean("Hello\x1b[31m red\x1b[0m\a\nWorld", false); strings.ContainsAny(got, "\x1b\a\n") {
		t.Fatalf("unsafe text: %q", got)
	}
	if clean("one\ntwo", true) != "one\ntwo" {
		t.Fatal("details lost line breaks")
	}
}

func TestSettingsOverlay(t *testing.T) {
	var dispatched string
	m := New(fixture()).WithSettings(SettingsData{
		ActiveTheme: "colors",
		Themes:      []string{"colors", "solarized-dark", "catppuccin"},
		ConfigPath:  "/test/config.toml",
		ThemePath:   "/test/colors.toml",
		RefreshMin:  30,
	}).WithActions(func(cmd string) tea.Cmd {
		dispatched = cmd
		return nil
	})
	m = resize(m, 100, 30)

	m = press(m, "e")
	if !m.SettingsOpen() || !m.Busy() {
		t.Fatal("settings not open")
	}
	view := m.View().Content
	if !strings.Contains(view, "Settings") || !strings.Contains(view, "solarized-dark") {
		t.Fatalf("settings view missing content: %s", view)
	}

	m = press(m, "j")
	if m.settingsData.Selected != 1 {
		t.Fatalf("expected selected 1, got %d", m.settingsData.Selected)
	}
	m = press(m, "k")
	if m.settingsData.Selected != 0 {
		t.Fatalf("expected selected 0, got %d", m.settingsData.Selected)
	}

	m = press(m, "j")
	m = press(m, "enter")
	if dispatched != "theme:solarized-dark" {
		t.Fatalf("expected action theme:solarized-dark, got %q", dispatched)
	}

	m = press(m, "esc")
	if m.SettingsOpen() {
		t.Fatal("settings did not close on esc")
	}

	m = press(m, "e")
	if !m.SettingsOpen() {
		t.Fatal("settings did not reopen on e")
	}
	m = press(m, "e")
	if m.SettingsOpen() {
		t.Fatal("settings did not close on e")
	}

	m = m.OpenSettings()
	if !m.SettingsOpen() {
		t.Fatal("expected OpenSettings to open")
	}
	m = m.CloseSettings()
	if m.SettingsOpen() {
		t.Fatal("expected CloseSettings to close")
	}
}

func TestUnicodeBoxHeadersAndVersion(t *testing.T) {
	m := New(fixture()).WithIcons("plain").WithVersion("v1.5.0")
	podcastsPane := ansi.Strip(m.pane(Podcasts, 40, 8))
	if !strings.Contains(podcastsPane, "╭──Podcasts") || !strings.Contains(podcastsPane, "[1]─╮") {
		t.Fatalf("unexpected podcasts header: %s", podcastsPane)
	}
	queuePane := ansi.Strip(m.pane(Queue, 50, 8))
	if !strings.Contains(queuePane, "╭──Queue") || !strings.Contains(queuePane, "[3]──v1.5.0─╮") {
		t.Fatalf("unexpected queue header with version: %s", queuePane)
	}
	playerPane := ansi.Strip(m.pane(PlayerPanel, 40, 8))
	if !strings.Contains(playerPane, "╭──Player") || !strings.Contains(playerPane, "[5]─╮") {
		t.Fatalf("unexpected player header: %s", playerPane)
	}
	for _, w := range []int{1, 2, 5, 10, 20} {
		p := m.pane(Queue, w, 5)
		for _, line := range strings.Split(p, "\n") {
			if ansi.StringWidth(line) > w {
				t.Fatalf("line width %d exceeds max width %d: %q", ansi.StringWidth(line), w, line)
			}
		}
	}
}

func TestColumnarLayoutAndGlyphs(t *testing.T) {
	m := New(Data{}).WithIcons("plain")
	epQueued := Item{ID: "1", PlainTitle: "Ep Queued", Channel: "Tech Daily", Date: "2026-04-10", Duration: "42:00", Queued: true}
	epDone := Item{ID: "2", PlainTitle: "Ep Done", Channel: "Tech Daily", Date: "2026-04-09", Duration: "35:10", Completed: true}

	wideEp := ansi.Strip(m.renderEpisodeRow(epQueued, 100, false))
	if !strings.Contains(wideEp, "[+]") || !strings.Contains(wideEp, "Tech Daily") || !strings.Contains(wideEp, "2026-04-10") {
		t.Fatalf("unexpected wide episode row: %s", wideEp)
	}

	doneEp := ansi.Strip(m.renderEpisodeRow(epDone, 100, true))
	if !strings.Contains(doneEp, "> [✓]") || !strings.Contains(doneEp, "35:10") {
		t.Fatalf("unexpected done episode row: %s", doneEp)
	}

	compactEp := ansi.Strip(m.renderEpisodeRow(epQueued, 50, false))
	if !strings.Contains(compactEp, "[+]") || !strings.Contains(compactEp, "42:00") {
		t.Fatalf("unexpected compact episode row: %s", compactEp)
	}

	qPlaying := Item{ID: "1", Index: 1, PlainTitle: "Track Playing", Channel: "Tech Daily", State: "playing", Position: "10:00", Duration: "40:00", Remains: "75%"}
	qPaused := Item{ID: "2", Index: 2, PlainTitle: "Track Paused", Channel: "Tech Daily", State: "paused", Position: "05:00", Duration: "20:00", Remains: "75%"}
	qStopped := Item{ID: "3", Index: 3, PlainTitle: "Track Stopped", Channel: "Other Show", State: "stopped", Position: "00:00", Duration: "15:00", Remains: "100%"}

	qPlayRow := ansi.Strip(m.renderQueueRow(qPlaying, 100, true))
	if !strings.Contains(qPlayRow, "> ▶") || !strings.Contains(qPlayRow, "1   ") || !strings.Contains(qPlayRow, "75%") || !strings.Contains(qPlayRow, "10:00/40:00") {
		t.Fatalf("unexpected queue play row: %s", qPlayRow)
	}

	qPauseRow := ansi.Strip(m.renderQueueRow(qPaused, 100, false))
	if !strings.Contains(qPauseRow, "⏸") || !strings.Contains(qPauseRow, "2   ") || !strings.Contains(qPauseRow, "75%") {
		t.Fatalf("unexpected queue pause row: %s", qPauseRow)
	}

	qStopRow := ansi.Strip(m.renderQueueRow(qStopped, 50, false))
	if !strings.Contains(qStopRow, "■") || !strings.Contains(qStopRow, "3   ") || !strings.Contains(qStopRow, "100%") {
		t.Fatalf("unexpected queue stop row: %s", qStopRow)
	}

	epHeader := ansi.Strip(m.renderEpisodesHeader(100))
	if !strings.Contains(epHeader, "Title") || !strings.Contains(epHeader, "Channel") || !strings.Contains(epHeader, "Date") || !strings.Contains(epHeader, "Duration") {
		t.Fatalf("unexpected episodes header: %s", epHeader)
	}

	qHeader := ansi.Strip(m.renderQueueHeader(100))
	if !strings.Contains(qHeader, "#") || !strings.Contains(qHeader, "Remains") || !strings.Contains(qHeader, "Title") || !strings.Contains(qHeader, "Time") {
		t.Fatalf("unexpected queue header: %s", qHeader)
	}
}

func TestBottomHintsContextual(t *testing.T) {
	m := New(Data{PodcastLabel: "My subscriptions"}).WithTheme(theme.Default())
	subHints := ansi.Strip(m.bottomHints())
	for _, expected := range []string{"f:search", "U:url", "b:discover", "v:view", ",/.:seek"} {
		if !strings.Contains(subHints, expected) {
			t.Fatalf("expected hint %q in subscriptions: %s", expected, subHints)
		}
	}

	mDiscover := New(Data{PodcastLabel: "Discover · Apple"}).WithTheme(theme.Default())
	discHints := ansi.Strip(mDiscover.bottomHints())
	for _, expected := range []string{"f:search", "U:url", "b:library", "t:top25", ",/.:seek"} {
		if !strings.Contains(discHints, expected) {
			t.Fatalf("expected hint %q in discover: %s", expected, discHints)
		}
	}

	mDetails := m.WithFocus(Details)
	detHints := ansi.Strip(mDetails.bottomHints())
	if !strings.Contains(detHints, "j/k:scroll") || !strings.Contains(detHints, "Ctrl-d/u:half-page") {
		t.Fatalf("unexpected details hints: %s", detHints)
	}

	mPlayer := m.WithFocus(PlayerPanel)
	playHints := ansi.Strip(mPlayer.bottomHints())
	if !strings.Contains(playHints, "Left/Right:select") || !strings.Contains(playHints, ",/.:seek") {
		t.Fatalf("unexpected player hints: %s", playHints)
	}
}
