package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func fixture() Data {
	return Data{
		Podcasts: []Item{{"a", "Alpha"}, {"b", "Beta"}, {"c", "Gamma"}},
		Episodes: []Item{{"1", "First episode"}, {"2", "Second episode"}},
		Queue:    []Item{{"2", "Queued episode"}}, Details: "Episode details", Player: Player{Status: "stopped"},
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
	data.Podcasts = []Item{{"new", "New"}, {"a", "Alpha"}, {"b", "Beta"}}
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
	if !strings.Contains(ansi.Strip(m.View().Content), "3 Queue") {
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
		data.Podcasts = append(data.Podcasts, Item{fmt.Sprint(i), strings.Repeat("界", 100)})
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
