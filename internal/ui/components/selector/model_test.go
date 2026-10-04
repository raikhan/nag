package selector

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/oronbz/nag/internal/keybind"
)

func testKeys(t *testing.T) keybind.Map {
	t.Helper()
	m, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile defaults: %v", err)
	}
	return m
}

func keyPress(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text}
}

func ctrlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func TestFuzzySelectionAndCancellation(t *testing.T) {
	keys := testKeys(t)
	opts := []Option{
		{ID: "none", Label: "None"},
		{ID: "low", Label: "Low"},
		{ID: "medium", Label: "Medium"},
		{ID: "high", Label: "High"},
	}

	s := New(keys)
	s.SetSize(40, 14)
	s.Open(opts, nil, false)
	if !s.Visible() {
		t.Fatal("chooser not visible after open")
	}

	// Typing reduces the choices to High; the query is not altered by nav keys.
	s, _ = s.Update(keyPress('h', "h"))
	s, _ = s.Update(keyPress('g', "g"))
	if got := s.query.Value(); got != "hg" {
		t.Fatalf("query = %q, want hg", got)
	}
	if _, ok := s.SelectedOption(); !ok {
		t.Fatal("no ranked match for hg")
	}
	if opt, _ := s.SelectedOption(); opt.ID != "high" {
		t.Fatalf("first ranked match = %s, want high", opt.ID)
	}

	// Navigation keys move the cursor without changing the query.
	before := s.query.Value()
	s, _ = s.Update(ctrlKey('j'))
	s, _ = s.Update(ctrlKey('n'))
	s, _ = s.Update(ctrlKey('k'))
	s, _ = s.Update(ctrlKey('p'))
	if s.query.Value() != before {
		t.Fatalf("navigation changed query to %q", s.query.Value())
	}

	// Enter returns the selected option's stable ID and closes.
	s, cmd := s.Update(keyPress(tea.KeyEnter, ""))
	if s.Visible() {
		t.Fatal("chooser still visible after confirm")
	}
	msg, ok := cmd().(SelectedMsg)
	if !ok {
		t.Fatalf("confirm produced %T", cmd())
	}
	if len(msg.IDs) != 1 || msg.IDs[0] != "high" {
		t.Fatalf("confirm IDs = %v, want [high]", msg.IDs)
	}

	// No-match Enter cannot choose anything.
	s = New(keys)
	s.Open(opts, nil, false)
	s, _ = s.Update(keyPress('z', "z"))
	s, cmd = s.Update(keyPress(tea.KeyEnter, ""))
	if cmd != nil {
		t.Fatalf("no-match confirm emitted %v", cmd())
	}
	if !s.Visible() {
		t.Fatal("chooser closed on no-match confirm")
	}

	// Cancel emits no selection.
	s, cmd = s.Update(keyPress(tea.KeyEscape, ""))
	if cmd == nil {
		t.Fatal("cancel produced no command")
	}
	if _, ok := cmd().(CancelledMsg); !ok {
		t.Fatalf("cancel produced %T", cmd())
	}
	if s.Visible() {
		t.Fatal("chooser still visible after cancel")
	}

	// Multi-select remembers hidden choices across query changes.
	s = New(keys)
	s.Open(opts, []string{"low"}, true)
	s, _ = s.Update(keyPress(' ', " "))
	// Resolve what space toggled: with empty query the first item is "None".
	// Toggle it off if it was selected, else on; recompute the committed set
	// through the emitted message below.
	s, _ = s.Update(keyPress('m', "m"))
	s, cmd = s.Update(keyPress(tea.KeyEnter, ""))
	if msg, ok := cmd().(SelectedMsg); ok {
		found := false
		for _, id := range msg.IDs {
			if id == "low" {
				found = true // hidden choice preserved across the query change
			}
		}
		if !found {
			t.Fatalf("multi commit lost hidden low: %v", msg.IDs)
		}
	} else {
		t.Fatalf("multi confirm produced %T", cmd())
	}

	// Empty option lists are inert.
	s = New(keys)
	s.Open(nil, nil, false)
	s, cmd = s.Update(keyPress(tea.KeyEnter, ""))
	if cmd != nil || !s.Visible() {
		t.Fatal("empty chooser confirmed")
	}
	s, _ = s.Update(keyPress(tea.KeyEscape, ""))
}

func TestKeysDoNotLeakIntoQuery(t *testing.T) {
	keys := testKeys(t)
	if !key.Matches(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}, keys.Bind("selector", "down")) {
		t.Fatal("ctrl+j should move the chooser cursor")
	}
	opts := []Option{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}
	s := New(keys)
	s.Open(opts, nil, false)
	s, _ = s.Update(ctrlKey('j'))
	if s.query.Value() != "" {
		t.Fatalf("navigation leaked %q into query", s.query.Value())
	}
}
