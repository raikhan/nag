package dialog

import (
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

func tabs(m CreateModel, n int) CreateModel {
	for range n {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	return m
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

// TestV2DialogInputAndPaste covers empty submit, key release, paste-as-text,
// backward field navigation and the confirm/list dialogs.
func TestV2DialogInputAndPaste(t *testing.T) {
	keys := testKeys(t)

	// Create-reminder dialog: empty Enter keeps it open with no submit.
	cm := NewCreate()
	cm.SetKeys(keys)
	cm.Show("Alpha")
	if !cm.Visible() {
		t.Fatal("create dialog did not open")
	}
	cm2, cmd := cm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("empty enter submitted: got cmd %v", cmd)
	}
	if !cm2.Visible() {
		t.Fatal("empty enter closed the dialog")
	}

	// Releasing Enter does not submit either.
	cm3, cmd2 := cm2.Update(tea.KeyReleaseMsg{Code: tea.KeyEnter})
	if cmd2 != nil {
		t.Fatalf("enter release submitted: got cmd %v", cmd2)
	}
	if !cm3.Visible() {
		t.Fatal("enter release closed the dialog")
	}

	// Pasted characters are text, not global actions.
	cm4, _ := cm3.Update(tea.PasteMsg{Content: "q ? n x"})
	if got := cm4.titleInput.Value(); got != "q ? n x" {
		t.Fatalf("paste produced title %q", got)
	}

	// Enter submits the pasted title and closes the dialog.
	cm5, cmd3 := cm4.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd3 == nil {
		t.Fatal("expected submit command after enter")
	}
	if cm5.Visible() {
		t.Fatal("dialog still visible after submit")
	}
	msg := mustCmd(t, cmd3, "submit")
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

	// Shift-Tab moves focus to the preceding field: pasted text must land
	// in the title field.
	fm := NewCreate()
	fm.SetKeys(keys)
	fm.Show("Alpha")
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // notes
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
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

// TestCreateEditDatePriorityRecurrence exercises the five-field dialog.
func TestCreateEditDatePriorityRecurrence(t *testing.T) {
	keys := testKeys(t)

	t.Run("calendar movement produces submitted due date", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = tabs(m, 2) // due date focused
		m.picker.SetValue("")
		m, _ = m.Update(tea.PasteMsg{Content: "2026-10-04"})
		if _, err := m.picker.Resolve(); err != nil {
			t.Fatalf("typed date invalid: %v", err)
		}
		m, _ = m.Update(ctrlPress('l')) // right: +1 day
		m, _ = m.Update(ctrlPress('j')) // down: +7 days
		m, msg := ctrlS(t, m)
		_ = msg
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
		m = tabs(m, 2)
		m, _ = m.Update(ctrlPress('j')) // +7 days from the edit base
		if m.picker.Value() != "2026-10-11 09:00" {
			t.Fatalf("calendar down wrote %q", m.picker.Value())
		}
		m, msg := ctrlS(t, m)
		_ = msg
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
		m = tabs(m, 2)
		m, _ = m.Update(tea.PasteMsg{Content: "not a date"})
		_, cmd := m.Update(ctrlPress('s'))
		if cmd != nil {
			t.Fatalf("invalid due submitted: %v", cmd)
		}
		if !m.Visible() {
			t.Fatal("invalid due closed the dialog")
		}
		if m.picker.Error() == "" {
			t.Fatal("no inline error for invalid due")
		}
	})

	t.Run("priority fuzzy selection", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.Show("Alpha")
		m, _ = m.Update(tea.PasteMsg{Content: "Priority smoke"})
		m = tabs(m, 3)                                       // priority
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // open chooser
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
		m, _ = m.Update(sel) // consume, m.titleInput.Value(), m.focusIndex)
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
		m, _ = m.Update(tea.PasteMsg{Content: "Repeating"})
		m = tabs(m, 2)
		m.picker.SetValue("2026-10-04")
		m = tabs(m, 2) // recurrence
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"daily"}})
		m, msg := ctrlS(t, m)
		sub := msg.(CreateSubmitMsg)
		if len(sub.Input.RecurrenceRules) != 1 ||
			sub.Input.RecurrenceRules[0].Frequency != eventkit.FrequencyDaily ||
			sub.Input.RecurrenceRules[0].Interval != 1 {
			t.Fatalf("rules = %+v, want Daily(1)", sub.Input.RecurrenceRules)
		}
	})

	t.Run("custom weekly mon fri interval 3 count 10", func(t *testing.T) {
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(editFixture())
		m = tabs(m, 4)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // presets, empty query
		m, _ = m.Update(press('c', "c"))                     // Custom…
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
		// End -> after occurrences = 10.
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(selectorSelectedMsg{IDs: []string{"2"}}) // after occurrences
		m, _ = m.Update(press('1', "1"))
		m, _ = m.Update(press('0', "0"))

		m, cmd = m.Update(ctrlPress('s')) // apply editor
		submit := mustCmd(t, cmd, "editor apply").(RecurrenceSubmitMsg)
		rules := submit.Rules
		if len(rules) != 1 || rules[0].Frequency != eventkit.FrequencyWeekly ||
			rules[0].Interval != 3 || len(rules[0].DaysOfTheWeek) != 2 ||
			rules[0].End == nil || rules[0].End.OccurrenceCount != 10 {
			t.Fatalf("draft rules = %+v", rules)
		}
		for _, d := range rules[0].DaysOfTheWeek {
			if d.DayOfTheWeek != eventkit.Monday && d.DayOfTheWeek != eventkit.Friday || d.WeekNumber != 0 {
				t.Fatalf("unexpected day %+v", d)
			}
		}

		// The outer form consumes the editor result and submits it.
		m, _ = m.Update(submit)
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
		m = tabs(m, 4)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	t.Run("untouched imported custom recurrence preserves rules and due", func(t *testing.T) {
		r := editFixture()
		r.RecurrenceRules = []eventkit.RecurrenceRule{eventkit.Weekly(2, eventkit.Saturday)}
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m, _ = m.Update(tea.PasteMsg{Content: " Renamed"}) // title-only edit
		m, msg := ctrlS(t, m)
		_ = msg
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
		m = tabs(m, 4)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
		m = tabs(m, 4)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if _, ok := mustCmd(t, cmd, "chooser cancel").(selectorCancelledMsg); !ok {
			t.Fatalf("cancel produced %T", cmd())
		}
		m, _ = m.Update(cmd) // consume the cancel message
		m, msg := ctrlS(t, m)
		edit := msg.(EditSubmitMsg)
		if edit.Input.RecurrenceRules != nil {
			t.Fatalf("cancel patched recurrence: %+v", *edit.Input.RecurrenceRules)
		}
	})

	t.Run("repeating reminders need a due date", func(t *testing.T) {
		r := editFixture()
		r.DueDate = nil
		m := NewCreate()
		m.SetKeys(keys)
		m.ShowEdit(r)
		m = tabs(m, 4)
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
}
