package commands

import (
	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/messages"
)

func FetchLists(client *reminders.Client) tea.Cmd {
	return func() tea.Msg {
		lists, err := client.ListsWithSmart()
		return messages.ListsLoadedMsg{Lists: lists, Err: err}
	}
}

func CreateList(client *reminders.Client, title, color string) tea.Cmd {
	return func() tea.Msg {
		err := client.CreateList(title, color)
		return messages.ListCreatedMsg{Err: err}
	}
}

func DeleteList(client *reminders.Client, id string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteList(id)
		return messages.ListDeletedMsg{Err: err}
	}
}

func UpdateList(client *reminders.Client, id string, title string, color string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdateList(id, title, color)
		return messages.ListUpdatedMsg{Err: err}
	}
}
