package reminders

import ekreminders "github.com/BRO3886/go-eventkit/reminders"

const (
	SmartListToday     = "__smart_today__"
	SmartListScheduled = "__smart_scheduled__"
	SmartListCompleted = "__smart_completed__"
)

func (c *Client) Lists() ([]ReminderList, error) {
	ekLists, err := c.ek.Lists()
	if err != nil {
		return nil, err
	}

	// Single query for all incomplete reminders, then count by list ID
	incompleteCounts := make(map[string]int)
	if all, err := c.ek.Reminders(ekreminders.WithCompleted(false)); err == nil {
		for _, r := range all {
			incompleteCounts[r.ListID]++
		}
	}

	lists := make([]ReminderList, len(ekLists))
	for i, l := range ekLists {
		lists[i] = ReminderList{
			ID:    l.ID,
			Title: l.Title,
			Color: l.Color,
			Count: incompleteCounts[l.ID],
		}
	}
	return lists, nil
}

// CreateList creates a list, optionally with a hex display colour. An
// empty colour is omitted so EventKit assigns its default.
func (c *Client) CreateList(title, color string) error {
	_, err := c.ek.CreateList(ekreminders.CreateListInput{Title: title, Color: color})
	return err
}

func (c *Client) DeleteList(id string) error {
	return c.ek.DeleteList(id)
}

// UpdateList renames a list and, when color is non-empty, sets its display
// colour. An empty colour is never transmitted, so a rename leaves the
// stored hue untouched.
func (c *Client) UpdateList(id string, title string, color string) error {
	input := ekreminders.UpdateListInput{Title: &title}
	if color != "" {
		input.Color = &color
	}
	_, err := c.ek.UpdateList(id, input)
	return err
}

// ListsWithSmart returns smart lists (Today, Scheduled, Completed) +
// separator + normal lists.
func (c *Client) ListsWithSmart() ([]ReminderList, error) {
	normal, err := c.Lists()
	if err != nil {
		return nil, err
	}

	todayItems, _ := c.TodayReminders(false)
	scheduledItems, _ := c.ScheduledReminders(false)

	smart := []ReminderList{
		{ID: SmartListToday, Title: "Today", Count: len(todayItems), Kind: ListSmart},
		{ID: SmartListScheduled, Title: "Scheduled", Count: len(scheduledItems), Kind: ListSmart},
		{ID: SmartListCompleted, Title: "Completed", Count: c.CompletedCount(), Kind: ListSmart},
		{Kind: ListSeparator},
	}

	return append(smart, normal...), nil
}
