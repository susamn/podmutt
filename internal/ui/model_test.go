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
		"left": {Code: tea.KeyLeft}, "right": {Code: tea.KeyRight},
		"pgdown": {Code: tea.KeyPgDown}, "pgup": {Code: tea.KeyPgUp},
		"ctrl+d": {Code: 'd', Mod: tea.ModCtrl},
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
	m = press(m, "tab", "tab", "tab")
	if m.focus != Podcasts {
		t.Fatalf("Tab did not skip supporting panes: %v", m.focus)
	}
	m = press(m, "5", "j", "G", "/", "4", "1", "2", "3")
	if m.focus != Queue {
		t.Fatal("numbered pane navigation failed")
	}
	m = press(m, "tab")
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
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "No items") || !strings.Contains(text, "No current episode") {
		t.Fatal("empty-state placeholders missing")
	}
	m = resize(New(Data{}), 50, 15)
	if !strings.Contains(ansi.Strip(m.View().Content), "No current episode") {
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
	if !strings.Contains(podcastsPane, "╭──Library") || !strings.Contains(podcastsPane, "[1]─╮") {
		t.Fatalf("unexpected podcasts header: %s", podcastsPane)
	}
	queuePane := ansi.Strip(m.pane(Queue, 50, 8))
	if !strings.Contains(queuePane, "╭──Queue · 1") || !strings.Contains(queuePane, "[3]──v1.5.0─╮") {
		t.Fatalf("unexpected queue header with version: %s", queuePane)
	}
	playerPane := ansi.Strip(m.pane(PlayerPanel, 40, 8))
	if !strings.Contains(playerPane, "╭──Now playing") || !strings.Contains(playerPane, "[5]─╮") {
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
	if !strings.Contains(doneEp, "[✓]") || strings.Contains(doneEp, "> ") || !strings.Contains(doneEp, "35:10") {
		t.Fatalf("unexpected done episode row: %s", doneEp)
	}

	compactEp := ansi.Strip(m.renderEpisodeRow(epQueued, 50, false))
	if !strings.Contains(compactEp, "[+]") || !strings.Contains(compactEp, "42:00") {
		t.Fatalf("unexpected compact episode row: %s", compactEp)
	}

	queued := Item{ID: "1", Index: 1, PlainTitle: "Queued episode", Channel: "Tech Daily", Position: "10:00", Duration: "40:00", Remains: "75%"}
	for _, width := range []int{50, 100} {
		row := ansi.Strip(m.renderQueueRow(queued, width, true))
		if strings.Contains(row, "> ") || !strings.Contains(row, "1. ") || strings.Contains(row, "75%") || strings.Contains(row, "10:00/40:00") || !strings.Contains(row, "Queued episode") || !strings.Contains(row, "Tech Daily") {
			t.Fatalf("unexpected pending row: %s", row)
		}
		if strings.ContainsAny(row, "▶⏸■") {
			t.Fatal("pending row has current transport glyph", row)
		}
	}

	epHeader := ansi.Strip(m.renderEpisodesHeader(100))
	if !strings.Contains(epHeader, "Title") || !strings.Contains(epHeader, "Channel") || !strings.Contains(epHeader, "Date") || !strings.Contains(epHeader, "Duration") {
		t.Fatalf("unexpected episodes header: %s", epHeader)
	}

	if pane := ansi.Strip(m.pane(Queue, 100, 10)); strings.Contains(pane, "Remains") {
		t.Fatal("redundant queue table header", pane)
	}
}

func TestRenderRowCharacterization(t *testing.T) {
	m := New(Data{}).WithIcons("plain")
	ep := Item{ID: "1", PlainTitle: "My Episode", Channel: "My Show", Date: "2026-04-12", Duration: "45:00", Queued: false}
	q := Item{ID: "2", Index: 1, PlainTitle: "Queued Title", Channel: "My Show", Position: "05:00", Duration: "50:00", Remains: "90%"}

	for _, width := range []int{60, 80, 120} {
		epRow := ansi.Strip(m.renderEpisodeRow(ep, width, false))
		if !strings.Contains(epRow, "[ ]") || !strings.Contains(epRow, "45:00") {
			t.Fatalf("width %d: unexpected episode row: %q", width, epRow)
		}
		if width >= 70 && (!strings.Contains(epRow, "My Show") || !strings.Contains(epRow, "2026-04-12")) {
			t.Fatalf("width %d: expected columns in episode row: %q", width, epRow)
		}

		qRow := ansi.Strip(m.renderQueueRow(q, width, true))
		if strings.Contains(qRow, "> ") || strings.Contains(qRow, "90%") || !strings.Contains(qRow, "Queued Title") {
			t.Fatalf("width %d: unexpected queue row: %q", width, qRow)
		}
		if width >= 70 && !strings.Contains(qRow, "My Show") {
			t.Fatalf("width %d: expected channel in queue row: %q", width, qRow)
		}
	}
}

func TestBottomHintsContextual(t *testing.T) {
	m := New(Data{PodcastLabel: "My subscriptions"}).WithTheme(theme.Default())
	subHints := ansi.Strip(m.bottomHints())
	for _, expected := range []string{"/:search", "o:sort episodes", "Tab/2:episodes", "f:find podcast"} {
		if !strings.Contains(subHints, expected) {
			t.Fatalf("expected hint %q in subscriptions: %s", expected, subHints)
		}
	}

	mDiscover := New(Data{PodcastLabel: "Discover · Apple"}).WithTheme(theme.Default())
	discHints := ansi.Strip(mDiscover.bottomHints())
	for _, expected := range []string{"/:search list", "f:find podcast", "b:library", "t:top25", ",/.:seek"} {
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

func TestPopupInputBoxStyling(t *testing.T) {
	m := New(Data{}).WithTheme(theme.Default())

	dlg := m.renderDialogModal(&Dialog{
		Title:      "Search: Subscriptions",
		Body:       "Offline search",
		InputLabel: "Query",
		InputValue: "tech news",
		Hint:       "Enter submit",
	}, 60, 15)
	strippedDlg := ansi.Strip(dlg)
	if !strings.Contains(strippedDlg, "Search: Subscriptions") || !strings.Contains(strippedDlg, "Query:  tech news▌") {
		t.Fatalf("unexpected dialog output: %s", strippedDlg)
	}

	prm := m.renderPromptModal("https://example.com/feed.xml", 60, 15)
	strippedPrm := ansi.Strip(prm)
	if !strings.Contains(strippedPrm, "Add feed or podcast website") || !strings.Contains(strippedPrm, "URL:  https://example.com/feed.xml▌") {
		t.Fatalf("unexpected prompt output: %s", strippedPrm)
	}
}

func TestHairlineSpectrumVisualizer(t *testing.T) {
	pal := theme.Default()
	lines2 := globalVisualizer.Render(60, 2, 5.0, "playing", pal)
	if len(lines2) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines2))
	}
	combined := ansi.Strip(strings.Join(lines2, "\n"))
	if !strings.Contains(combined, "│") && !strings.Contains(combined, "╵") && !strings.Contains(combined, "╷") {
		t.Fatalf("expected hairline spectrum characters: %s", combined)
	}

	lines1 := globalVisualizer.Render(60, 1, 5.0, "playing", pal)
	if len(lines1) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines1))
	}

	stopped := ansi.Strip(strings.Join(globalVisualizer.Render(60, 2, 5.0, "stopped", pal), ""))
	if strings.TrimSpace(stopped) != "" {
		t.Fatalf("expected empty visualizer when stopped: %q", stopped)
	}
}
