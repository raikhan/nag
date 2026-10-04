package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/messages"
)

func mustCompileDefaults(t *testing.T) keybind.Map {
	t.Helper()
	keys, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile defaults: %v", err)
	}
	return keys
}

// mustEffectiveAceAlphabet computes the label alphabet a raw configured
// alphabet produces against the given compiled bindings.
func mustEffectiveAceAlphabet(t *testing.T, keys keybind.Map, alphabet string) string {
	t.Helper()
	eff, err := keybind.EffectiveAceAlphabet(alphabet, keys)
	if err != nil {
		t.Fatalf("effective ace alphabet for %q: %v", alphabet, err)
	}
	return eff
}

// newTestModel builds a root model with a fixed 120x32 terminal size and
// seeded lists/reminders. It never calls Init, creates an EventKit client,
// or executes backend commands.
func newTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: []reminders.Reminder{
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

// TestHLMovesBetweenPanels covers the vim-style panel moves: h focuses the
// lists pane and l the reminders pane, both idempotent, and neither pages
// the focused list (paging keeps left/right/pgup/pgdn/b/u/f).
func TestHLMovesBetweenPanels(t *testing.T) {
	m := newTestModel(t)
	if m.focusedPanel != PanelLists {
		t.Fatalf("start focus = %v, want PanelLists", m.focusedPanel)
	}

	m = deliver2(t, m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("l from lists: focus = %v, want PanelReminders", m.focusedPanel)
	}
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("l twice: focus = %v, want PanelReminders", m.focusedPanel)
	}

	before, _ := m.reminderPanel.SelectedReminder()
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'h', Text: "h"})
	if m.focusedPanel != PanelLists {
		t.Fatalf("h from reminders: focus = %v, want PanelLists", m.focusedPanel)
	}
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'h', Text: "h"})
	if m.focusedPanel != PanelLists {
		t.Fatalf("h twice: focus = %v, want PanelLists", m.focusedPanel)
	}
	after, _ := m.reminderPanel.SelectedReminder()
	if after.ID != before.ID {
		t.Fatalf("h moved the reminders selection %q -> %q", before.ID, after.ID)
	}

	// Paging still reaches the focused list through the arrow keys.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got, _ := m.reminderPanel.SelectedReminder(); got.ID != after.ID {
		t.Fatalf("right changed the selection %q -> %q", after.ID, got.ID)
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
	if selRem2, _ := m.reminderPanel.SelectedReminder(); !reflect.DeepEqual(selRem2, selRem) {
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
	m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{Kind: reminders.ListSeparator},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: []reminders.Reminder{
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

// TestRemappedListDownDrivesPanel verifies a configured list.down binding
// replaces the default: the new key navigates and the old one does not.
func TestRemappedListDownDrivesPanel(t *testing.T) {
	b := keybind.Defaults()
	b["list"] = map[string][]string{"down": {"ctrl+n"}}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	m := NewModel(nil, keys, mustEffectiveAceAlphabet(t, keys, keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})

	if _, ok := m.listPanel.SelectedList(); !ok {
		t.Fatal("expected a list selected")
	}
	// New key moves down.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if list, _ := m.listPanel.SelectedList(); list.ID != "query" {
		t.Fatalf("ctrl+n should move selection to Query, got %q", list.ID)
	}
	// Old default key is replaced and does nothing.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if list, _ := m.listPanel.SelectedList(); list.ID != "query" {
		t.Fatalf("remapped list.down must replace j; j moved selection to %q", list.ID)
	}
}

// TestRemappedListDownDoesNotLeakIntoFilter verifies that a remapped panel
// navigation key types into the filter input instead of moving the list.
func TestRemappedListDownDoesNotLeakIntoFilter(t *testing.T) {
	b := keybind.Defaults()
	b["list"] = map[string][]string{"down": {"ctrl+n"}}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	m := NewModel(nil, keys, mustEffectiveAceAlphabet(t, keys, keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})

	// Open the filter with '/'.
	m = deliver2(t, m, tea.KeyPressMsg{Code: '/'})
	if !m.listPanel.Filtering() {
		t.Fatal("expected list filter to open")
	}
	// ctrl+n while filtering must type into the filter, not navigate.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if !m.listPanel.Filtering() {
		t.Fatal("ctrl+n while filtering must not clear the filter state")
	}
	view := ansi.Strip(m.listPanel.View())
	if !strings.Contains(view, "n") {
		t.Fatalf("expected 'n' typed into filter input, view:\n%s", view)
	}
}

// TestRemappedNextPanelDrivesRoot verifies a configured global.next_panel
// binding drives panel cycling and the old key is inert.
func TestRemappedNextPanelDrivesRoot(t *testing.T) {
	b := keybind.Defaults()
	b["global"] = map[string][]string{"next_panel": {"ctrl+t"}}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	m := NewModel(nil, keys, mustEffectiveAceAlphabet(t, keys, keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
	}})

	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusedPanel != PanelLists {
		t.Fatalf("remapped next_panel must replace tab; tab cycled to %v", m.focusedPanel)
	}
	m = deliver2(t, m, tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("ctrl+t should focus reminders, got %v", m.focusedPanel)
	}
}

// TestCompiledKeysCarryRemaps asserts the compiled map stored on the model
func aceStart(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := deliver(t, m, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !next.ace.active {
		t.Fatal("pressing z did not enter ace mode")
	}
	return next, cmd
}

// TestAceJumpSidebarSelectsListWithoutActions verifies a sidebar label
// selects and loads that list while the sidebar stays focused and no
// pending focus cycle happens.
func TestAceJumpSidebarSelectsListWithoutActions(t *testing.T) {
	m := newTestModel(t)
	beforeLines := len(strings.Split(ansi.Strip(m.listPanel.View()), "\n"))

	m, _ = aceStart(t, m)
	if got := m.listPanel.VisibleIDs(); len(got) != 2 {
		t.Fatalf("expected 2 sidebar ace targets, got %v", got)
	}
	view := ansi.Strip(m.listPanel.View())
	if !strings.Contains(view, "[a]") || !strings.Contains(view, "[s]") {
		t.Fatalf("expected ace labels rendered, view:\n%s", view)
	}
	if afterLines := len(strings.Split(ansi.Strip(m.listPanel.View()), "\n")); afterLines != beforeLines {
		t.Fatalf("labels must consume width, not height: %d -> %d lines", beforeLines, afterLines)
	}

	// Label s is the second sidebar list (Query). It must not run
	// handleEnter's pendingFocus cycle.
	next, cmd := deliver(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if cmd == nil {
		t.Fatal("sidebar jump should fetch the selected list")
	}
	if next.selectedList == nil || next.selectedList.ID != "query" {
		t.Fatalf("expected Query selected, got %+v", next.selectedList)
	}
	if next.pendingFocus {
		t.Fatal("sidebar ace jump must not set pendingFocus")
	}
	if next.focusedPanel != PanelLists {
		t.Fatalf("sidebar must stay focused, got %v", next.focusedPanel)
	}
	if next.ace.active {
		t.Fatal("ace must exit after a jump")
	}
	if strings.Contains(ansi.Strip(next.listPanel.View()), "[a]") {
		t.Fatal("ace labels were not cleared after the jump")
	}
}

// TestAceJumpReminderSelectsOnly verifies a reminder label moves the
// selection within the reminders panel and performs no other action.
func TestAceJumpReminderSelectsOnly(t *testing.T) {
	m := newTestModel(t)
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = aceStart(t, m)

	next, cmd := deliver(t, m, tea.KeyPressMsg{Code: 'f', Text: "f"})
	if cmd != nil {
		t.Fatal("reminder jump must not trigger any command")
	}
	if r, ok := next.reminderPanel.SelectedReminder(); !ok || r.ID != "r-query" {
		t.Fatalf("expected r-query selected, got %+v ok=%v", r, ok)
	}
	if next.focusedPanel != PanelReminders {
		t.Fatalf("expected reminders focused, got %v", next.focusedPanel)
	}
	if r, _ := next.reminderPanel.SelectedReminder(); r.Completed {
		t.Fatal("ace jump must not complete the reminder")
	}
	if next.ace.active {
		t.Fatal("ace must exit after a jump")
	}
}

// TestAceJumpTwoLetterLabelsPrefixBackspaceCancel exercises multi-letter
// labels, dimming prefix narrowing, invalid extensions, backspace and
// cancellation.
func TestAceJumpTwoLetterLabelsPrefixBackspaceCancel(t *testing.T) {
	m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), "as"), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "l1", Title: "One", Kind: reminders.ListNormal},
		{ID: "l2", Title: "Two", Kind: reminders.ListNormal},
		{ID: "l3", Title: "Three", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "l1", Title: "One", Kind: reminders.ListNormal}

	m, _ = aceStart(t, m)
	// 3 targets with the 2-letter alphabet "as" -> two-letter labels
	// aa, as, sa.
	view := ansi.Strip(m.listPanel.View())
	if !strings.Contains(view, "[aa]") || !strings.Contains(view, "[as]") || !strings.Contains(view, "[sa]") {
		t.Fatalf("expected two-letter labels, view:\n%s", view)
	}

	// Type a: narrows the prefix, no jump yet.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.ace.prefix != "a" {
		t.Fatalf("expected prefix a, got %q", m.ace.prefix)
	}
	if list, _ := m.listPanel.SelectedList(); list.ID != "l1" {
		t.Fatalf("prefix typing must not jump, selection moved to %q", list.ID)
	}

	// Backspace removes the letter, then a full label jumps.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.ace.prefix != "" || !m.ace.active {
		t.Fatalf("backspace must clear the prefix, got %q active=%v", m.ace.prefix, m.ace.active)
	}

	// s starts the prefix for sa only; a second s matches nothing and
	// keeps the old prefix.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.ace.prefix != "s" {
		t.Fatalf("expected prefix s, got %q", m.ace.prefix)
	}
	m = deliver2(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.ace.prefix != "s" || !m.ace.active {
		t.Fatalf("invalid extension must keep prefix s, got %q active=%v", m.ace.prefix, m.ace.active)
	}

	// a completes sa and jumps to the third list.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if list, _ := m.listPanel.SelectedList(); list.ID != "l3" {
		t.Fatalf("label sa should select l3, got %q", list.ID)
	}
	if m.ace.active {
		t.Fatal("ace must exit after the jump")
	}

	// Cancel via esc clears labels and leaves the selection unchanged.
	m, _ = aceStart(t, m)
	before, _ := m.listPanel.SelectedList()
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.ace.active {
		t.Fatal("esc must cancel ace")
	}
	if strings.Contains(ansi.Strip(m.listPanel.View()), "[a") {
		t.Fatal("labels not cleared on cancel")
	}
	if after, _ := m.listPanel.SelectedList(); after.ID != before.ID {
		t.Fatalf("cancel changed the selection: %q -> %q", before.ID, after.ID)
	}
}

// TestAceJumpCapturesBeforeRootActions verifies alphabet letters are ace
// input, not root shortcuts like delete.
func TestAceJumpCapturesBeforeRootActions(t *testing.T) {
	m := newTestModel(t)
	m, _ = aceStart(t, m)
	// d is an ace letter: with four targets it is a full label and jumps,
	// but it must never open the delete dialog.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.confirmDlg.Visible() {
		t.Fatal("d opened the delete dialog while ace was active")
	}
	if m.ace.active {
		t.Fatalf("expected d to complete a full label, prefix=%q", m.ace.prefix)
	}
}

// TestAceJumpExcludesSeparatorsOffScreenAndFilteredRows verifies target
// eligibility at the panel level.
func TestAceJumpExcludesSeparatorsOffScreenAndFilteredRows(t *testing.T) {
	m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	lists := []reminders.ReminderList{
		{ID: "today", Title: "Today", Kind: reminders.ListSmart},
		{Kind: reminders.ListSeparator},
		{ID: "work", Title: "Work", Kind: reminders.ListNormal},
		{Kind: reminders.ListSeparator},
		{Kind: reminders.ListSeparator},
		{ID: "home", Title: "Home", Kind: reminders.ListNormal},
	}
	for i := 0; i < 60; i++ {
		lists = append(lists, reminders.ReminderList{
			ID:    fmt.Sprintf("bulk-%d", i),
			Title: fmt.Sprintf("Bulk %d", i),
			Kind:  reminders.ListNormal,
		})
	}
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: lists})

	ids := m.listPanel.VisibleIDs()
	if len(ids) == 0 || len(ids) >= len(lists) {
		t.Fatalf("expected a paginated slice, got %d of %d", len(ids), len(lists))
	}
	for _, id := range ids {
		if id == "" {
			t.Fatal("separator must not be an ace target")
		}
	}
	// The last list is off the first page: not targetable.
	if m.listPanel.SelectID("bulk-59") {
		t.Fatal("off-screen rows must not be selectable by ID")
	}
	if !m.listPanel.SelectID(ids[0]) {
		t.Fatalf("visible ID %q must be selectable", ids[0])
	}

	// Filtered-out rows are not targets. Bubbles applies filtering via an
	// async cmd, so deliver each keystroke's resulting messages.
	for _, key := range []tea.KeyPressMsg{
		{Code: '/'},
		{Code: 'w', Text: "w"},
		{Code: 'o', Text: "o"},
		{Code: 'r', Text: "r"},
		{Code: 'k', Text: "k"},
	} {
		var cmd tea.Cmd
		m, cmd = deliver(t, m, key)
		m = deliverCmdResults(t, m, cmd)
	}
	if !m.listPanel.Filtering() {
		t.Fatal("expected list filtering to be active")
	}
	filtered := m.listPanel.VisibleIDs()
	if len(filtered) != 1 || filtered[0] != "work" {
		t.Fatalf("filter must narrow ace targets to Work, got %v", filtered)
	}
}

// deliverCmdResults executes a returned command and delivers every message
// it produces, flattening tea.BatchMsg like the runtime would.
func deliverCmdResults(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, c := range msg {
			m = deliverCmdResults(t, m, c)
		}
		return m
	default:
		return deliver2(t, m, msg)
	}
}

// TestAceJumpIgnoresStaleListID verifies a reminder load for another list
// is ignored entirely: no display, loading or focus change.
func TestAceJumpIgnoresStaleListID(t *testing.T) {
	m := newTestModel(t)
	beforeRem := m.reminderPanel.Reminders()
	beforeFocus := m.focusedPanel

	m = deliver2(t, m, messages.RemindersLoadedMsg{
		ListID:    "some-other-list",
		Reminders: []reminders.Reminder{{ID: "intruder", Title: "Intruder"}},
	})
	if got := m.reminderPanel.Reminders(); len(got) != len(beforeRem) {
		t.Fatalf("stale load replaced reminders: %d -> %d", len(beforeRem), len(got))
	}
	if m.focusedPanel != beforeFocus {
		t.Fatalf("stale load changed focus: %v -> %v", beforeFocus, m.focusedPanel)
	}
	if m.pendingFocus {
		t.Fatal("stale load must not consume pendingFocus")
	}

	// A nil selectedList also ignores the result.
	m2 := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), -1)
	m2 = deliver2(t, m2, tea.WindowSizeMsg{Width: 120, Height: 32})
	m2 = deliver2(t, m2, messages.RemindersLoadedMsg{
		Reminders: []reminders.Reminder{{ID: "x", Title: "X"}},
	})
	if got := m2.reminderPanel.Reminders(); len(got) != 0 {
		t.Fatalf("load without a selected list must be ignored, got %d reminders", len(got))
	}
}

// TestAceSurvivesUnchangedRefreshAndCancelsOnRealChanges verifies that a
// background refresh only ends an in-flight ace jump when it actually
// changes the visible rows, and that direct user/terminal events still do.
func TestAceSurvivesUnchangedRefreshAndCancelsOnRealChanges(t *testing.T) {
	lists := []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}
	rows := []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha"},
		{ID: "r-query", Title: "Query"},
	}

	m := newTestModel(t)

	m, _ = aceStart(t, m)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	if m.ace.active {
		t.Fatal("resize must cancel ace")
	}

	m, _ = aceStart(t, m)
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	if m.ace.active {
		t.Fatal("mouse activity must cancel ace")
	}

	// Unchanged polls keep the jump alive.
	m, _ = aceStart(t, m)
	m = deliver2(t, m, messages.TickMsg{})
	if !m.ace.active {
		t.Fatal("auto-refresh tick must retain ace")
	}
	m = deliver2(t, m, delayedRefreshMsg{})
	if !m.ace.active {
		t.Fatal("delayed refresh must retain ace")
	}
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: lists})
	if !m.ace.active {
		t.Fatal("unchanged lists load must retain ace")
	}
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: rows})
	if !m.ace.active {
		t.Fatal("unchanged reminders load must retain ace")
	}

	// Spinner-only messages retain ace.
	m = deliver2(t, m, spinner.TickMsg{ID: m.spinner.ID()})
	if !m.ace.active {
		t.Fatal("spinner tick must retain ace")
	}

	// Real changes to the sidebar cancel the jump.
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: append(append([]reminders.ReminderList{}, lists...),
		reminders.ReminderList{ID: "extra", Title: "Extra", Kind: reminders.ListNormal})})
	if m.ace.active {
		t.Fatal("adding a list must cancel ace")
	}

	m, _ = aceStart(t, m)
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Renamed", Kind: reminders.ListNormal},
		lists[1],
	}})
	if m.ace.active {
		t.Fatal("renaming a list must cancel ace")
	}

	// A changed reminder title cancels the jump.
	m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}
	m, _ = aceStart(t, m)
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha renamed"},
		rows[1],
	}})
	if m.ace.active {
		t.Fatal("changed reminder rows must cancel ace")
	}
}

// TestAceStartWithNoTargetsStaysNormal verifies ace does not activate
// without jumpable rows and reports the reason.
func TestAceStartWithNoTargetsStaysNormal(t *testing.T) {
	m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if m.ace.active {
		t.Fatal("ace must not activate with no targets")
	}
	if !strings.Contains(ansi.Strip(m.statusBar.View()), "No jump targets") {
		t.Fatalf("expected status hint, view:\n%s", ansi.Strip(m.statusBar.View()))
	}
}

// TestAceToggleAndTimeout verifies the configured trigger toggles ace off,
// trigger letters never appear in labels, and the expiry timer honours
// generation ordering instead of wall-clock sleeps.
func TestAceToggleAndTimeout(t *testing.T) {
	// Bindings whose trigger is ["a","ctrl+g"]: bare a leaves the label
	// alphabet, ctrl+g does not remove bare g.
	b := keybind.Defaults()
	b["global"]["ace_jump"] = []string{"a", "ctrl+g"}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	eff := mustEffectiveAceAlphabet(t, keys, keybind.DefaultAceAlphabet)
	if strings.ContainsRune(eff, 'a') {
		t.Fatalf("trigger letter a must not be a label letter: %q", eff)
	}
	m := NewModel(nil, keys, eff, -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}

	// Enter with a, exit with the alternate trigger ctrl+g; selection and
	// labels restore.
	m, _ = aceStartWith(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	view := ansi.Strip(m.listPanel.View())
	if strings.Contains(view, "[a]") {
		t.Fatalf("bare a must never be a label, view:\n%s", view)
	}
	before, _ := m.listPanel.SelectedList()
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if m.ace.active {
		t.Fatal("ctrl+g must toggle ace off")
	}
	if strings.Contains(ansi.Strip(m.listPanel.View()), "[") {
		t.Fatal("labels not cleared after toggle-off")
	}
	if after, _ := m.listPanel.SelectedList(); after.ID != before.ID {
		t.Fatalf("toggle-off changed selection: %q -> %q", before.ID, after.ID)
	}

	// Same trigger toggles off too.
	m, _ = aceStartWith(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.ace.active {
		t.Fatal("a must toggle ace off while active")
	}

	// Tiny alphabet: three targets with "as" produce aa/as/sa; toggling off
	// after a partial prefix must not select anything.
	tiny := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), "as"), -1)
	tiny = deliver2(t, tiny, tea.WindowSizeMsg{Width: 120, Height: 32})
	tiny = deliver2(t, tiny, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "l1", Title: "One", Kind: reminders.ListNormal},
		{ID: "l2", Title: "Two", Kind: reminders.ListNormal},
		{ID: "l3", Title: "Three", Kind: reminders.ListNormal},
	}})
	tiny.selectedList = &reminders.ReminderList{ID: "l1", Title: "One", Kind: reminders.ListNormal}
	tiny, _ = aceStart(t, tiny)
	tv := ansi.Strip(tiny.listPanel.View())
	for _, label := range []string{"[aa]", "[as]", "[sa]"} {
		if !strings.Contains(tv, label) {
			t.Fatalf("missing label %s, view:\n%s", label, tv)
		}
	}
	tiny = deliver2(t, tiny, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if tiny.ace.prefix != "a" {
		t.Fatalf("expected prefix a, got %q", tiny.ace.prefix)
	}
	tiny = deliver2(t, tiny, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if tiny.ace.active {
		t.Fatal("trigger must toggle ace off after a prefix")
	}
	if list, _ := tiny.listPanel.SelectedList(); list.ID != "l1" {
		t.Fatalf("toggle after prefix must not select, got %q", list.ID)
	}
}

// aceStartWith enters ace mode through an arbitrary configured trigger key.
func aceStartWith(t *testing.T, m Model, key tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := deliver(t, m, key)
	if !next.ace.active {
		t.Fatal("trigger key did not enter ace mode")
	}
	return next, cmd
}

// TestAceTimeoutGenerations verifies expiry behaves as a one-shot timer per
// activation with stale messages ignored.
func TestAceTimeoutGenerations(t *testing.T) {
	build := func(t *testing.T, seconds int64) Model {
		t.Helper()
		m := NewModel(nil, mustCompileDefaults(t), mustEffectiveAceAlphabet(t, mustCompileDefaults(t), keybind.DefaultAceAlphabet), seconds)
		m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
		m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
			{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
			{ID: "query", Title: "Query", Kind: reminders.ListNormal},
		}})
		m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}
		return m
	}

	t.Run("matching generation expires", func(t *testing.T) {
		m := build(t, 2)
		m, cmd := aceStart(t, m)
		if cmd == nil {
			t.Fatal("positive timeout must return an expiry command")
		}
		gen := m.aceGeneration
		m = deliver2(t, m, aceTimeoutMsg{generation: gen})
		if m.ace.active {
			t.Fatal("matching timeout must exit ace")
		}
	})

	t.Run("stale generation after re-entry ignored", func(t *testing.T) {
		m := build(t, 2)
		m, _ = aceStart(t, m)
		stale := m.aceGeneration
		m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.ace.active {
			t.Fatal("esc must exit ace")
		}
		m, _ = aceStart(t, m)
		if m.aceGeneration == stale {
			t.Fatal("generation must advance on re-entry")
		}
		m = deliver2(t, m, aceTimeoutMsg{generation: stale})
		if !m.ace.active {
			t.Fatal("stale timeout must not cancel the new snapshot")
		}
		m = deliver2(t, m, aceTimeoutMsg{generation: m.aceGeneration})
		if m.ace.active {
			t.Fatal("matching timeout must exit ace")
		}
	})

	t.Run("zero expires immediately without stale activation", func(t *testing.T) {
		m := build(t, 0)
		m, cmd := aceStart(t, m)
		if cmd == nil {
			t.Fatal("zero timeout must still schedule the expiry command")
		}
		m = deliver2(t, m, aceTimeoutMsg{generation: m.aceGeneration})
		if m.ace.active {
			t.Fatal("zero timeout must leave normal mode")
		}
	})

	t.Run("negative disables expiry but not invalidation", func(t *testing.T) {
		m := build(t, -1)
		m, cmd := aceStart(t, m)
		if cmd != nil {
			t.Fatal("disabled timeout must not schedule an expiry command")
		}
		if !m.ace.active {
			t.Fatal("ace should be active")
		}
		m = deliver2(t, m, aceTimeoutMsg{generation: m.aceGeneration})
		if !m.ace.active {
			t.Fatal("with -1 no expiry message should ever cancel ace")
		}
		// Ordinary invalidating messages still exit.
		m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
		if m.ace.active {
			t.Fatal("resize must still exit ace with -1")
		}
	})
}

// TestV2SyncRefreshPreservesSelection covers the 2s server-side sync: an
// identical refresh skips the panel write entirely, and a changed refresh
// keeps the selection on the same reminder ID.
func TestV2SyncRefreshPreservesSelection(t *testing.T) {
	m := newTestModel(t)
	if !m.reminderPanel.SelectID("r-query") {
		t.Fatal("could not select r-query")
	}
	identical := []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha"},
		{ID: "r-query", Title: "Query"},
	}
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: identical})
	if m.displayedListID != "alpha" {
		t.Fatalf("displayedListID = %q", m.displayedListID)
	}
	if r, ok := m.reminderPanel.SelectedReminder(); !ok || r.ID != "r-query" {
		t.Fatalf("identical refresh moved selection to %v", r)
	}

	// First item deleted server-side: the same reminder stays selected.
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: identical[1:]})
	if r, ok := m.reminderPanel.SelectedReminder(); !ok || r.ID != "r-query" {
		t.Fatalf("deletion shifted selection to %v", r)
	}
}

// TestV2SidebarNavigationLoadsListWithoutEnter covers the right panel
// following sidebar navigation: moving down loads the highlighted list and
// Enter on the open list reloads and requests focus.
func TestV2SidebarNavigationLoadsListWithoutEnter(t *testing.T) {
	keys := mustCompileDefaults(t)
	m := NewModel(nil, keys, mustEffectiveAceAlphabet(t, keys, keybind.DefaultAceAlphabet), -1)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	if m.selectedList == nil || m.selectedList.ID != "alpha" {
		t.Fatalf("startup did not load the first list: %v", m.selectedList)
	}

	// Sidebar navigation loads the highlighted list without Enter.
	m, cmd := deliver(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd == nil {
		t.Fatal("navigation did not emit a fetch command")
	}
	if m.selectedList == nil || m.selectedList.ID != "query" {
		t.Fatalf("selection = %v, want query", m.selectedList)
	}

	// Navigating within the open list does not refetch.
	_, cmd = deliver(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil {
		t.Fatal("staying on the open list refetched")
	}

	// Enter on the open list reloads and moves focus once loaded.
	m, cmd = deliver(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter did not reload the open list")
	}
	if !m.pendingFocus {
		t.Fatal("enter did not set pendingFocus")
	}
}

// TestEnterEditsReminderFromRemindersPane verifies Enter is the edit key for
// the highlighted reminder while the reminders pane holds focus.
func TestEnterEditsReminderFromRemindersPane(t *testing.T) {
	m := newTestModel(t)
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusedPanel != PanelReminders {
		t.Fatalf("focus = %v, want PanelReminders", m.focusedPanel)
	}
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.createDlg.Visible() {
		t.Fatal("Enter on a reminder must open the edit form")
	}

	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'e', Text: "e"})
	if !m.createDlg.Visible() {
		t.Fatal("e on a reminder must open the edit form")
	}
}

// listRowY maps an item index to the terminal row rendering it: the panel
// border occupies row 0, the Bubbles title bar two content rows, and every
// item is two rows tall.
func listRowY(i int) int { return 3 + i*2 }

// stepClock advances 100ms per call, so consecutive calls always land
// inside the double-click window; slowClock advances 5s so they never do.
func steppingClock(step time.Duration) func() time.Time {
	base := time.Date(2026, time.October, 4, 9, 0, 0, 0, time.UTC)
	n := 0
	return func() time.Time {
		n++
		return base.Add(time.Duration(n) * step)
	}
}

// TestMouseSelectsRowsAndDoubleClickOpensEditor verifies a click picks the
// row under the cursor, the sidebar click loads that list, and a second
// click on the same row inside the double-click window opens the editor.
func TestMouseSelectsRowsAndDoubleClickOpensEditor(t *testing.T) {
	m := newTestModel(t)

	// A click on the second sidebar row loads that list.
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: listRowY(1), Button: tea.MouseLeft})
	if m.selectedList == nil || m.selectedList.ID != "query" {
		t.Fatalf("click loaded list %v, want query", m.selectedList)
	}
	if l, _ := m.listPanel.SelectedList(); l.ID != "query" {
		t.Fatalf("sidebar selection = %q, want query", l.ID)
	}

	// A click on a reminder row selects it without opening anything.
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(1), Button: tea.MouseLeft})
	if r, ok := m.reminderPanel.SelectedReminder(); !ok || r.ID != "r-query" {
		t.Fatalf("reminder selection = %+v, want r-query", r)
	}
	if m.createDlg.Visible() {
		t.Fatal("a single click must not open the editor")
	}

	// A click on the panel border hits no row.
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: 0, Button: tea.MouseLeft})
	if m.createDlg.Visible() {
		t.Fatal("a click on the border opened the editor")
	}

	// Two clicks on the same row inside the window open the editor.
	m.clock = steppingClock(100 * time.Millisecond)
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(0), Button: tea.MouseLeft})
	if m.createDlg.Visible() {
		t.Fatal("the first click of a double click already opened the editor")
	}
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(0), Button: tea.MouseLeft})
	if !m.createDlg.Visible() {
		t.Fatal("double click did not open the reminder editor")
	}
	if r, ok := m.reminderPanel.SelectedReminder(); !ok || r.ID != "r-alpha" {
		t.Fatalf("double click opened the wrong row: %+v", r)
	}

	// Two clicks on the same list row open the list editor.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m.clock = steppingClock(100 * time.Millisecond)
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: listRowY(1), Button: tea.MouseLeft})
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: listRowY(1), Button: tea.MouseLeft})
	if !m.createListDlg.Visible() {
		t.Fatal("double click did not open the list editor")
	}

	// The same two clicks, too far apart, stay single clicks.
	m = deliver2(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m.clock = steppingClock(5 * time.Second)
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(1), Button: tea.MouseLeft})
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(1), Button: tea.MouseLeft})
	if m.createDlg.Visible() {
		t.Fatal("slow clicks must not count as a double click")
	}
}

// newDragTestModel seeds the sidebar with smart lists and the separator so
// drop-target eligibility is exercised, and reminders that already know
// their list.
func newDragTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil, mustCompileDefaults(t), "", -1)

	// Clicks far apart in time, so a press/release pair never reads as a
	// double click and swallows the drag.
	m.clock = steppingClock(5 * time.Second)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: reminders.SmartListToday, Title: "Today", Kind: reminders.ListSmart},
		{ID: reminders.SmartListScheduled, Title: "Scheduled", Kind: reminders.ListSmart},
		{Kind: reminders.ListSeparator},
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal}
	m = deliver2(t, m, messages.RemindersLoadedMsg{ListID: "alpha", Reminders: []reminders.Reminder{
		{ID: "r-alpha", Title: "Alpha", ListID: "alpha"},
		{ID: "r-two", Title: "Two", ListID: "alpha"},
	}})
	return m
}

// TestReminderDragMovesToDroppedList verifies a press, a held-button move
// over another list and a release issue the move, while a press and release
// without motion, a drop on a smart list and a drop on the source list do
// not.
func TestReminderDragMovesToDroppedList(t *testing.T) {
	const (
		todayRow   = 0
		separator  = 2
		alphaRow   = 3
		queryRow   = 4
		reminderRW = 1
	)

	m := newDragTestModel(t)
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(reminderRW), Button: tea.MouseLeft})

	// A held-button motion over a smart list never becomes a drop target.
	m = deliver2(t, m, tea.MouseMotionMsg{X: 5, Y: listRowY(todayRow), Button: tea.MouseLeft})
	if m.dropID != "" {
		t.Fatalf("smart list accepted a drop: %q", m.dropID)
	}
	if !m.dragActive {
		t.Fatal("held-button motion did not start a drag")
	}

	// The reminder row is marked as lifted while dragging.
	if !strings.Contains(m.reminderPanel.View(), "⠿") {
		t.Fatal("dragged reminder row is not marked as lifted")
	}

	// Moving over a normal list marks it.
	m = deliver2(t, m, tea.MouseMotionMsg{X: 5, Y: listRowY(queryRow), Button: tea.MouseLeft})
	if m.dropID != "query" {
		t.Fatalf("drop target = %q, want query", m.dropID)
	}
	if !strings.Contains(m.listPanel.View(), "⤓ drop") {
		t.Fatal("drop target row carries no marker")
	}

	// Releasing there issues the move and clears the drag state.
	m, cmd := deliver(t, m, tea.MouseReleaseMsg{X: 5, Y: listRowY(queryRow), Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("drop on another list produced no move command")
	}
	if m.dragID != "" || m.dragActive || m.dropID != "" {
		t.Fatal("drag state survived the release")
	}
	if strings.Contains(m.listPanel.View(), "⤓ drop") {
		t.Fatal("drop marker survived the release")
	}

	// A press and release without motion moves nothing.
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(reminderRW), Button: tea.MouseLeft})
	m, cmd = deliver(t, m, tea.MouseReleaseMsg{X: 100, Y: listRowY(reminderRW), Button: tea.MouseLeft})
	if cmd != nil {
		t.Fatal("a click without motion moved a reminder")
	}

	// Dropping on the list the reminder already lives in moves nothing.
	m = deliver2(t, m, tea.MouseClickMsg{X: 100, Y: listRowY(reminderRW), Button: tea.MouseLeft})
	m = deliver2(t, m, tea.MouseMotionMsg{X: 5, Y: listRowY(alphaRow), Button: tea.MouseLeft})
	if m.dropID != "alpha" {
		t.Fatalf("drop target = %q, want alpha", m.dropID)
	}
	m, cmd = deliver(t, m, tea.MouseReleaseMsg{X: 5, Y: listRowY(alphaRow), Button: tea.MouseLeft})
	if cmd != nil {
		t.Fatal("a drop on the source list moved the reminder")
	}

	// The separator row is not a row at all: a click there neither moves
	// the selection nor loads anything.
	before, _ := m.listPanel.SelectedList()
	loaded := m.selectedList
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: listRowY(separator), Button: tea.MouseLeft})
	after, _ := m.listPanel.SelectedList()
	if after.ID != before.ID || m.selectedList != loaded {
		t.Fatalf("separator click changed the selection: %q -> %q", before.ID, after.ID)
	}
}

// TestReminderMovedMsgReportsOutcome verifies a move result refreshes and
// reports success, and surfaces the failure without touching the panels.
func TestReminderMovedMsgReportsOutcome(t *testing.T) {
	m := newDragTestModel(t)
	m, cmd := deliver(t, m, messages.ReminderMovedMsg{ListTitle: "Query"})
	if !strings.Contains(m.viewContent(), `Moved to "Query"`) {
		t.Fatal("move success is not reported")
	}
	if cmd == nil {
		t.Fatal("a move did not refresh lists and reminders")
	}

	m = deliver2(t, m, messages.ReminderMovedMsg{Err: fmt.Errorf("boom")})
	if !strings.Contains(m.viewContent(), "Move failed: boom") {
		t.Fatal("move failure is not reported")
	}
}

// TestCreateOnlyMode verifies `nag --create`: the form opens on its own,
// ctrl+c and Esc both exit, a typed "q" stays text, and a successful save
// quits instead of returning to a sidebar that is not there.
func TestCreateOnlyMode(t *testing.T) {
	lists := []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha"},
		{ID: "query", Title: "Query"},
	}
	newCreate := func(list string) Model {
		m := NewCreateModel(nil, mustCompileDefaults(t), list)
		m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
		return deliver2(t, m, messages.ListsLoadedMsg{Lists: lists})
	}

	m := newCreate("Alpha")
	if !m.createDlg.Visible() {
		t.Fatal("create-only mode did not open the form")
	}
	if !strings.Contains(m.viewContent(), "[l] List") {
		t.Fatal("the List row is missing from the create-only form")
	}

	// A typed "q" reaches the title field instead of quitting.
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !m.createDlg.Visible() || m.createDlg.TitleValue() != "q" {
		t.Fatalf("typed q did not reach the title: %q", m.createDlg.TitleValue())
	}

	// ctrl+c exits while the form is open.
	m, cmd := deliver(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit create-only mode")
	}

	// With no list preselected the form opens on the picker itself.
	m = newCreate("")
	if !strings.Contains(ansi.Strip(m.viewContent()), "Type to filter") {
		t.Fatal("create-only with no list did not open the list picker")
	}

	// ctrl+s with the chooser open reports the missing list instead of
	// being swallowed by the picker.
	m = newCreate("")
	m, cmd = deliver(t, m, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatalf("ctrl+s submitted an unfinished form: %v", cmd())
	}
	if !strings.Contains(ansi.Strip(m.viewContent()), "Select a list") {
		t.Fatalf("ctrl+s with the chooser open did not report the error:\n%s", ansi.Strip(m.viewContent()))
	}

	// Esc first cancels the open field, then the form, which is the only
	// exit in this mode.
	m = newCreate("Alpha")
	m, cmd = deliver(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil || !m.createDlg.Visible() {
		t.Fatal("esc in a field must not close the form")
	}
	m, cmd = deliver(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc did not quit create-only mode")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("esc did not quit create-only mode")
	}

	// A successful save quits; a failed one reopens the form with the error.
	m = newCreate("Alpha")
	m, cmd = deliver(t, m, messages.ReminderCreatedMsg{})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("a created reminder did not quit create-only mode")
	}

	m = deliver2(t, newCreate("Alpha"), messages.ReminderCreatedMsg{Err: fmt.Errorf("boom")})
	if !m.createDlg.Visible() {
		t.Fatal("a failed create closed the form")
	}
	if !strings.Contains(m.createDlg.ErrorText(), "Create failed: boom") {
		t.Fatalf("error text = %q", m.createDlg.ErrorText())
	}
}
