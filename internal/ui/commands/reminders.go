package commands

import (
	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/messages"
)

func FetchReminders(client *reminders.Client, listID, listName string, showCompleted bool) tea.Cmd {
	return func() tea.Msg {
		items, err := client.Reminders(listName, showCompleted)
		return messages.RemindersLoadedMsg{Reminders: items, ListID: listID, Err: err}
	}
}

func FetchTodayReminders(client *reminders.Client, showCompleted bool) tea.Cmd {
	return func() tea.Msg {
		items, err := client.TodayReminders(showCompleted)
		return messages.RemindersLoadedMsg{Reminders: items, ListID: reminders.SmartListToday, Err: err}
	}
}

func FetchScheduledReminders(client *reminders.Client, showCompleted bool) tea.Cmd {
	return func() tea.Msg {
		items, err := client.ScheduledReminders(showCompleted)
		return messages.RemindersLoadedMsg{Reminders: items, ListID: reminders.SmartListScheduled, Err: err}
	}
}

func FetchCompletedReminders(client *reminders.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := client.CompletedReminders()
		return messages.RemindersLoadedMsg{Reminders: items, ListID: reminders.SmartListCompleted, Err: err}
	}
}

// UncompleteFromCompleted clears a completed reminder's completion and
// makes sure it lands back in its original list, re-creating that list when
// it no longer exists. Completing never moves a reminder, so the only way
// it loses its list is the list being deleted while the reminder sat in the
// Completed view.
func UncompleteFromCompleted(client *reminders.Client, id, listID, listTitle string) tea.Cmd {
	return func() tea.Msg {
		client.InvalidateCompletedCount()
		title, recreated, err := client.EnsureList(listID, listTitle)
		if err != nil {
			return messages.ReminderCompletedMsg{Err: err}
		}
		r, err := client.UncompleteReminder(id)
		if err != nil {
			return messages.ReminderCompletedMsg{Err: err}
		}
		if !recreated {
			return messages.ReminderCompletedMsg{Reminder: r, Err: nil}
		}
		if _, err := client.UpdateReminder(id, reminders.UpdateReminderInput{ListName: &title}); err != nil {
			return messages.ReminderCompletedMsg{Err: err}
		}
		moved, err := client.Reminder(id)
		if err != nil {
			return messages.ReminderCompletedMsg{Err: err}
		}
		return messages.ReminderCompletedMsg{Reminder: moved, Err: nil}
	}
}

func CreateReminder(client *reminders.Client, input reminders.CreateReminderInput) tea.Cmd {
	return func() tea.Msg {
		r, err := client.CreateReminder(input)
		return messages.ReminderCreatedMsg{Reminder: r, Err: err}
	}
}

func UpdateReminder(client *reminders.Client, id string, input reminders.UpdateReminderInput) tea.Cmd {
	return func() tea.Msg {
		r, err := client.UpdateReminder(id, input)
		return messages.ReminderUpdatedMsg{Reminder: r, Err: err}
	}
}

func ToggleComplete(client *reminders.Client, id string, currentlyCompleted bool) tea.Cmd {
	return func() tea.Msg {
		var r *reminders.Reminder
		var err error
		if currentlyCompleted {
			r, err = client.UncompleteReminder(id)
		} else {
			r, err = client.CompleteReminder(id)
		}
		// The sidebar badge counts completions, so it can never wait out
		// its TTL after a toggle.
		client.InvalidateCompletedCount()
		return messages.ReminderCompletedMsg{Reminder: r, Err: err}
	}
}

func DeleteReminder(client *reminders.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteReminder(id)
		return messages.ReminderDeletedMsg{ID: id, Err: err}
	}
}

func MoveReminder(client *reminders.Client, id, listName string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.UpdateReminder(id, reminders.UpdateReminderInput{ListName: &listName})
		return messages.ReminderMovedMsg{ListTitle: listName, Err: err}
	}
}
