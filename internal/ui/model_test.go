package ui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/messages"
)

// newTestModel builds a root model with a fixed 120x32 terminal size and
// seeded lists/reminders. It never calls Init, creates an EventKit client,
// or executes backend commands.
func newTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m = deliver2(t, m, messages.RemindersLoadedMsg{Reminders: []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha"},
		{ID: "r-query", Title: "Query"},
	}})
	return m
}

func deliver(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	ret, cmd := m.Update(msg)
	next, ok := ret.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected model type %T", ret)
	}
	return next, cmd
}

func TestV2MouseFocusAndModalIsolation(t *testing.T) {
	m := newTestModel(t)
	if m.focusedPanel != PanelLists {
		t.Fatalf("expected lists focused initially, got %v", m.focusedPanel)
	}

	// Left click in the right panel switches focus to reminders.
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: 5, Button: tea.MouseLeft})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("left click on right panel: expected reminders focused, got %v", m.focusedPanel)
	}

	// Release does not switch focus.
	m = deliver2(t, m, tea.MouseReleaseMsg{X: 100, Y: 5, Button: tea.MouseLeft})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("release switched focus to %v", m.focusedPanel)
	}

	// Motion does not switch focus.
	m = deliver2(t, m, tea.MouseMotionMsg{X: 5, Y: 5})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("motion switched focus to %v", m.focusedPanel)
	}

	// Right click does not switch focus.
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseRight})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("right click switched focus to %v", m.focusedPanel)
	}

	// Wheel over the unfocused (left) panel does not switch focus.
	m = deliver2(t, m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	m = deliver2(t, m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("wheel switched focus to %v", m.focusedPanel)
	}

	// With a dialog visible, a click on the other panel neither changes
	// focus nor dismisses the modal.
	m.createListDlg.Show()
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	if !m.createListDlg.Visible() {
		t.Fatal("click dismissed the create-list dialog")
	}
	if m.focusedPanel != PanelReminders {
		t.Fatalf("click with dialog visible changed focus to %v", m.focusedPanel)
	}

	// Same isolation with the help overlay.
	m.createListDlg.Hide()
	m.helpOverlay.Toggle()
	if !m.helpOverlay.Visible() {
		t.Fatal("help overlay did not open")
	}
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	if !m.helpOverlay.Visible() {
		t.Fatal("click dismissed the help overlay")
	}
	if m.focusedPanel != PanelReminders {
		t.Fatalf("click with help visible changed focus to %v", m.focusedPanel)
	}
}

func deliver2(t *testing.T, m Model, msg tea.Msg) Model {
	next, _ := deliver(t, m, msg)
	return next
}

func TestV2KeyPressReleaseAndFiltering(t *testing.T) {
	m := newTestModel(t)

	// A pressed Tab switches panels once.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("tab press: expected reminders focused, got %v", m.focusedPanel)
	}

	// Its release does not switch again.
	m = deliver2(t, m, tea.KeyReleaseMsg{Code: tea.KeyTab})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("tab release switched focus to %v", m.focusedPanel)
	}

	// Shift-Tab switches back.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focusedPanel != PanelLists {
		t.Fatalf("shift-tab: expected lists focused, got %v", m.focusedPanel)
	}

	// Start filtering with /.
	m = deliver2(t, m, tea.KeyPressMsg{Code: '/'})
	if !m.listPanel.Filtering() {
		t.Fatal("pressing / did not start list filtering")
	}

	// Typing q is filter input, not quit.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !m.listPanel.Filtering() {
		t.Fatal("typing q cancelled or left filtering")
	}
	if view := ansi.Strip(m.listPanel.View()); !strings.Contains(view, "Filter: q") {
		t.Fatalf("filter term not rendered; view = %q", view)
	}

	// Escape cancels filtering and returns to normal navigation.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.listPanel.Filtering() {
		t.Fatal("escape did not cancel filtering")
	}
}

func TestV2BackgroundPreservesListState(t *testing.T) {
	m := newTestModel(t)

	// Select the second list item through normal navigation.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if list, ok := m.listPanel.SelectedList(); !ok || list.ID != "query" {
		t.Fatalf("expected second list selected, got %q ok=%v", list.ID, ok)
	}

	// Focus reminders and select the second item there too.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if r, ok := m.reminderPanel.SelectedReminder(); !ok || r.ID != "r-query" {
		t.Fatalf("expected second reminder selected, got %q ok=%v", r.ID, ok)
	}

	// Back to lists, establish filtering with displayed term q.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = deliver2(t, m, tea.KeyPressMsg{Code: '/'})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !m.listPanel.Filtering() {
		t.Fatal("list filtering not active before background replies")
	}

	selList, _ := m.listPanel.SelectedList()
	selRem, _ := m.reminderPanel.SelectedReminder()
	listFiltering := m.listPanel.Filtering()
	remFiltering := m.reminderPanel.Filtering()
	focus := m.focusedPanel
	filterView := ansi.Strip(m.listPanel.View())

	// Open the create-list dialog, then deliver background replies.
	m.createListDlg.Show()
	m = deliver2(t, m, tea.BackgroundColorMsg{Color: color.White})
	m = deliver2(t, m, tea.BackgroundColorMsg{Color: color.Black})

	if !m.createListDlg.Visible() {
		t.Fatal("background reply dismissed the dialog")
	}
	if selList2, _ := m.listPanel.SelectedList(); selList2 != selList {
		t.Fatalf("list selection changed: %q -> %q", selList.ID, selList2.ID)
	}
	if selRem2, _ := m.reminderPanel.SelectedReminder(); selRem2 != selRem {
		t.Fatalf("reminder selection changed: %q -> %q", selRem.ID, selRem2.ID)
	}
	if m.listPanel.Filtering() != listFiltering || !m.listPanel.Filtering() {
		t.Fatalf("list filter state changed: %v -> %v", listFiltering, m.listPanel.Filtering())
	}
	if m.reminderPanel.Filtering() != remFiltering {
		t.Fatalf("reminder filter state changed: %v -> %v", remFiltering, m.reminderPanel.Filtering())
	}
	if m.focusedPanel != focus {
		t.Fatalf("focus changed: %v -> %v", focus, m.focusedPanel)
	}
	if view := ansi.Strip(m.listPanel.View()); view != filterView {
		t.Fatalf("displayed filter changed:\nbefore = %q\nafter  = %q", filterView, view)
	}
}

// TestV2PanelContentFitsWidth guards the separator spill-over: a separator
// item is rendered at the list's full width, so the list must be sized to the
// panel's inner content width (outer width minus the 2 border columns).
func TestV2PanelContentFitsWidth(t *testing.T) {
	m := NewModel(nil)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{Kind: reminders.ListSeparator},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m = deliver2(t, m, messages.RemindersLoadedMsg{Reminders: []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha"},
		{ID: "r-query", Title: "Query"},
	}})

	innerLists := m.layout.ListsWidth - 2
	innerReminders := m.layout.RemindersWidth - 2
	for _, panel := range []struct {
		name    string
		view    string
		maximum int
	}{
		{"lists", m.listPanel.View(), innerLists},
		{"reminders", m.reminderPanel.View(), innerReminders},
	} {
		for i, line := range strings.Split(ansi.Strip(panel.view), "\n") {
			if w := lipgloss.Width(line); w > panel.maximum {
				t.Errorf("%s panel line %d is %d wide, max inner width is %d: %q",
					panel.name, i, w, panel.maximum, line)
			}
		}
	}
}
