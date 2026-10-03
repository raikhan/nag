package reminderpanel

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Model struct {
	list    list.Model
	focused bool
}

func New(width, height int) Model {
	delegate := NewDelegate()

	l := list.New([]list.Item{}, delegate, width, height)
	l.Title = "Reminders"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	m := Model{list: l}
	m.SetDarkBackground(true)
	return m
}

// SetDarkBackground reapplies the list's styles for a dark or light
// terminal background without touching items, selection, or filter state.
func (m *Model) SetDarkBackground(isDark bool) {
	m.list.Styles = list.DefaultStyles(isDark)
	m.list.Styles.Title = styles.TitleStyle
	m.list.FilterInput.SetStyles(m.list.Styles.Filter)
	m.list.Paginator.ActiveDot = m.list.Styles.ActivePaginationDot.String()
	m.list.Paginator.InactiveDot = m.list.Styles.InactivePaginationDot.String()
}

func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

func (m *Model) SetTitle(title string) {
	m.list.Title = title
}

func (m *Model) SetReminders(items []reminders.Reminder) {
	prevIndex := m.list.Index()
	listItems := make([]list.Item, len(items))
	for i, r := range items {
		listItems[i] = Item{Reminder: r}
	}
	m.list.SetItems(listItems)
	if prevIndex > 0 && prevIndex < len(listItems) {
		m.list.Select(prevIndex)
	}
}

func (m *Model) UpdateReminder(updated reminders.Reminder) {
	items := m.list.Items()
	for i, item := range items {
		if ri, ok := item.(Item); ok && ri.Reminder.ID == updated.ID {
			items[i] = Item{Reminder: updated}
			break
		}
	}
	idx := m.list.Index()
	m.list.SetItems(items)
	m.list.Select(idx)
}

func (m Model) Reminders() []reminders.Reminder {
	items := m.list.Items()
	result := make([]reminders.Reminder, 0, len(items))
	for _, item := range items {
		if ri, ok := item.(Item); ok {
			result = append(result, ri.Reminder)
		}
	}
	return result
}

func (m Model) SelectedReminder() (reminders.Reminder, bool) {
	item, ok := m.list.SelectedItem().(Item)
	if !ok {
		return reminders.Reminder{}, false
	}
	return item.Reminder, true
}

func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		if _, ok := msg.(tea.MouseMsg); !ok {
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return m.list.View()
}
