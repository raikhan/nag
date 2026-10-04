package reminderpanel

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
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

func groups() []reminders.CompletedGroup {
	return []reminders.CompletedGroup{
		{Label: "Today", Items: []reminders.Reminder{{ID: "t1"}, {ID: "t2"}}},
		{Label: "Yesterday", Items: []reminders.Reminder{{ID: "y1"}}},
	}
}

func groupedModel(t *testing.T) Model {
	t.Helper()
	m := New(40, 12)
	m.SetKeys(mustKeys(t))
	m.SetFocused(true)
	m.SetGroups(groups())
	return m
}

func selectedID(t *testing.T, m Model) string {
	t.Helper()
	r, ok := m.SelectedReminder()
	if !ok {
		t.Fatal("no reminder selected")
	}
	return r.ID
}

func press(m Model, key rune) Model {
	next, _ := m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return next
}

func TestSetGroupsHidesHeadersFromEveryReader(t *testing.T) {
	m := groupedModel(t)
	// Two section headers are interleaved with the three reminders.
	if items := m.list.Items(); len(items) != 5 {
		t.Fatalf("row count = %d, want 2 headers + 3 reminders", len(items))
	}
	// Four rows fit the page, so the second section header ends it and its
	// reminder spills to the next page: headers never surface as targets.
	if want := []string{"t1", "t2"}; !reflect.DeepEqual(m.VisibleIDs(), want) {
		t.Fatalf("VisibleIDs = %v, want %v", m.VisibleIDs(), want)
	}
	// The panel opens on the first reminder, not on the "Today" header.
	if got := selectedID(t, m); got != "t1" {
		t.Fatalf("initial selection = %s, want t1", got)
	}
}

func TestNavigationSkipsGroupHeaders(t *testing.T) {
	m := groupedModel(t)

	m = press(m, 'j')
	if got := selectedID(t, m); got != "t2" {
		t.Fatalf("after down = %s, want t2", got)
	}
	// The next row is the "Yesterday" header; one more step must land on
	// the reminder under it.
	m = press(m, 'j')
	if got := selectedID(t, m); got != "y1" {
		t.Fatalf("after crossing a header = %s, want y1", got)
	}
	m = press(m, 'k')
	if got := selectedID(t, m); got != "t2" {
		t.Fatalf("after moving back up = %s, want t2", got)
	}
}

func TestRefreshKeepsSelectionOnSameReminder(t *testing.T) {
	m := groupedModel(t)
	m = press(m, 'j')

	m.SetGroups(groups())
	if got := selectedID(t, m); got != "t2" {
		t.Fatalf("selection after refresh = %s, want t2", got)
	}
}

func TestHeaderFilterValueIsEmpty(t *testing.T) {
	if got := (Item{Header: "Today"}).FilterValue(); got != "" {
		t.Fatalf("header FilterValue = %q, want empty so filters drop sections", got)
	}
	if got := (Item{Reminder: reminders.Reminder{Title: "bins", Notes: "out"}}).FilterValue(); got != "bins out" {
		t.Fatalf("reminder FilterValue = %q", got)
	}
}
