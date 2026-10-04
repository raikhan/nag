package dialog

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/BRO3886/go-eventkit"

	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
)

func testKeys(t *testing.T) keybind.Map {
	t.Helper()
	m, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile defaults: %v", err)
	}
	return m
}

func press(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: text}
}

func ctrlPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func altPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModAlt}
}

func mustCmd(t *testing.T, cmd tea.Cmd, name string) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected %s command", name)
	}
	return cmd()
}

// ctrlS updates the model with ctrl+s and requires a resulting message.
func ctrlS(t *testing.T, m CreateModel) (CreateModel, tea.Msg) {
	t.Helper()
	m2, cmd := m.Update(ctrlPress('s'))
	return m2, mustCmd(t, cmd, "ctrl+s save")
}

// browseTo moves the browsing selection to target with the default j alias.
func browseTo(t *testing.T, m CreateModel, target formField) CreateModel {
	t.Helper()
	for range formFieldCount {
		if m.selected == target && m.mode == formBrowsing {
			return m
		}
		m, _ = m.Update(press('j', "j"))
	}
	t.Fatalf("j never reached field %d", target)
	return m
}

// openFieldFor opens a field via its default mnemonic jump.
func openFieldFor(t *testing.T, m CreateModel, target formField) CreateModel {
	t.Helper()
	var jump rune
	switch target {
	case fieldTitle:
		jump = 't'
	case fieldNotes:
		jump = 'n'
	case fieldDate:
		jump = 'd'
	case fieldTime:
		jump = 'i'
	case fieldAlarm:
		jump = 'a'
	case fieldPriority:
		jump = 'p'
	case fieldRecurrence:
		jump = 'r'
	}
	m, _ = m.Update(press(jump, string(jump)))
	if m.mode != formEditing || m.active != target {
		t.Fatalf("jump did not open field %d (mode %v active %v)", target, m.mode, m.active)
	}
	return m
}

// noFollow parks a freshly opened create form in the left-hand menu, the
// state these tests drive one field at a time. Follow mode itself has its
// own tests.
func noFollow(m CreateModel) CreateModel {
	m.follow = false
	m.mode = formBrowsing
	m.active = noField
	m.selected = fieldTitle
	m.blurAll()
	m.rebuildPane()
	return m
}

// finishField commits the open editor with Enter and returns to browsing.
func finishField(t *testing.T, m CreateModel) CreateModel {
	t.Helper()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != formBrowsing {
		t.Fatalf("enter did not finish editing (mode %v, err %q)", m.mode, m.errText)
	}
	return m
}

// TestV2DialogInputAndPaste covers empty submit, key release, paste-as-text,
// backward field navigation and the confirm/list dialogs.
func TestV2DialogInputAndPaste(t *testing.T) {
	keys := testKeys(t)

	// Create-reminder dialog: empty Enter opens the title editor, never a
	// submit.
	cm := NewCreate()
	cm.SetKeys(keys)
	cm.Show("Alpha")
	cm = noFollow(cm)
	if !cm.Visible() {
		t.Fatal("create dialog did not open")
	}
	cm2, cmd := cm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("empty enter submitted: got cmd %v", cmd)
	}
	if !cm2.Visible() || cm2.mode != formEditing || cm2.active != fieldTitle {
		t.Fatalf("empty enter did not open the title editor: mode %v active %v", cm2.mode, cm2.active)
	}

	// Releasing Enter does not submit either.
	cm3, cmd2 := cm2.Update(tea.KeyReleaseMsg{Code: tea.KeyEnter})
	if cmd2 != nil {
		t.Fatalf("enter release submitted: got cmd %v", cmd2)
	}
	if !cm3.Visible() {
		t.Fatal("enter release closed the dialog")
	}

	// Pasted characters land in the open title editor as text.
	cm4, _ := cm3.Update(tea.PasteMsg{Content: "q ? n x"})
	if got := cm4.titleInput.Value(); got != "q ? n x" {
		t.Fatalf("paste produced title %q", got)
	}

	// Enter finishes the field; ctrl+s submits the form.
	cm4 = finishField(t, cm4)
	cm5, msg := ctrlS(t, cm4)
	if cm5.Visible() {
		t.Fatal("dialog still visible after submit")
	}
	sub, ok := msg.(CreateSubmitMsg)
	if !ok {
		t.Fatalf("expected CreateSubmitMsg, got %T", msg)
	}
	if sub.Input.Title != "q ? n x" {
		t.Fatalf("submitted title = %q", sub.Input.Title)
	}
	if sub.Input.ListName != "Alpha" {
		t.Fatalf("submitted list = %q", sub.Input.ListName)
	}

	// Shift-Tab moves browsing back to the title row: pasted text must land
	// in the title field.
	fm := NewCreate()
	fm.SetKeys(keys)
	fm.Show("Alpha")
	fm = noFollow(fm)
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // notes selected
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if fm.selected != fieldTitle {
		t.Fatalf("shift+tab selected %d", fm.selected)
	}
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // open title
	fm, _ = fm.Update(tea.PasteMsg{Content: "shift landed here"})
	if got := fm.titleInput.Value(); got != "shift landed here" {
		t.Fatalf("shift-tab paste landed in title = %q", got)
	}

	// Create-list dialog: paste + enter submits the pasted title.
	lm := NewCreateList()
	lm.SetKeys(keys)
	lm.Show()
	lm2, _ := lm.Update(tea.PasteMsg{Content: "q ? n x"})
	if got := lm2.titleInput.Value(); got != "q ? n x" {
		t.Fatalf("list paste produced title %q", got)
	}
	_, cmd4 := lm2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg2 := mustCmd(t, cmd4, "list submit")
	sub2, ok := msg2.(CreateListSubmitMsg)
	if !ok {
		t.Fatalf("expected CreateListSubmitMsg, got %T", msg2)
	}
	if sub2.Title != "q ? n x" {
		t.Fatalf("submitted list title = %q", sub2.Title)
	}

	// Confirm dialog: Y/N, enter/esc transitions.
	cf := NewConfirm()
	cf.SetKeys(keys)
	cf.Show("Delete?", ConfirmDelete)
	_, cmd5 := cf.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	msg3 := mustCmd(t, cmd5, "confirm yes")
	if yes, ok := msg3.(ConfirmYesMsg); !ok || yes.Action != ConfirmDelete {
		t.Fatalf("Y produced %#v", msg3)
	}

	cf3 := NewConfirm()
	cf3.SetKeys(keys)
	cf3.Show("Delete?", ConfirmDelete)
	_, cmd6 := cf3.Update(tea.KeyPressMsg{Code: 'N', Text: "N"})
	if _, ok := mustCmd(t, cmd6, "confirm no").(ConfirmNoMsg); !ok {
		t.Fatal("N did not answer no")
	}

	cf5 := NewConfirm()
	cf5.SetKeys(keys)
	cf5.Show("Delete?", ConfirmDeleteList)
	cf6, cmd7 := cf5.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cf6.Visible() {
		t.Fatal("confirm still visible after enter")
	}
	if yes, ok := mustCmd(t, cmd7, "enter confirm").(ConfirmYesMsg); !ok || yes.Action != ConfirmDeleteList {
		t.Fatalf("enter produced %#v", cmd7())
	}

	cf7 := NewConfirm()
	cf7.SetKeys(keys)
	cf7.Show("Delete?", ConfirmDelete)
	_, cmd8 := cf7.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := mustCmd(t, cmd8, "esc no").(ConfirmNoMsg); !ok {
		t.Fatal("escape did not answer no")
	}
}

func TestCreateFormBrowseEditAndJumps(t *testing.T) {
	keys := testKeys(t)

	m := NewCreate()
	m.SetKeys(keys)
	m.Show("Alpha")
	m = noFollow(m)
	if m.mode != formBrowsing || m.selected != fieldTitle {
		t.Fatalf("browsing must start at title: mode %v selected %d", m.mode, m.selected)
	}
	if m.titleInput.Focused() || m.notesInput.Focused() || m.timeInput.Focused() || m.picker.Focused() {
		t.Fatal("no editor may be focused while browsing")
	}

	// Tab cycles all seven rows without opening any editor.
	order := []formField{fieldNotes, fieldDate, fieldTime, fieldAlarm, fieldPriority, fieldRecurrence, fieldTitle}
	for _, want := range order {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.mode != formBrowsing || m.selected != want {
			t.Fatalf("tab: mode %v selected %d, want browsing %d", m.mode, m.selected, want)
		}
	}
	// j does the same.
	for _, want := range order {
		m, _ = m.Update(press('j', "j"))
		if m.mode != formBrowsing || m.selected != want {
			t.Fatalf("j: mode %v selected %d, want browsing %d", m.mode, m.selected, want)
		}
	}
	// Shift-Tab and k go backwards.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.selected != fieldRecurrence {
		t.Fatalf("shift+tab selected %d", m.selected)
	}
	m, _ = m.Update(press('k', "k"))
	if m.selected != fieldPriority {
		t.Fatalf("k selected %d", m.selected)
	}

	// Other typed keys and paste do nothing while browsing.
	m, _ = m.Update(press('x', "x"))
	m, _ = m.Update(tea.PasteMsg{Content: "junk"})
	if m.mode != formBrowsing {
		t.Fatal("browsing changed by typing/paste")
	}

	// Enter opens the selected field, Enter finishes it, no submit.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != formEditing || m.active != fieldPriority {
		t.Fatalf("enter did not open priority: %v %d", m.mode, m.active)
	}
	if !m.selector.Visible() {
		t.Fatal("priority selector did not open")
	}
	// Selector captures input: Esc cancels it back to browsing.
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := mustCmd(t, cmd, "selector cancel").(selectorCancelledMsg); !ok {
		t.Fatal("esc did not cancel the selector")
	}
	m, _ = m.Update(selectorCancelledMsg{})
	if m.mode != formBrowsing || m.selected != fieldPriority {
		t.Fatalf("cancel did not return to browsing priority: %v %d", m.mode, m.selected)
	}

	// Every mnemonic opens its field.
	for _, f := range []formField{fieldTitle, fieldNotes, fieldDate, fieldTime} {
		opened := openFieldFor(t, m, f)
		opened, _ = opened.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // cancel
		if opened.mode != formBrowsing {
			t.Fatalf("field %d did not close on esc", f)
		}
	}

	// While editing Title, j/k and mnemonic letters remain text.
	e := openFieldFor(t, m, fieldTitle)
	for _, r := range "jktdiapr" {
		e, _ = e.Update(press(r, string(r)))
	}
	if e.titleInput.Value() != "jktdiapr" {
		t.Fatalf("mnemonic letters became commands: %q", e.titleInput.Value())
	}
	if e.mode != formEditing {
		t.Fatal("letters closed the editor")
	}
	// Key releases are inert.
	e, _ = e.Update(tea.KeyReleaseMsg{Code: 'j'})
	if e.titleInput.Value() != "jktdiapr" {
		t.Fatalf("key release changed the text: %q", e.titleInput.Value())
	}
	// Esc restores the snapshot and returns to browsing.
	e, _ = e.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if e.mode != formBrowsing || e.titleInput.Value() != "" {
		t.Fatalf("esc did not restore: mode %v title %q", e.mode, e.titleInput.Value())
	}

	// One visible selected-row treatment: the marker appears for each of the
	// seven fields when selected.
	m.SetSize(120, 40)
	for _, f := range []formField{fieldTitle, fieldNotes, fieldDate, fieldTime, fieldAlarm, fieldPriority, fieldRecurrence} {
		m.selected = f
		view := m.View()
		if !strings.Contains(view, ">") {
			t.Fatalf("field %d selection marker missing in view", f)
		}
		if !strings.Contains(view, m.fieldLabel(f)) {
			t.Fatalf("field %d label missing in view", f)
		}
	}
}

func editFixture() reminders.Reminder {
	due := time.Date(2026, time.October, 4, 9, 0, 0, 0, time.Local)
	return reminders.Reminder{
		ID:       "rem-1",
		Title:    "Fixture",
		Notes:    "notes",
		DueDate:  &due,
		Priority: reminders.PriorityMedium,
	}
}

// TestCreateEditDatePriorityRecurrence exercises the six-field dialog.
func TestCreateEditDatePriorityRecurrence(t *testing.T) {
	keys := testKeys(t)

	t.Run("calendar movement produces submitted due date", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldDate)
		m.picker.SetValue("")
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-04"})
		if _, err := m.picker.Resolve(); err != nil {
			t.Fatalf("typed date invalid: %v", err)
		}
		m, _ = m.Update(ctrlPress('l')) // right: +1 day
		m, _ = m.Update(ctrlPress('j')) // down: +7 days
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit, ok := msg.(EditSubmitMsg)
		if !ok {
			t.Fatalf("ctrl+s produced %T", msg)
		}
		want := time.Date(2026, time.October, 12, 9, 0, 0, 0, time.Local)
		if edit.Input.DueDate == nil || !edit.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", edit.Input.DueDate, want)
		}
	})

	t.Run("typed date moves calendar and retains time", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(ctrlPress('j')) // +7 days from the edit base
		if m.picker.Value() != "2026-10-11" {
			t.Fatalf("calendar down wrote %q", m.picker.Value())
		}
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		want := time.Date(2026, time.October, 11, 9, 0, 0, 0, time.Local)
		if !edit.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", edit.Input.DueDate, want)
		}
	})

	t.Run("invalid due blocks submission", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "not a date"})
		m, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatalf("invalid due submitted: %v", cmd)
		}
		if !m.Visible() || m.mode != formEditing || m.active != fieldDate {
			t.Fatalf("invalid due did not stay editing: %v %d", m.mode, m.active)
		}
		if m.errText == "" || m.picker.Error() == "" {
			t.Fatal("no inline error for invalid due")
		}
	})

	t.Run("priority fuzzy selection", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.Show("Alpha")
		m = noFollow(m)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Priority smoke"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldPriority)
		if !m.selector.Visible() {
			t.Fatal("priority chooser did not open")
		}
		m, _ = m.Update(press('h', "h"))
		m, _ = m.Update(press('g', "g"))
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("chooser confirm produced no command")
		}
		sel, ok := cmd().(selectorSelectedMsg)
		if !ok {
			t.Fatalf("confirm produced %T", cmd())
		}
		m, _ = m.Update(sel) // consume the result
		if m.mode != formBrowsing || m.selected != fieldPriority {
			t.Fatalf("selector confirm did not return to browsing: %v %d", m.mode, m.selected)
		}
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		if sub.Input.Priority != reminders.PriorityHigh {
			t.Fatalf("priority = %d, want high", sub.Input.Priority)
		}
	})

	t.Run("repeat preset daily", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.Show("Alpha")
		m = noFollow(m)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Repeating"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m.picker.SetValue("2026-10-04")
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"daily"}})
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		if len(sub.Input.RecurrenceRules) != 1 ||
			sub.Input.RecurrenceRules[0].Frequency != eventkit.FrequencyDaily ||
			sub.Input.RecurrenceRules[0].Interval != 1 {
			t.Fatalf("rules = %+v, want Daily(1)", sub.Input.RecurrenceRules)
		}
	})

	t.Run("custom weekly mon fri interval 3 until a date", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldRecurrence) // presets, empty query
		m, _ = m.Update(press('c', "c"))        // Custom…
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if sel := mustCmd(t, cmd, "custom"); len(sel.(selectorSelectedMsg).IDs) != 1 ||
			sel.(selectorSelectedMsg).IDs[0] != "custom" {
			t.Fatal("custom preset not selected")
		} else {
			m, _ = m.Update(sel)
		}
		if !m.recurrenceE.Visible() {
			t.Fatal("custom editor did not open")
		}

		// Frequency -> weekly.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(press('w', "w"))
		m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(mustCmd(t, cmd, "freq").(selectorSelectedMsg))
		// Interval = 3.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace}) // clear default 1
		m, _ = m.Update(press('3', "3"))
		// Days -> Monday and Friday.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"0", "4"}})
		// End -> On date = 2026-12-31.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"1"}}) // on date
		for _, r := range []string{"2", "0", "2", "6", "-", "1", "2", "-", "3", "1"} {
			m, _ = m.Update(press(rune(r[0]), r))
		}

		m, cmd = m.Update(ctrlPress('s')) // apply editor
		submit := mustCmd(t, cmd, "editor apply").(RecurrenceSubmitMsg)
		rules := submit.Rules
		if len(rules) != 1 || rules[0].Frequency != eventkit.FrequencyWeekly ||
			rules[0].Interval != 3 || len(rules[0].DaysOfTheWeek) != 2 ||
			rules[0].End == nil || rules[0].End.EndDate == nil ||
			rules[0].End.EndDate.Format("2006-01-02") != "2026-12-31" {
			t.Fatalf("draft rules = %+v", rules)
		}
		for _, d := range rules[0].DaysOfTheWeek {
			if d.DayOfTheWeek != eventkit.Monday && d.DayOfTheWeek != eventkit.Friday || d.WeekNumber != 0 {
				t.Fatalf("unexpected day %+v", d)
			}
		}

		// The outer form consumes the editor result and submits it.
		m, _ = m.Update(submit)
		if m.mode != formBrowsing || m.selected != fieldRecurrence {
			t.Fatalf("apply did not return to browsing recurrence: %v %d", m.mode, m.selected)
		}
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.RecurrenceRules == nil ||
			!rulesEqual(*edit.Input.RecurrenceRules, rules) {
			t.Fatalf("submitted rules = %+v", edit.Input.RecurrenceRules)
		}
	})

	t.Run("monthly last weekday", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(press('c', "c"))
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(mustCmd(t, cmd, "custom").(selectorSelectedMsg))

		// Frequency -> monthly.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"monthly"}})
		// Mode -> On the.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"onthe"}})
		// Ordinal -> Last.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"-1"}})
		// Kind -> Weekday.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"2"}}) // dayKindWeekday

		m, cmd = m.Update(ctrlPress('s'))
		rules := mustCmd(t, cmd, "editor apply").(RecurrenceSubmitMsg).Rules
		want := eventkit.Monthly(1)
		want.DaysOfTheWeek = []eventkit.RecurrenceDayOfWeek{
			{DayOfTheWeek: eventkit.Monday, WeekNumber: 0},
			{DayOfTheWeek: eventkit.Tuesday, WeekNumber: 0},
			{DayOfTheWeek: eventkit.Wednesday, WeekNumber: 0},
			{DayOfTheWeek: eventkit.Thursday, WeekNumber: 0},
			{DayOfTheWeek: eventkit.Friday, WeekNumber: 0},
		}
		want.SetPositions = []int{-1}
		if len(rules) != 1 || !ruleEqual(rules[0], want) {
			t.Fatalf("rules = %+v, want %+v", rules, want)
		}
	})

	t.Run("invalid custom interval stays in the editor", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(press('c', "c"))
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(mustCmd(t, cmd, "custom").(selectorSelectedMsg))

		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // interval
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		m, cmd = m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatalf("invalid interval applied: %v", cmd())
		}
		if !m.recurrenceE.Visible() || m.recurrenceE.errText == "" {
			t.Fatalf("custom editor did not show the validation error")
		}
		// Correcting the interval applies the rule.
		m.recurrenceE.intervalIn.SetValue("2")
		m, cmd = m.Update(ctrlPress('s'))
		sub := mustCmd(t, cmd, "editor apply").(RecurrenceSubmitMsg)
		if len(sub.Rules) != 1 || sub.Rules[0].Interval != 2 {
			t.Fatalf("corrected rules = %+v", sub.Rules)
		}
		m, _ = m.Update(sub)
	})

	t.Run("untouched imported custom recurrence preserves rules and due", func(t *testing.T) {
		r := editFixture()
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Weekly(2, eventkit.Saturday)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: " Renamed"})
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.RecurrenceRules != nil {
			t.Fatalf("untouched recurrence patched: %+v", *edit.Input.RecurrenceRules)
		}
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("title-only edit touched the due date: %+v", edit.Input)
		}
	})

	t.Run("explicit clear sends nonnil empty pointer", func(t *testing.T) {
		r := editFixture()
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Daily(1)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"none"}})
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.RecurrenceRules == nil || len(*edit.Input.RecurrenceRules) != 0 {
			t.Fatalf("clear recurrence = %#v, want nonnil empty", edit.Input.RecurrenceRules)
		}
	})

	t.Run("cancel leaves rules unchanged", func(t *testing.T) {
		r := editFixture()
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Weekly(2, eventkit.Saturday)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = openFieldFor(t, m, fieldRecurrence)
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if _, ok := mustCmd(t, cmd, "chooser cancel").(selectorCancelledMsg); !ok {
			t.Fatalf("cancel produced %T", cmd())
		}
		m, _ = m.Update(selectorCancelledMsg{}) // consume the cancel message
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.RecurrenceRules != nil {
			t.Fatalf("cancel patched recurrence: %+v", *edit.Input.RecurrenceRules)
		}
	})

	t.Run("repeating reminders need a due date", func(t *testing.T) {
		// Creating with recurrence but no due date is blocked.
		m := NewCreate()
		m.SetKeys(keys)
		m.Show("Alpha")
		m = noFollow(m)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Repeating"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"daily"}})
		m2, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatalf("submitted without a due date: %v", cmd)
		}
		if !m2.Visible() {
			t.Fatal("form closed despite missing due date")
		}
		if m2.errText != "Repeating reminders need a due date" {
			t.Fatalf("error = %q", m2.errText)
		}
	})

	t.Run("changed recurrence without due blocks edit save", func(t *testing.T) {
		r := editFixture()
		r.DueDate = nil
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Daily(1)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"weekly"}})
		m2, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatalf("submitted changed recurrence without due: %v", cmd)
		}
		if m2.errText != "Repeating reminders need a due date" {
			t.Fatalf("error = %q", m2.errText)
		}
	})

	t.Run("unrelated edit of imported repeat without due is permitted", func(t *testing.T) {
		r := editFixture()
		r.DueDate = nil
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Daily(1)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = openFieldFor(t, m, fieldRecurrence)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"daily"}}) // unchanged
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("unrelated edit patched due: %+v", edit.Input)
		}
		if edit.Input.RecurrenceRules != nil {
			t.Fatalf("unrelated edit patched rules: %+v", *edit.Input.RecurrenceRules)
		}
	})
}

func perth(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Australia/Perth")
	if err != nil {
		t.Fatalf("load Perth: %v", err)
	}
	return loc
}

func newCreateAt(t *testing.T, now time.Time) CreateModel {
	t.Helper()
	m := NewCreate()
	m.SetKeys(testKeys(t))
	m.showAt("Alpha", now)
	m.SetSize(120, 40)
	return noFollow(m)
}

func newEditAt(t *testing.T, r reminders.Reminder, now time.Time) CreateModel {
	t.Helper()
	m := NewCreate()
	m.SetKeys(testKeys(t))
	m.showEditAt(r, now)
	m.SetSize(120, 40)
	return m
}

func TestDateAndTimeSubmission(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	t.Run("date plus 6p composes local evening", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-04"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "6p"})
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		want := time.Date(2026, time.October, 4, 18, 0, 0, 0, loc)
		if sub.Input.DueDate == nil || !sub.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v (10:00Z)", sub.Input.DueDate, want)
		}
		if got := sub.Input.DueDate.UTC(); got.Hour() != 10 || got.Minute() != 0 {
			t.Fatalf("instant = %v, want 10:00Z", got)
		}
	})

	t.Run("1413 composes afternoon", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-04"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "1413"})
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		want := time.Date(2026, time.October, 4, 14, 13, 0, 0, loc)
		if sub.Input.DueDate == nil || !sub.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", sub.Input.DueDate, want)
		}
		if got := sub.Input.DueDate.UTC(); got.Hour() != 6 || got.Minute() != 13 {
			t.Fatalf("instant = %v, want 06:13Z", got)
		}
	})

	t.Run("date only means 09:00 local", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-04"})
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		want := time.Date(2026, time.October, 4, 9, 0, 0, 0, loc)
		if sub.Input.DueDate == nil || !sub.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", sub.Input.DueDate, want)
		}
	})

	t.Run("calendar keys change only the date", func(t *testing.T) {
		due := time.Date(2026, time.October, 4, 14, 13, 0, 0, loc)
		r := reminders.Reminder{ID: "r1", Title: "Fix", DueDate: &due}
		m := newEditAt(t, r, now)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(ctrlPress('l')) // +1 day
		m, _ = m.Update(ctrlPress('j')) // +7 days
		if m.picker.Value() != "2026-10-12" {
			t.Fatalf("date = %q, want 2026-10-12", m.picker.Value())
		}
		if got := m.summaryValue(fieldTime); got != "2:13pm AWST" {
			t.Fatalf("time summary changed: %q", got)
		}
		// Changing Time never changes Date.
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m.timeInput.SetValue("") // clear the prefilled draft
		m, _ = m.Update(tea.PasteMsg{Content: "6p"})
		if m.summaryValue(fieldDate) != "2026-10-12" {
			t.Fatalf("date summary changed: %q", m.summaryValue(fieldDate))
		}
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		want := time.Date(2026, time.October, 12, 18, 0, 0, 0, loc)
		if edit.Input.DueDate == nil || !edit.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", edit.Input.DueDate, want)
		}
	})

	t.Run("fuzzy entry commits the resolved date", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "tom"})
		// While editing, the summary shows the resolved date, not the term.
		if got := m.summaryValue(fieldDate); got != "2026-10-05" {
			t.Fatalf("editing summary = %q, want 2026-10-05", got)
		}
		m = finishField(t, m)
		if m.committedDate != "2026-10-05" {
			t.Fatalf("committedDate = %q, want 2026-10-05", m.committedDate)
		}
		if m.picker.Value() != "2026-10-05" {
			t.Fatalf("picker not synced to canonical date: %q", m.picker.Value())
		}
		if got := m.summaryValue(fieldDate); got != "2026-10-05" {
			t.Fatalf("browsing summary = %q, want 2026-10-05", got)
		}
	})

	t.Run("imported UTC timestamp prefills local date and time", func(t *testing.T) {
		imported := time.Date(2026, 10, 3, 18, 13, 0, 0, time.UTC)
		r := reminders.Reminder{ID: "r2", Title: "Fix", DueDate: &imported}
		m := newEditAt(t, r, now)
		if got := m.summaryValue(fieldDate); got != "2026-10-04" {
			t.Fatalf("date summary = %q", got)
		}
		if got := m.summaryValue(fieldTime); got != "2:13am AWST" {
			t.Fatalf("time summary = %q", got)
		}
	})

	t.Run("time without date blocks save and opens date", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "6p"})
		m = finishField(t, m) // time alone commits as a draft
		if m.committedTime != "6pm" || m.committedDate != "" {
			t.Fatalf("time draft not committed: %q %q", m.committedTime, m.committedDate)
		}
		m2, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatal("time-only save submitted")
		}
		if m2.errText != "Set a date before a time" {
			t.Fatalf("error = %q", m2.errText)
		}
		if m2.mode != formEditing || m2.active != fieldDate {
			t.Fatalf("save did not open date editing: %v %d", m2.mode, m2.active)
		}
	})

	t.Run("empty and invalid time commits", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // empty commits
		if m.committedTime != "" || m.mode != formBrowsing {
			t.Fatalf("empty time did not commit: %q %v", m.committedTime, m.mode)
		}
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "25:99"})
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil || m.mode != formEditing || m.errText == "" {
			t.Fatalf("invalid time did not stay editing: %v %v %q", cmd, m.mode, m.errText)
		}
		// Esc restores and returns to browsing.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.mode != formBrowsing || m.committedTime != "" {
			t.Fatalf("esc did not restore the time field: %v %q", m.mode, m.committedTime)
		}
	})

	t.Run("nonexistent DST clock rejects", func(t *testing.T) {
		ny, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		nowNY := time.Date(2026, time.March, 8, 1, 0, 0, 0, ny)
		m := newCreateAt(t, nowNY)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-03-08"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "2:30am"})
		m = finishField(t, m)
		m2, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatal("nonexistent clock submitted")
		}
		if !strings.Contains(m2.errText, "Time does not exist on that date") {
			t.Fatalf("error = %q", m2.errText)
		}
	})
}

// TestRemindMeRow covers the Remind me row: the due-date guard, the chooser
// contents, and the sparse alarm patch it writes.
func TestRemindMeRow(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	t.Run("without a date the row explains itself", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "No due"})
		m = finishField(t, m)
		m, _ = m.Update(press('a', "a"))
		if m.mode != formBrowsing || m.selected != fieldAlarm {
			t.Fatalf("guard changed the mode: mode %v selected %d", m.mode, m.selected)
		}
		if m.errText != "Set a date to use an early reminder" {
			t.Fatalf("errText = %q", m.errText)
		}
		if m.selector.Visible() {
			t.Fatal("the chooser must not open without a date")
		}
	})

	t.Run("with a date the chooser opens on None", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Call mum"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-05"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldAlarm)
		if !m.selector.Visible() {
			t.Fatal("the chooser did not open")
		}
		opts := alarmOptions()
		want := []string{"None", "At due time", "5 minutes before", "10 minutes before",
			"15 minutes before", "30 minutes before", "1 hour before", "2 hours before", "1 day before"}
		if len(opts) != len(want) {
			t.Fatalf("chooser rows = %d, want %d", len(opts), len(want))
		}
		for i, o := range opts {
			if o.Label != want[i] {
				t.Fatalf("row %d = %q, want %q", i, o.Label, want[i])
			}
		}
		if alarmChoiceID(m) != "none" {
			t.Fatalf("preselected row = %q, want none", alarmChoiceID(m))
		}
	})

	t.Run("30 minutes before patches the alarm in", func(t *testing.T) {
		m := newEditAt(t, editFixture(), now)
		m = openFieldFor(t, m, fieldAlarm)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"30m"}})
		if m.alarmLabel() != "30 minutes before" {
			t.Fatalf("label = %q", m.alarmLabel())
		}
		_, msg := ctrlS(t, m)
		sub, ok := msg.(EditSubmitMsg)
		if !ok {
			t.Fatalf("save produced %T", msg)
		}
		if sub.Input.Alarms == nil {
			t.Fatal("saving a new alarm must patch it")
		}
		want := []reminders.Alarm{{RelativeOffset: -30 * time.Minute}}
		if !reminders.AlarmsEqual(*sub.Input.Alarms, want) {
			t.Fatalf("alarms = %+v, want %+v", *sub.Input.Alarms, want)
		}
	})

	t.Run("At due time uses the composed due instant", func(t *testing.T) {
		m := newEditAt(t, editFixture(), now)
		m = openFieldFor(t, m, fieldAlarm)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"attime"}})
		_, msg := ctrlS(t, m)
		sub := msg.(EditSubmitMsg)
		due, err := m.composeDue()
		if err != nil || due == nil {
			t.Fatalf("compose due: %v", err)
		}
		want := []reminders.Alarm{{AbsoluteDate: due}}
		if sub.Input.Alarms == nil || !reminders.AlarmsEqual(*sub.Input.Alarms, want) {
			t.Fatalf("alarms = %+v, want %+v", sub.Input.Alarms, want)
		}
	})

	t.Run("an untouched row sends no alarm patch", func(t *testing.T) {
		m := newEditAt(t, editFixture(), now)
		_, msg := ctrlS(t, m)
		sub := msg.(EditSubmitMsg)
		if sub.Input.Alarms != nil {
			t.Fatalf("unchanged row patched alarms: %+v", *sub.Input.Alarms)
		}
	})

	t.Run("an existing alarm round-trips and re-saves untouched", func(t *testing.T) {
		r := editFixture()
		r.Alarms = []reminders.Alarm{{RelativeOffset: -15 * time.Minute}}
		m := newEditAt(t, r, now)
		if m.alarmLabel() != "15 minutes before" {
			t.Fatalf("seeded label = %q", m.alarmLabel())
		}
		_, msg := ctrlS(t, m)
		sub := msg.(EditSubmitMsg)
		if sub.Input.Alarms != nil {
			t.Fatalf("unchanged seeded row patched alarms: %+v", *sub.Input.Alarms)
		}
	})

	t.Run("clearing the date drops the alarm", func(t *testing.T) {
		r := editFixture()
		r.Alarms = []reminders.Alarm{{RelativeOffset: -15 * time.Minute}}
		m := newEditAt(t, r, now)
		m = openFieldFor(t, m, fieldDate)
		m.picker.SetValue("")
		m = finishField(t, m)
		if m.alarmKind != alarmNone {
			t.Fatalf("alarm kind = %d, want none", m.alarmKind)
		}
		if m.alarmLabel() != "None" {
			t.Fatalf("label = %q, want None", m.alarmLabel())
		}
		_, msg := ctrlS(t, m)
		sub := msg.(EditSubmitMsg)
		if sub.Input.Alarms == nil {
			t.Fatal("clearing the date must remove the alarm")
		}
		if len(*sub.Input.Alarms) != 0 {
			t.Fatalf("alarms = %+v, want an empty removal", *sub.Input.Alarms)
		}
	})
}

func TestDateSuggestionCyclingAndCompletion(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	m := newCreateAt(t, now)
	m = openFieldFor(t, m, fieldTitle)
	m, _ = m.Update(tea.PasteMsg{Content: "Fix"})
	m = finishField(t, m)
	m = openFieldFor(t, m, fieldTime)
	m, _ = m.Update(tea.PasteMsg{Content: "6p"})
	m = finishField(t, m)
	m = openFieldFor(t, m, fieldDate)
	m, _ = m.Update(tea.PasteMsg{Content: "to"})
	// Next suggestion cycles without resetting the candidate, then the
	// picker-owned ctrl+y completes it.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(ctrlPress('y'))
	if m.picker.Value() != "tomorrow" {
		t.Fatalf("completion produced %q", m.picker.Value())
	}
	// Tab while the date editor is open is a no-op: the field stays open.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.mode != formEditing || m.active != fieldDate {
		t.Fatalf("tab interrupted editing: mode %v active %d", m.mode, m.active)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.mode != formEditing || m.active != fieldDate {
		t.Fatalf("shift-tab interrupted editing: mode %v active %d", m.mode, m.active)
	}
	// Enter finishes and browses the same row.
	m = finishField(t, m)
	if m.selected != fieldDate {
		t.Fatalf("enter selected %d, want fieldDate", m.selected)
	}
	m, msg := ctrlS(t, m)
	sub := msg.(CreateSubmitMsg)
	want := time.Date(2026, time.October, 5, 18, 0, 0, 0, loc)
	if sub.Input.DueDate == nil || !sub.Input.DueDate.Equal(want) {
		t.Fatalf("due = %v, want %v", sub.Input.DueDate, want)
	}
}

func TestDirtyPreservationAndClearing(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	fixture := func() reminders.Reminder {
		due := time.Date(2026, time.October, 4, 18, 0, 0, 0, loc)
		return reminders.Reminder{ID: "r1", Title: "Fix", DueDate: &due}
	}

	t.Run("title-only edit leaves no due patch", func(t *testing.T) {
		m := newEditAt(t, fixture(), now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: " Renamed"})
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("due patched: %+v", edit.Input)
		}
	})

	t.Run("priority-only edit leaves no due patch", func(t *testing.T) {
		m := newEditAt(t, fixture(), now)
		m = openFieldFor(t, m, fieldPriority)
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"high"}})
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("due patched: %+v", edit.Input)
		}
	})

	t.Run("unchanged and cancelled date/time leave no patch", func(t *testing.T) {
		m := newEditAt(t, fixture(), now)
		m = openFieldFor(t, m, fieldDate)
		m = finishField(t, m) // commit unchanged
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // cancel
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("due patched: %+v", edit.Input)
		}
	})

	t.Run("equivalent time 18:00 does not patch", func(t *testing.T) {
		m := newEditAt(t, fixture(), now)
		m = openFieldFor(t, m, fieldTime)
		m.timeInput.SetValue("18:00")
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.DueDate != nil || edit.Input.ClearDueDate {
			t.Fatalf("equivalent time patched: %+v", edit.Input)
		}
	})

	t.Run("clear date clears time and sets ClearDueDate", func(t *testing.T) {
		m := newEditAt(t, fixture(), now)
		m = openFieldFor(t, m, fieldDate)
		m.picker.SetValue("")
		m = finishField(t, m)
		if m.committedTime != "" || m.timeInput.Value() != "" {
			t.Fatalf("clearing date did not clear time: %q %q", m.committedTime, m.timeInput.Value())
		}
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if !edit.Input.ClearDueDate {
			t.Fatalf("ClearDueDate not set: %+v", edit.Input)
		}
	})

	t.Run("clearing due also clears recurrence", func(t *testing.T) {
		r := fixture()
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Daily(1)}
		m := newEditAt(t, r, now)
		m = openFieldFor(t, m, fieldDate)
		m.picker.SetValue("")
		m = finishField(t, m)
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if !edit.Input.ClearDueDate {
			t.Fatalf("ClearDueDate not set: %+v", edit.Input)
		}
		if edit.Input.RecurrenceRules == nil || len(*edit.Input.RecurrenceRules) != 0 {
			t.Fatalf("recurrence not cleared: %#v", edit.Input.RecurrenceRules)
		}
	})
}

// TestFormBehavioralRemaps verifies configured remaps replace defaults in
// their contexts without leaking into text editing.
func TestFormBehavioralRemaps(t *testing.T) {
	b := keybind.Defaults()
	b["form"] = map[string][]string{
		"next_field": {"down", "ctrl+n"},
		"jump_time":  {"m"},
		"save":       {"alt+s"},
	}
	b["field"] = map[string][]string{"confirm": {"ctrl+e"}}
	b["calendar"] = map[string][]string{"right": {"alt+l"}}
	b["selector"] = map[string][]string{"down": {"ctrl+d"}}
	keys, err := keybind.Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	m := NewCreate()
	m.SetKeys(keys)
	m.showAt("Alpha", now)
	m = noFollow(m)
	m.SetSize(120, 40)

	// down moves browsing; j does nothing.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != fieldNotes {
		t.Fatalf("down selected %d", m.selected)
	}
	m, _ = m.Update(press('j', "j"))
	if m.selected != fieldNotes || m.mode != formBrowsing {
		t.Fatalf("j must do nothing while browsing: %d %v", m.selected, m.mode)
	}

	// m opens the time editor; i is disabled.
	m, _ = m.Update(press('m', "m"))
	if m.mode != formEditing || m.active != fieldTime {
		t.Fatalf("m did not open time: %v %d", m.mode, m.active)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = m.Update(press('i', "i"))
	if m.mode != formBrowsing {
		t.Fatal("disabled jump i opened an editor")
	}

	// ctrl+e finishes the field; m and j are text while editing.
	m, _ = m.Update(press('m', "m"))
	m, _ = m.Update(press('j', "j"))
	if got := m.timeInput.Value(); got != "j" {
		t.Fatalf("m/j not text in time editor: %q", got)
	}
	// Replace the invalid draft with a valid time, then finish.
	m.timeInput.SetValue("6p")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	if m.mode != formBrowsing {
		t.Fatal("ctrl+e did not finish the field")
	}
	if m.committedTime != "6pm" {
		t.Fatalf("committed time = %q", m.committedTime)
	}

	// alt+s saves the form (after a title).
	m = openFieldFor(t, m, fieldTitle)
	m, _ = m.Update(tea.PasteMsg{Content: "Remap"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // cancel title
	m, _ = m.Update(altPress('s'))
	// alt+s with an empty title blocks and opens the title editor.
	if m.mode != formEditing || m.active != fieldTitle || m.errText != "Title is required" {
		t.Fatalf("alt+s did not enforce the title: %v %d %q", m.mode, m.active, m.errText)
	}

	// Date: alt+l moves only the date.
	d := NewCreate()
	d.SetKeys(keys)
	d.showEditAt(reminders.Reminder{ID: "r", Title: "F", DueDate: new(time.Date(2026, 10, 4, 9, 0, 0, 0, loc))}, now)
	d.SetSize(120, 40)
	d = openFieldFor(t, d, fieldDate)
	d, _ = d.Update(altPress('l'))
	if d.picker.Value() != "2026-10-05" {
		t.Fatalf("alt+l did not move the date: %q", d.picker.Value())
	}
	d, _ = d.Update(ctrlPress('l'))
	if d.picker.Value() != "2026-10-05" {
		t.Fatalf("ctrl+l must not move the date after remap: %q", d.picker.Value())
	}

	// Selector: ctrl+d moves the selection, not the query.
	m2 := NewCreate()
	m2.SetKeys(keys)
	m2.showAt("Alpha", now)
	m2 = noFollow(m2)
	m2.SetSize(120, 40)
	m2 = openFieldFor(t, m2, fieldPriority)
	m2, _ = m2.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	m2, cmd := m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	sel := mustCmd(t, cmd, "selector confirm").(selectorSelectedMsg)
	if len(sel.IDs) != 1 || sel.IDs[0] != "low" {
		t.Fatalf("ctrl+d did not move the selection: %v", sel.IDs)
	}
}

// TestV2NotesTextareaAndEditor covers the multiline Notes editor: ctrl+j
// inserts a newline, Enter finishes the field, ctrl+o launches $EDITOR and
// the editor result message replaces the value.
func TestV2NotesTextareaAndEditor(t *testing.T) {
	keys := testKeys(t)
	m := NewCreate()
	m.SetKeys(keys)
	m.Show("Alpha")
	m = noFollow(m)
	m = openFieldFor(t, m, fieldNotes)

	m, _ = m.Update(tea.PasteMsg{Content: "a"})
	m, _ = m.Update(ctrlPress('j'))
	m, _ = m.Update(tea.PasteMsg{Content: "b"})
	if got := m.notesInput.Value(); got != "a\nb" {
		t.Fatalf("notes value after ctrl+j = %q, want %q", got, "a\nb")
	}
	if got := m.summaryValue(fieldNotes); got != "a b" {
		t.Fatalf("summary flattens newlines: got %q, want %q", got, "a b")
	}

	// Enter finishes the field; the multiline value survives.
	m = finishField(t, m)
	if m.notesInput.Value() != "a\nb" {
		t.Fatalf("notes value lost after finishing: %q", m.notesInput.Value())
	}

	// ctrl+o on Notes launches the editor; on other fields it does nothing.
	m = openFieldFor(t, m, fieldNotes)
	m2, cmd := m.Update(ctrlPress('o'))
	if cmd == nil {
		t.Fatal("ctrl+o did not launch the notes editor")
	}
	if m2.mode != formEditing || m2.active != fieldNotes {
		t.Fatalf("editor launch changed field state (mode %v active %v)", m2.mode, m2.active)
	}
	m3 := NewCreate()
	m3.SetKeys(keys)
	m3.Show("Alpha")
	m3 = noFollow(m3)
	m3 = openFieldFor(t, m3, fieldTitle)
	if _, cmd := m3.Update(ctrlPress('o')); cmd != nil {
		t.Fatal("ctrl+o must not trigger outside the Notes field")
	}

	// The editor completion message replaces the value and stays editing.
	m4, _ := m2.Update(notesEditorDoneMsg{content: "line1\nline2"})
	if m4.notesInput.Value() != "line1\nline2" {
		t.Fatalf("editor content not applied: %q", m4.notesInput.Value())
	}
	if m4.mode != formEditing || m4.active != fieldNotes {
		t.Fatalf("editor result dropped editing state (mode %v active %v)", m4.mode, m4.active)
	}
	m5, _ := m2.Update(notesEditorDoneMsg{err: errors.New("boom")})
	if m5.errText == "" {
		t.Fatal("editor failure not surfaced")
	}
}

// newFollowCreate opens the create form at its defaults, with follow mode on
// and the Title row already open.
func newFollowCreate(t *testing.T, now time.Time) CreateModel {
	t.Helper()
	m := NewCreate()
	m.SetKeys(testKeys(t))
	m.showAt("Alpha", now)
	m.SetSize(120, 40)
	return m
}

// finishInto presses Enter and asserts follow mode opened the expected row.
func finishInto(t *testing.T, m CreateModel, want formField) CreateModel {
	t.Helper()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != formEditing || m.active != want {
		t.Fatalf("enter did not open field %d (mode %v active %d, err %q)", want, m.mode, m.active, m.errText)
	}
	return m
}

// feedSelectorResult runs a chooser command and feeds its message back.
func feedSelectorResult(t *testing.T, m CreateModel, cmd tea.Cmd) CreateModel {
	t.Helper()
	m, extra := m.Update(mustCmd(t, cmd, "selector confirm"))
	if extra != nil {
		t.Fatalf("selector result produced an unexpected command: %T", extra)
	}
	return m
}

// TestTimeFieldKeys covers the Time field's own keys and the canonical clock a
// relative offset commits to.
func TestTimeFieldKeys(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	t.Run("ctrl+d clears the open draft", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "6pm"})
		if m.timeDraft == nil {
			t.Fatal("the draft did not resolve")
		}
		m, _ = m.Update(ctrlPress('d'))
		if m.timeInput.Value() != "" || m.timeDraft != nil || m.timeDraftErr != "" {
			t.Fatalf("ctrl+d left input %q, draft %v, err %q",
				m.timeInput.Value(), m.timeDraft, m.timeDraftErr)
		}
		if m.committedTime != "" {
			t.Fatalf("clearing the draft changed the committed time: %q", m.committedTime)
		}
		m = finishField(t, m)
		if m.committedTime != "" || m.timeClock != nil {
			t.Fatalf("clear not committed: %q %v", m.committedTime, m.timeClock)
		}
		if got := m.summaryValue(fieldTime); got != "None" {
			t.Fatalf("summary = %q, want None", got)
		}
	})

	t.Run("a relative offset commits the resolved clock", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(tea.PasteMsg{Content: "Relative"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-05"})
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "+75min"})
		if m.timeDraft == nil || m.timeDraft.String() != "2:30pm" {
			t.Fatalf("preview did not resolve +75min: %v", m.timeDraft)
		}
		m = finishField(t, m)
		if m.committedTime != "2:30pm" {
			t.Fatalf("committedTime = %q, want 2:30pm", m.committedTime)
		}
		if got := m.timeInput.Value(); got != "2:30pm" {
			t.Fatalf("time input = %q, want the canonical clock", got)
		}
		_, cmd := m.Update(ctrlPress('s'))
		msg, ok := mustCmd(t, cmd, "create submit").(CreateSubmitMsg)
		if !ok || msg.Input.DueDate == nil {
			t.Fatalf("no due date submitted: %+v", msg)
		}
		want := time.Date(2026, time.October, 5, 14, 30, 0, 0, loc)
		if !msg.Input.DueDate.Equal(want) {
			t.Fatalf("due = %v, want %v", msg.Input.DueDate, want)
		}
	})

	t.Run("an invalid relative form is rejected", func(t *testing.T) {
		m := newCreateAt(t, now)
		m = openFieldFor(t, m, fieldTime)
		m, _ = m.Update(tea.PasteMsg{Content: "+1h30m"})
		if m.timeDraftErr == "" {
			t.Fatalf("combined form accepted: draft %v", m.timeDraft)
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.active != fieldTime {
			t.Fatalf("invalid time closed the field: active %d", m.active)
		}
		if m.committedTime != "" {
			t.Fatalf("invalid time committed %q", m.committedTime)
		}
	})
}

// TestRecurrenceChoicePreselection covers reopening the recurrence chooser
// with the committed choice still highlighted.
func TestRecurrenceChoicePreselection(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	cases := []struct {
		name  string
		rules []eventkit.RecurrenceRule
		want  string
	}{
		{"none", nil, "none"},
		{"preset", []eventkit.RecurrenceRule{eventkit.Daily(1)}, "daily"},
		{"custom representable", []eventkit.RecurrenceRule{
			eventkit.Weekly(2, eventkit.Monday, eventkit.Wednesday)}, "custom"},
		{"custom unrepresentable", []eventkit.RecurrenceRule{{
			Frequency:     eventkit.FrequencyDaily,
			Interval:      1,
			DaysOfTheWeek: []eventkit.RecurrenceDayOfWeek{{DayOfTheWeek: eventkit.Monday}},
		}}, "custom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCreateAt(t, now)
			m.recurrence = tc.rules
			m = openFieldFor(t, m, fieldRecurrence)
			opt, ok := m.selector.SelectedOption()
			if !ok || opt.ID != tc.want {
				t.Fatalf("chooser highlights %q (%v), want %q", opt.ID, ok, tc.want)
			}
		})
	}

	t.Run("applying a custom schedule keeps Custom highlighted", func(t *testing.T) {
		m := newCreateAt(t, now)
		m.recurrence = []eventkit.RecurrenceRule{
			eventkit.Weekly(3, eventkit.Monday, eventkit.Wednesday)}
		m = openFieldFor(t, m, fieldRecurrence)
		m, extra := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // Custom…
		m, extra = m.Update(mustCmd(t, extra, "recurrence chooser"))
		if extra != nil {
			t.Fatalf("opening Custom produced an unexpected command: %T", extra)
		}
		if !m.recurrenceE.Visible() {
			t.Fatal("Custom editor did not open")
		}
		var applyCmd tea.Cmd
		m.recurrenceE, applyCmd = m.recurrenceE.Update(ctrlPress('s'))
		msg, ok := mustCmd(t, applyCmd, "recurrence apply").(RecurrenceSubmitMsg)
		if !ok {
			t.Fatalf("apply produced no RecurrenceSubmitMsg (err %q)", m.recurrenceE.errText)
		}
		m2, _ := m.Update(msg)
		if got := m2.summaryValue(fieldRecurrence); got != "Every 3 weeks on Mon, Wed" {
			t.Fatalf("summary = %q", got)
		}
		m2 = openFieldFor(t, m2, fieldRecurrence)
		opt, ok := m2.selector.SelectedOption()
		if !ok || opt.ID != "custom" {
			t.Fatalf("reopened chooser highlights %q (%v), want custom", opt.ID, ok)
		}
	})
}

// TestRecurrenceDraftRestoration covers the Custom editor keeping its draft
// across reopening, and Reset dropping it between reminders.
func TestRecurrenceDraftRestoration(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	base := midnightOf(now)

	// Cancel: the draft the user typed survives the close.
	m := NewRecurrence(testKeys(t))
	m.SetSize(80, 24)
	m.Show(nil, now, base, false)
	m.frequency = eventkit.FrequencyWeekly
	m.intervalIn.SetValue("3")
	m.weeklyDays = []bool{true, false, true, false, false, false, false}
	m.endMode = endOnDate
	m.endPicker.SetValue("2026-12-31")
	m.endDateOn = true
	m.Hide()

	m.Show(nil, now, base, false)
	if m.frequency != eventkit.FrequencyWeekly {
		t.Fatalf("frequency = %v, want weekly", m.frequency)
	}
	if got := m.intervalIn.Value(); got != "3" {
		t.Fatalf("interval = %q, want 3", got)
	}
	if want := []bool{true, false, true, false, false, false, false}; !boolsEqual(m.weeklyDays, want) {
		t.Fatalf("weekdays = %v, want %v", m.weeklyDays, want)
	}
	if m.endMode != endOnDate || m.endPickerValue().Format("2006-01-02") != "2026-12-31" || !m.endDateOn {
		t.Fatalf("end not preserved: mode %d on %v", m.endMode, m.endDateOn)
	}

	// Apply: reopening restores the applied draft, not a rounded prefill.
	cmd := m.apply()
	if cmd == nil {
		t.Fatalf("apply rejected the draft: %q", m.errText)
	}
	msg, ok := mustCmd(t, cmd, "recurrence apply").(RecurrenceSubmitMsg)
	if !ok {
		t.Fatal("apply produced no RecurrenceSubmitMsg")
	}
	m.Show(msg.Rules, now, base, false)
	if got := m.intervalIn.Value(); got != "3" || m.endMode != endOnDate {
		t.Fatalf("applied draft lost: interval %q end mode %d", got, m.endMode)
	}

	// Reset: a draft never leaks into the next reminder.
	m.Reset()
	m.Show(nil, now, base, false)
	if m.frequency != eventkit.FrequencyDaily || m.intervalIn.Value() != "1" {
		t.Fatalf("Reset kept the draft: frequency %v interval %q", m.frequency, m.intervalIn.Value())
	}
}

// TestRecurrenceEditorNavigation covers the Custom editor's j/k field
// stepping, the follow-mode chooser walk and the toggle being reachable from
// inside the editor.
func TestRecurrenceEditorNavigation(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	base := midnightOf(now)

	open := func(follow bool) RecurrenceModel {
		m := NewRecurrence(testKeys(t))
		m.SetSize(80, 24)
		m.Show(nil, now, base, follow)
		return m
	}

	t.Run("j and k step the editor fields", func(t *testing.T) {
		m := open(false)
		if m.focusIndex != 0 || m.focusedField() != fFrequency {
			t.Fatalf("editor starts at index %d (%v), want Frequency", m.focusIndex, m.focusedField())
		}
		m, _ = m.Update(press('j', "j"))
		if m.focusIndex != 1 || m.focusedField() != fInterval {
			t.Fatalf("j moved to index %d (%v), want the interval field", m.focusIndex, m.focusedField())
		}
		if !m.intervalIn.Focused() {
			t.Fatal("j did not focus the interval input")
		}
		m, _ = m.Update(press('k', "k"))
		if m.focusIndex != 0 || m.focusedField() != fFrequency {
			t.Fatalf("k moved to index %d (%v), want Frequency", m.focusIndex, m.focusedField())
		}
	})

	t.Run("follow leaves the cursor on Frequency without stepping", func(t *testing.T) {
		m := open(false)
		m, _ = m.Update(press(tea.KeyEnter, ""))
		if !m.selector.Visible() {
			t.Fatal("enter on Frequency did not open the chooser")
		}
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"weekly"}})
		if m.frequency != eventkit.FrequencyWeekly {
			t.Fatalf("frequency = %v, want weekly", m.frequency)
		}
		if m.focusedField() != fFrequency {
			t.Fatalf("follow off left the cursor on %v, want Frequency", m.focusedField())
		}
	})

	t.Run("follow steps to the field after the finished chooser", func(t *testing.T) {
		m := open(true)
		m, _ = m.Update(press(tea.KeyEnter, ""))
		if !m.selector.Visible() {
			t.Fatal("enter on Frequency did not open the chooser")
		}
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"weekly"}})
		if m.frequency != eventkit.FrequencyWeekly {
			t.Fatalf("frequency = %v, want weekly", m.frequency)
		}
		if m.focusedField() != fInterval || !m.intervalIn.Focused() {
			t.Fatalf("follow left the cursor on %v, want the interval field", m.focusedField())
		}
	})

	t.Run("follow finds the grown end-date field", func(t *testing.T) {
		m := open(true)
		m, _ = m.Update(press('j', "j"))
		m, _ = m.Update(press('j', "j"))
		if m.focusedField() != fEnd {
			t.Fatalf("cursor on %v, want End repeat", m.focusedField())
		}
		m, _ = m.Update(press(tea.KeyEnter, ""))
		if !m.selector.Visible() {
			t.Fatal("enter on End repeat did not open the chooser")
		}
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"1"}})
		if m.endMode != endOnDate {
			t.Fatalf("end mode = %d, want endOnDate", m.endMode)
		}
		if m.focusedField() != fEndValue {
			t.Fatalf("follow left the cursor on %v, want the end-date input", m.focusedField())
		}
	})

	t.Run("cancelling a chooser leaves the cursor alone", func(t *testing.T) {
		m := open(true)
		m, _ = m.Update(press('j', "j"))
		m, _ = m.Update(press('j', "j"))
		m, _ = m.Update(press(tea.KeyEnter, ""))
		if !m.selector.Visible() {
			t.Fatal("enter on End repeat did not open the chooser")
		}
		before := m.focusIndex
		m, cmd := m.Update(press(tea.KeyEscape, ""))
		msg, ok := mustCmd(t, cmd, "chooser cancel").(selectorCancelledMsg)
		if !ok {
			t.Fatalf("esc produced no selectorCancelledMsg")
		}
		m, _ = m.Update(msg)
		if m.focusIndex != before || m.focusedField() != fEnd {
			t.Fatalf("cancel moved the cursor to index %d (%v)", m.focusIndex, m.focusedField())
		}
	})

	t.Run("ctrl+f toggles follow inside the editor", func(t *testing.T) {
		m := open(false)
		m, _ = m.Update(ctrlPress('f'))
		if !m.follow {
			t.Fatal("ctrl+f did not arm follow in the editor")
		}
		m, _ = m.Update(ctrlPress('f'))
		if m.follow {
			t.Fatal("ctrl+f did not disarm follow in the editor")
		}
	})

	t.Run("the footer keeps the follow state in a narrow pane", func(t *testing.T) {
		m := NewRecurrence(testKeys(t))
		m.SetSize(40, 24)
		m.Show(nil, now, base, true)
		v := m.View()
		if !strings.Contains(v, "follow:off") {
			t.Fatalf("footer clips the armed state: %q", v)
		}
		m, _ = m.Update(ctrlPress('f'))
		v = m.View()
		if !strings.Contains(v, "follow:on") {
			t.Fatalf("footer clips the disarmed state: %q", v)
		}
	})

	t.Run("the editor's toggle reaches the form", func(t *testing.T) {
		cm := newCreateAt(t, now)
		cm = openFieldFor(t, cm, fieldRecurrence)
		cm, cmd := cm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if _, ok := mustCmd(t, cmd, "recurrence chooser").(selectorSelectedMsg); !ok {
			t.Fatal("enter did not open the recurrence chooser")
		}
		cm, _ = cm.Update(selectorSelectedMsg{IDs: []string{"custom"}})
		if !cm.recurrenceE.Visible() {
			t.Fatal("Custom editor did not open")
		}
		if cm.follow {
			t.Fatal("editor opened armed on a form with follow off")
		}
		cm, _ = cm.Update(ctrlPress('f'))
		if !cm.follow {
			t.Fatal("the editor's ctrl+f did not reach the form")
		}
		if v := cm.recurrenceE.View(); !strings.Contains(v, "follow:off") {
			t.Fatalf("editor footer does not show the new mode: %q", v)
		}
	})
}

// TestRecurrenceEnterWalksEveryFieldInFollowMode verifies the new Enter
// contract in the Custom editor: with follow armed it steps to the next
// field and applies from the last one, so a walk needs no Ctrl-S; with
// follow off Enter on a plain field still applies immediately.
func TestRecurrenceEnterWalksEveryFieldInFollowMode(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	base := midnightOf(now)

	open := func(follow bool) RecurrenceModel {
		m := NewRecurrence(testKeys(t))
		m.SetSize(80, 24)
		m.Show(nil, now, base, follow)
		return m
	}

	t.Run("follow advances from Interval instead of applying", func(t *testing.T) {
		m := open(true)
		m, _ = m.Update(press(tea.KeyEnter, ""))
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"weekly"}})
		if m.focusedField() != fInterval {
			t.Fatalf("cursor on %v, want the interval field", m.focusedField())
		}
		m, cmd := m.Update(press(tea.KeyEnter, ""))
		if cmd != nil {
			t.Fatal("Enter on Interval with follow armed must not apply")
		}
		if m.focusedField() != fWeeklyDays {
			t.Fatalf("Enter left the cursor on %v, want the days field", m.focusedField())
		}
		if m.visible != true {
			t.Fatal("the editor closed mid-walk")
		}
	})

	t.Run("follow applies from the last field", func(t *testing.T) {
		m := open(true)
		fs := m.fields()
		m.focusIndex = len(fs) - 1
		m.focusField()
		if got := m.focusedField(); got != fEnd {
			t.Fatalf("last field is %v, want End repeat", got)
		}
		m, cmd := m.Update(press(tea.KeyEnter, ""))
		if !m.selector.Visible() {
			t.Fatal("Enter on the last field must open its chooser")
		}
		m, cmd = m.Update(selectorSelectedMsg{IDs: []string{"0"}})
		msg, ok := mustCmd(t, cmd, "walk apply").(RecurrenceSubmitMsg)
		if !ok {
			t.Fatal("a walk that reaches the last field must apply")
		}
		if len(msg.Rules) != 1 {
			t.Fatalf("applied rules = %+v", msg.Rules)
		}
		if m.visible {
			t.Fatal("applying must close the editor")
		}
	})

	t.Run("follow off still applies from Interval", func(t *testing.T) {
		m := open(false)
		m.focusIndex = 1
		m.focusField()
		if got := m.focusedField(); got != fInterval {
			t.Fatalf("cursor on %v, want the interval field", got)
		}
		m, cmd := m.Update(press(tea.KeyEnter, ""))
		if _, ok := mustCmd(t, cmd, "apply").(RecurrenceSubmitMsg); !ok {
			t.Fatal("Enter on Interval with follow off must apply")
		}
		if m.visible {
			t.Fatal("applying must close the editor")
		}
	})

	t.Run("end offers only Never and On date", func(t *testing.T) {
		m := open(false)
		opts := m.chooserOptions(fEnd)
		if len(opts) != 2 {
			t.Fatalf("end options = %+v, want Never and On date only", opts)
		}
		if opts[0].Label != "Never" || opts[1].Label != "On date" {
			t.Fatalf("end options = %q, %q", opts[0].Label, opts[1].Label)
		}
	})

	t.Run("an occurrence-count rule opens on Never with a warning", func(t *testing.T) {
		rules := []eventkit.RecurrenceRule{{
			Frequency: eventkit.FrequencyWeekly,
			Interval:  2,
			DaysOfTheWeek: []eventkit.RecurrenceDayOfWeek{
				{DayOfTheWeek: eventkit.Monday, WeekNumber: 0},
			},
			End: &eventkit.RecurrenceEnd{OccurrenceCount: 5},
		}}
		p := prefillFields(rules)
		if p == nil {
			t.Fatal("the rule is representable and must prefill")
		}
		if p.endMode != endNever {
			t.Fatalf("end mode = %d, want endNever", p.endMode)
		}
		if p.droppedCount != 5 {
			t.Fatalf("droppedCount = %d, want 5", p.droppedCount)
		}
		m := open(false)
		m.Show(rules, now, base, false)
		if !strings.Contains(m.banner, "ends after 5 occurrences") {
			t.Fatalf("banner = %q, want the dropped-count warning", m.banner)
		}
	})
}

func boolsEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFollowMode covers the create-time follow default, the Enter walk and the
// toggle that disarms it.
func TestFollowMode(t *testing.T) {
	loc := perth(t)
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	t.Run("create opens on the title row", func(t *testing.T) {
		m := newFollowCreate(t, now)
		if !m.follow {
			t.Fatal("follow must be the create default")
		}
		if m.mode != formEditing || m.active != fieldTitle {
			t.Fatalf("create opened at mode %v active %d", m.mode, m.active)
		}
		if !m.titleInput.Focused() {
			t.Fatal("the title input is not focused")
		}
	})

	t.Run("enter walks every row and rests after recurrence", func(t *testing.T) {
		m := newFollowCreate(t, now)
		m, _ = m.Update(tea.PasteMsg{Content: "Walk"})
		m = finishInto(t, m, fieldNotes)
		m = finishInto(t, m, fieldDate)
		m = finishInto(t, m, fieldTime)
		m = finishInto(t, m, fieldPriority)

		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = feedSelectorResult(t, m, cmd)
		if m.mode != formEditing || m.active != fieldRecurrence {
			t.Fatalf("priority did not advance to recurrence: mode %v active %d", m.mode, m.active)
		}
		m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = feedSelectorResult(t, m, cmd)
		if m.mode != formBrowsing || m.selected != fieldRecurrence {
			t.Fatalf("follow did not rest at the recurrence row: mode %v selected %d", m.mode, m.selected)
		}
		if m.titleInput.Value() != "Walk" {
			t.Fatalf("the typed title was lost: %q", m.titleInput.Value())
		}
	})

	t.Run("esc returns to the menu without advancing", func(t *testing.T) {
		m := newFollowCreate(t, now)
		m = finishInto(t, m, fieldNotes)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.mode != formBrowsing || m.selected != fieldNotes {
			t.Fatalf("esc left mode %v selected %d", m.mode, m.selected)
		}
	})

	t.Run("ctrl+f disarms follow without moving the cursor", func(t *testing.T) {
		m := newFollowCreate(t, now)
		m, _ = m.Update(ctrlPress('f'))
		if m.follow {
			t.Fatal("ctrl+f did not disarm follow")
		}
		if m.active != fieldTitle {
			t.Fatalf("the toggle moved the cursor: active %d", m.active)
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.mode != formBrowsing || m.selected != fieldTitle {
			t.Fatalf("enter with follow off did not rest at the menu: mode %v selected %d", m.mode, m.selected)
		}
		if !strings.Contains(m.footerHints(), "follow:on") {
			t.Fatalf("footer does not offer to re-arm follow: %q", m.footerHints())
		}
	})

	t.Run("a cancelled chooser rests at the menu", func(t *testing.T) {
		m := newFollowCreate(t, now)
		m = finishInto(t, m, fieldNotes)
		m = finishInto(t, m, fieldDate)
		m = finishInto(t, m, fieldTime)
		m = finishInto(t, m, fieldPriority)
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		cancelled, ok := mustCmd(t, cmd, "selector cancel").(selectorCancelledMsg)
		if !ok {
			t.Fatalf("esc did not cancel the chooser: %T", cancelled)
		}
		m, _ = m.Update(cancelled)
		if m.mode != formBrowsing || m.selected != fieldPriority {
			t.Fatalf("cancel left mode %v selected %d", m.mode, m.selected)
		}
	})

	t.Run("edit opens in the menu with follow off", func(t *testing.T) {
		m := newEditAt(t, reminders.Reminder{ID: "r", Title: "Edit"}, now)
		if m.follow {
			t.Fatal("edit must not start in follow mode")
		}
		if m.mode != formBrowsing || m.selected != fieldTitle {
			t.Fatalf("edit opened at mode %v selected %d", m.mode, m.selected)
		}
	})
}

// TestCustomRecurrenceLabels covers the summary row describing a custom
// schedule instead of collapsing it to "Custom".
func TestCustomRecurrenceLabels(t *testing.T) {
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, perth(t))
	end := time.Date(2026, time.December, 31, 23, 59, 59, 0, time.Local)

	weekly := func() eventkit.RecurrenceRule {
		return eventkit.Weekly(2, eventkit.Monday, eventkit.Wednesday)
	}
	withEnd := func(r eventkit.RecurrenceRule, e eventkit.RecurrenceEnd) []eventkit.RecurrenceRule {
		r.End = &e
		return []eventkit.RecurrenceRule{r}
	}
	days := func(wd ...eventkit.Weekday) []eventkit.RecurrenceDayOfWeek {
		out := make([]eventkit.RecurrenceDayOfWeek, 0, len(wd))
		for _, w := range wd {
			out = append(out, eventkit.RecurrenceDayOfWeek{DayOfTheWeek: w})
		}
		return out
	}

	cases := []struct {
		name  string
		rules []eventkit.RecurrenceRule
		want  string
	}{
		{"daily", []eventkit.RecurrenceRule{eventkit.Daily(1)}, "Daily"},
		{"daily interval", []eventkit.RecurrenceRule{eventkit.Daily(3)}, "Every 3 days"},
		{"weekly", []eventkit.RecurrenceRule{eventkit.Weekly(1)}, "Weekly"},
		{"weekly interval", []eventkit.RecurrenceRule{eventkit.Weekly(3)}, "Every 3 weeks"},
		{"weekly days", []eventkit.RecurrenceRule{weekly()}, "Every 2 weeks on Mon, Wed"},
		{"monthly", []eventkit.RecurrenceRule{eventkit.Monthly(1)}, "Monthly"},
		{"monthly interval", []eventkit.RecurrenceRule{eventkit.Monthly(4)}, "Every 4 months"},
		{"monthly days", []eventkit.RecurrenceRule{eventkit.Monthly(1, 15, -1)}, "Monthly on 15, Last"},
		{"monthly on the", []eventkit.RecurrenceRule{{
			Frequency:     eventkit.FrequencyMonthly,
			Interval:      1,
			DaysOfTheWeek: []eventkit.RecurrenceDayOfWeek{{DayOfTheWeek: eventkit.Monday, WeekNumber: 1}},
		}}, "Monthly on the first Monday"},
		{"monthly weekdays", []eventkit.RecurrenceRule{{
			Frequency: eventkit.FrequencyMonthly,
			Interval:  1,
			DaysOfTheWeek: days(eventkit.Monday, eventkit.Tuesday, eventkit.Wednesday,
				eventkit.Thursday, eventkit.Friday),
			SetPositions: []int{1},
		}}, "Monthly on the weekdays"},
		{"monthly weekend days", []eventkit.RecurrenceRule{{
			Frequency:     eventkit.FrequencyMonthly,
			Interval:      1,
			DaysOfTheWeek: days(eventkit.Saturday, eventkit.Sunday),
			SetPositions:  []int{1},
		}}, "Monthly on the weekend days"},
		{"yearly same day", []eventkit.RecurrenceRule{{
			Frequency:       eventkit.FrequencyYearly,
			Interval:        1,
			MonthsOfTheYear: []int{10},
		}}, "Every year in October"},
		{"yearly on the", []eventkit.RecurrenceRule{{
			Frequency:       eventkit.FrequencyYearly,
			Interval:        1,
			MonthsOfTheYear: []int{1, 3},
			DaysOfTheWeek:   []eventkit.RecurrenceDayOfWeek{{DayOfTheWeek: eventkit.Monday, WeekNumber: 2}},
		}}, "Every year in January, March on the second Monday"},
		{"until", withEnd(weekly(), eventkit.RecurrenceEnd{EndDate: &end}),
			"Every 2 weeks on Mon, Wed until 2026-12-31"},
		{"count", withEnd(weekly(), eventkit.RecurrenceEnd{OccurrenceCount: 5}),
			"Every 2 weeks on Mon, Wed for 5 occurrences"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCreateAt(t, now)
			m.recurrence = tc.rules
			if got := m.summaryValue(fieldRecurrence); got != tc.want {
				t.Fatalf("label = %q, want %q", got, tc.want)
			}
		})
	}
}
