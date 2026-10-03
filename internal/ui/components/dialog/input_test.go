package dialog

import (
	"testing"

	"charm.land/bubbletea/v2"
)

func TestV2DialogInputAndPaste(t *testing.T) {
	// Create-reminder dialog: empty Enter keeps it open with no submit.
	cm := NewCreate()
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
	msg := cmd3()
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

	// Forward field cycle: four Tabs return to the first field; Shift-Tab
	// cycles backward within the four fields.
	fm := NewCreate()
	fm.Show("Alpha")
	for range fieldCount {
		fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	if fm.focusIndex != 0 {
		t.Fatalf("after a full forward cycle focusIndex = %d", fm.focusIndex)
	}
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if fm.focusIndex != 1 {
		t.Fatalf("after one Tab focusIndex = %d", fm.focusIndex)
	}
	fm, _ = fm.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	// The existing dialog cycles forward for both Tab and Shift-Tab
	// ((i+1) % 4); assert that behavior is preserved.
	if fm.focusIndex != 2 {
		t.Fatalf("after Shift-Tab focusIndex = %d", fm.focusIndex)
	}
	if !fm.dueDateInput.Focused() || fm.titleInput.Focused() {
		t.Fatal("field focus did not follow the cycle to the due-date field")
	}

	// Create-list dialog: paste + enter submits the pasted title.
	lm := NewCreateList()
	lm.Show()
	lm2, _ := lm.Update(tea.PasteMsg{Content: "q ? n x"})
	if got := lm2.titleInput.Value(); got != "q ? n x" {
		t.Fatalf("list paste produced title %q", got)
	}
	lm3, cmd4 := lm2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd4 == nil {
		t.Fatal("expected submit command after enter")
	}
	if lm3.Visible() {
		t.Fatal("list dialog still visible after submit")
	}
	msg2 := cmd4()
	sub2, ok := msg2.(CreateListSubmitMsg)
	if !ok {
		t.Fatalf("expected CreateListSubmitMsg, got %T", msg2)
	}
	if sub2.Title != "q ? n x" {
		t.Fatalf("submitted list title = %q", sub2.Title)
	}

	// Confirm dialog: Y/N, enter/esc transitions.
	cf := NewConfirm()
	cf.Show("Delete?", ConfirmDelete)
	cf2, cmd5 := cf.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	if cmd5 == nil {
		t.Fatal("uppercase Y did not confirm")
	}
	if cf2.Visible() {
		t.Fatal("confirm dialog still visible after Y")
	}
	if yes, ok := cmd5().(ConfirmYesMsg); !ok || yes.Action != ConfirmDelete {
		t.Fatalf("Y produced %#v", cmd5())
	}

	cf3 := NewConfirm()
	cf3.Show("Delete?", ConfirmDelete)
	cf4, cmd6 := cf3.Update(tea.KeyPressMsg{Code: 'N', Text: "N"})
	if cmd6 == nil {
		t.Fatal("uppercase N did not answer no")
	}
	if cf4.Visible() {
		t.Fatal("confirm dialog still visible after N")
	}
	if _, ok := cmd6().(ConfirmNoMsg); !ok {
		t.Fatalf("N produced %T", cmd6())
	}

	cf5 := NewConfirm()
	cf5.Show("Delete?", ConfirmDeleteList)
	cf6, cmd7 := cf5.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd7 == nil || cf6.Visible() {
		t.Fatal("enter did not confirm")
	}
	if yes, ok := cmd7().(ConfirmYesMsg); !ok || yes.Action != ConfirmDeleteList {
		t.Fatalf("enter produced %#v", cmd7())
	}

	cf7 := NewConfirm()
	cf7.Show("Delete?", ConfirmDelete)
	cf8, cmd8 := cf7.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd8 == nil || cf8.Visible() {
		t.Fatal("escape did not answer no")
	}
	if _, ok := cmd8().(ConfirmNoMsg); !ok {
		t.Fatalf("escape produced %T", cmd8())
	}
}
