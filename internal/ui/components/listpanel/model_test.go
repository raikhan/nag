package listpanel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
)

func mustKeys(t *testing.T) keybind.Map {
	t.Helper()
	keys, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile defaults: %v", err)
	}
	return keys
}

func separator() reminders.ReminderList { return reminders.ReminderList{Kind: reminders.ListSeparator} }

func normal(id, title string) reminders.ReminderList {
	return reminders.ReminderList{ID: id, Title: title, Kind: reminders.ListNormal}
}

// fixture: a leading separator, the smart lists, the divider between smart
// and regular lists, Work, consecutive separators mid-list, Play, and a
// trailing separator.
func fixture() []reminders.ReminderList {
	return []reminders.ReminderList{
		separator(),
		{ID: reminders.SmartListToday, Title: "Today", Kind: reminders.ListSmart},
		{ID: reminders.SmartListScheduled, Title: "Scheduled", Kind: reminders.ListSmart},
		separator(),
		normal("work", "Work"),
		separator(),
		separator(),
		normal("play", "Play"),
		separator(),
	}
}

func newFixtureModel(t *testing.T) Model {
	t.Helper()
	m := New(40, 12)
	m.SetKeys(mustKeys(t))
	m.SetFocused(true)
	m.SetLists(fixture())
	return m
}

func selectedID(t *testing.T, m Model) (string, bool) {
	t.Helper()
	list, ok := m.SelectedList()
	if !ok {
		return "", false
	}
	return list.ID, true
}

// TestNavigationSkipsSeparators covers plan step 2: selection normalization
// never leaves a separator selected across all movement kinds.
func TestNavigationSkipsSeparators(t *testing.T) {
	m := newFixtureModel(t)

	// Load normalization prefers the following selectable row: the leading
	// separator must not stay selected.
	if id, ok := selectedID(t, m); !ok || id != reminders.SmartListToday {
		t.Fatalf("after SetLists expected Today selected, got %q ok=%v", id, ok)
	}

	// One down from Today lands on Scheduled.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if id, ok := selectedID(t, m); !ok || id != reminders.SmartListScheduled {
		t.Fatalf("expected Scheduled selected, got %q ok=%v", id, ok)
	}

	// One down from Scheduled crosses the divider straight to Work.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if id, ok := selectedID(t, m); !ok || id != "work" {
		t.Fatalf("one down from Scheduled expected Work, got %q ok=%v", id, ok)
	}

	// One up from Work returns to Scheduled.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if id, ok := selectedID(t, m); !ok || id != reminders.SmartListScheduled {
		t.Fatalf("one up from Work expected Scheduled, got %q ok=%v", id, ok)
	}

	// Top lands on a selectable row even though index 0 is a separator.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if id, ok := selectedID(t, m); !ok || id != reminders.SmartListToday {
		t.Fatalf("top expected Today, got %q ok=%v", id, ok)
	}

	// Bottom lands on Play, never the trailing separator.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if id, ok := selectedID(t, m); !ok || id != "play" {
		t.Fatalf("bottom expected Play, got %q ok=%v", id, ok)
	}

	// Half-page down/up never leave a separator selected.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if _, ok := selectedID(t, m); !ok {
		t.Fatal("half page down left a separator selected")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if _, ok := selectedID(t, m); !ok {
		t.Fatal("half page up left a separator selected")
	}

	// Page up/down never leave a separator selected.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	for range 20 {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
		if _, ok := selectedID(t, m); !ok {
			t.Fatal("next page left a separator selected")
		}
	}
	for range 20 {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
		if _, ok := selectedID(t, m); !ok {
			t.Fatal("previous page left a separator selected")
		}
	}

	// Mouse wheel movement crosses the divider too and never rests on a
	// separator.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"}) // Scheduled
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if id, ok := selectedID(t, m); !ok || id != "work" {
		t.Fatalf("wheel down from Scheduled expected Work, got %q ok=%v", id, ok)
	}
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if id, ok := selectedID(t, m); !ok || id != reminders.SmartListScheduled {
		t.Fatalf("wheel up from Work expected Scheduled, got %q ok=%v", id, ok)
	}
}

// TestNavigationWithFilterSkipsSeparators verifies filtered sets remain safe.
func TestNavigationWithFilterSkipsSeparators(t *testing.T) {
	m := newFixtureModel(t)
	m, _ = m.Update(tea.KeyPressMsg{Code: '/'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if !m.Filtering() {
		t.Fatal("expected filtering state")
	}
	// Typing while filtering must not crash and never rest on a separator.
	for range 10 {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		if _, ok := selectedID(t, m); !ok && len(m.list.VisibleItems()) > 0 {
			t.Fatal("filtering left a separator selected")
		}
	}
	// Navigate within the filtered set.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if _, ok := selectedID(t, m); !ok {
		t.Fatal("navigation in filtered set left a separator selected")
	}
}

// TestSeparatorOnlySetIsSafe verifies empty and separator-only item sets.
func TestSeparatorOnlySetIsSafe(t *testing.T) {
	m := New(40, 12)
	m.SetKeys(mustKeys(t))
	m.SetFocused(true)
	m.SetLists([]reminders.ReminderList{separator(), separator()})

	if _, ok := selectedID(t, m); ok {
		t.Fatal("separator-only set must not yield a selected list")
	}
	// Movement is a no-op, not a panic.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if _, ok := selectedID(t, m); ok {
		t.Fatal("separator-only set must not yield a selected list after movement")
	}

	empty := New(40, 12)
	empty.SetKeys(mustKeys(t))
	empty.SetFocused(true)
	empty.SetLists(nil)
	empty, _ = empty.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if _, ok := selectedID(t, empty); ok {
		t.Fatal("empty set must not yield a selected list")
	}
}

// TestListColourDropMarkerAndRowHitTesting covers the three sidebar
// additions: the colour dot per row, the drop-target marker, and the
// content-row mapping mouse clicks rely on.
func TestListColourDropMarkerAndRowHitTesting(t *testing.T) {
	coloured := normal("work", "Work")
	coloured.Color = "#2E7FFA"
	m := New(30, 20)
	m.SetKeys(mustKeys(t))
	m.SetLists([]reminders.ReminderList{coloured, normal("play", "Play")})

	// Only the coloured row carries a dot.
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "● Work") {
		t.Fatalf("colour dot missing:\n%s", view)
	}
	if strings.Contains(view, "● Play") {
		t.Fatalf("a colourless list rendered a dot:\n%s", view)
	}

	// The drop marker appears on the target row only.
	m.SetDropTarget("play")
	view = ansi.Strip(m.View())
	if strings.Count(view, "⤓ drop") != 1 {
		t.Fatalf("drop marker count != 1:\n%s", view)
	}
	if !strings.Contains(view, "⤓ drop") || !strings.Contains(view, "Play") {
		t.Fatalf("drop marker not on the target row:\n%s", view)
	}
	m.SetDropTarget("")
	if strings.Contains(ansi.Strip(m.View()), "⤓ drop") {
		t.Fatal("drop marker survived clearing the target")
	}

	// Row mapping: the title bar and the area past the last item hit
	// nothing, and every item row resolves to its own stable ID.
	if _, ok := m.IDAtContentRow(1); ok {
		t.Fatal("the title bar resolved to a row")
	}
	if _, ok := m.IDAtContentRow(listContentTopRows + 2*2); ok {
		t.Fatal("a row past the last item resolved")
	}
	for i, want := range []string{"work", "play"} {
		for _, row := range []int{listContentTopRows + i*2, listContentTopRows + i*2 + 1} {
			got, ok := m.IDAtContentRow(row)
			if !ok || got != want {
				t.Fatalf("row %d = (%q, %v), want %q", row, got, ok, want)
			}
		}
	}
}
