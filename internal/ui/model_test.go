package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

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

// newTestModel builds a root model with a fixed 120x32 terminal size and
// seeded lists/reminders. It never calls Init, creates an EventKit client,
// or executes backend commands.
func newTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil, mustCompileDefaults(t), keybind.DefaultAceAlphabet)
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
	m := NewModel(nil, mustCompileDefaults(t), keybind.DefaultAceAlphabet)
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
	m := NewModel(nil, keys, keybind.DefaultAceAlphabet)
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
	m := NewModel(nil, keys, keybind.DefaultAceAlphabet)
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
	m := NewModel(nil, keys, keybind.DefaultAceAlphabet)
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
// reflects configured remaps for the scope/action pairs owned by sibling
// components (dialog submit, calendar right, selector down, ace trigger).
func TestCompiledKeysCarryRemaps(t *testing.T) {
	b := keybind.Defaults()
	b["dialog"] = map[string][]string{"submit": {"ctrl+s"}}
	b["calendar"] = map[string][]string{"right": {"ctrl+right"}}
	b["selector"] = map[string][]string{"down": {"ctrl+down"}}
	b["global"] = map[string][]string{"ace_jump": {"a"}}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	m := NewModel(nil, keys, keybind.DefaultAceAlphabet)
	for _, tc := range []struct{ scope, action, alias string }{
		{"dialog", "submit", "ctrl+s"},
		{"calendar", "right", "ctrl+right"},
		{"selector", "down", "ctrl+down"},
		{"global", "ace_jump", "a"},
	} {
		aliases := m.keys.Aliases(tc.scope, tc.action)
		if len(aliases) != 1 || aliases[0] != tc.alias {
			t.Fatalf("%s.%s aliases = %v, want [%s]", tc.scope, tc.action, aliases, tc.alias)
		}
	}
}

func aceStart(t *testing.T, m Model) Model {
	t.Helper()
	next, _ := deliver(t, m, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !next.ace.active {
		t.Fatal("pressing z did not enter ace mode")
	}
	return next
}

// TestAceJumpSidebarSelectsListWithoutActions verifies a sidebar label
// selects and loads that list while the sidebar stays focused and no
// pending focus cycle happens.
func TestAceJumpSidebarSelectsListWithoutActions(t *testing.T) {
	m := newTestModel(t)
	beforeLines := len(strings.Split(ansi.Strip(m.listPanel.View()), "\n"))

	m = aceStart(t, m)
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
	m = aceStart(t, m)

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
	m := NewModel(nil, mustCompileDefaults(t), "as")
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "l1", Title: "One", Kind: reminders.ListNormal},
		{ID: "l2", Title: "Two", Kind: reminders.ListNormal},
		{ID: "l3", Title: "Three", Kind: reminders.ListNormal},
	}})
	m.selectedList = &reminders.ReminderList{ID: "l1", Title: "One", Kind: reminders.ListNormal}

	m = aceStart(t, m)
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
	m = aceStart(t, m)
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
	m = aceStart(t, m)
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
	m := NewModel(nil, mustCompileDefaults(t), keybind.DefaultAceAlphabet)
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
	m2 := NewModel(nil, mustCompileDefaults(t), keybind.DefaultAceAlphabet)
	m2 = deliver2(t, m2, tea.WindowSizeMsg{Width: 120, Height: 32})
	m2 = deliver2(t, m2, messages.RemindersLoadedMsg{
		Reminders: []reminders.Reminder{{ID: "x", Title: "X"}},
	})
	if got := m2.reminderPanel.Reminders(); len(got) != 0 {
		t.Fatalf("load without a selected list must be ignored, got %d reminders", len(got))
	}
}

// TestAceExitsOnResizeMouseListsAndTickButRetainsOnSpinner verifies the
// cancellation rules around transient messages.
func TestAceExitsOnResizeMouseListsAndTickButRetainsOnSpinner(t *testing.T) {
	m := newTestModel(t)

	m = aceStart(t, m)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	if m.ace.active {
		t.Fatal("resize must cancel ace")
	}

	m = aceStart(t, m)
	m = deliver2(t, m, tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	if m.ace.active {
		t.Fatal("mouse activity must cancel ace")
	}

	m = aceStart(t, m)
	m = deliver2(t, m, messages.TickMsg{})
	if m.ace.active {
		t.Fatal("auto-refresh tick must cancel ace")
	}

	m = aceStart(t, m)
	m = deliver2(t, m, messages.ListsLoadedMsg{Lists: []reminders.ReminderList{
		{ID: "alpha", Title: "Alpha", Kind: reminders.ListNormal},
		{ID: "query", Title: "Query", Kind: reminders.ListNormal},
	}})
	if m.ace.active {
		t.Fatal("lists load must cancel ace")
	}

	// Spinner-only messages retain ace.
	m = aceStart(t, m)
	m = deliver2(t, m, spinner.TickMsg{ID: m.spinner.ID()})
	if !m.ace.active {
		t.Fatal("spinner tick must retain ace")
	}
}

// TestAceStartWithNoTargetsStaysNormal verifies ace does not activate
// without jumpable rows and reports the reason.
func TestAceStartWithNoTargetsStaysNormal(t *testing.T) {
	m := NewModel(nil, mustCompileDefaults(t), keybind.DefaultAceAlphabet)
	m = deliver2(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = deliver2(t, m, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if m.ace.active {
		t.Fatal("ace must not activate with no targets")
	}
	if !strings.Contains(ansi.Strip(m.statusBar.View()), "No jump targets") {
		t.Fatalf("expected status hint, view:\n%s", ansi.Strip(m.statusBar.View()))
	}
}
