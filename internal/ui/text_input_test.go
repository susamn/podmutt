package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTextInputAppendAndLimit(t *testing.T) {
	ti := TextInput{Limit: 5}
	ti.Append("hello world")
	if ti.Value != "hello" {
		t.Fatalf("expected clamped value 'hello', got %q", ti.Value)
	}

	ti2 := TextInput{}
	ti2.Append("a\x00b\x1fc")
	if ti2.Value != "a b c" {
		t.Fatalf("expected control characters sanitized, got %q", ti2.Value)
	}
}

func TestTextInputBackspaceUnicode(t *testing.T) {
	ti := TextInput{Value: "hello 👋世界"}
	ti.Backspace()
	if ti.Value != "hello 👋世" {
		t.Fatalf("expected rune trimmed, got %q", ti.Value)
	}
	ti.Backspace()
	if ti.Value != "hello 👋" {
		t.Fatalf("expected rune trimmed, got %q", ti.Value)
	}
	ti.Backspace()
	if ti.Value != "hello " {
		t.Fatalf("expected multi-byte emoji trimmed, got %q", ti.Value)
	}

	empty := TextInput{}
	empty.Backspace()
	if empty.Value != "" {
		t.Fatalf("expected empty on backspace, got %q", empty.Value)
	}
}

func TestTextInputClear(t *testing.T) {
	ti := TextInput{Value: "some text"}
	ti.Clear()
	if ti.Value != "" {
		t.Fatalf("expected cleared value, got %q", ti.Value)
	}
}

func TestTextInputEditKey(t *testing.T) {
	ti := TextInput{Value: "abc"}
	if !ti.EditKey(tea.KeyPressMsg{Code: tea.KeyBackspace}) || ti.Value != "ab" {
		t.Fatalf("backspace key failed: %q", ti.Value)
	}
	if !ti.EditKey(tea.KeyPressMsg{Code: 'c', Text: "c"}) || ti.Value != "abc" {
		t.Fatalf("text key failed: %q", ti.Value)
	}
	if !ti.EditKey(tea.KeyPressMsg{Code: 'u'}) { // ctrl+u: key string will be ctrl+u in practice, but let's test string representation
	}
	ti.EditKey(tea.KeyPressMsg{Text: "d"})
	if ti.Value != "abcd" {
		t.Fatalf("append failed: %q", ti.Value)
	}
	// ctrl+u
	if !ti.EditKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}) || ti.Value != "" {
		t.Fatalf("ctrl+u failed: %q", ti.Value)
	}
	// non-printable key
	if ti.EditKey(tea.KeyPressMsg{Code: tea.KeyEscape}) {
		t.Fatal("escape key should not be handled by EditKey")
	}
}
