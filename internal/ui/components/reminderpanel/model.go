package reminderpanel

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Model struct {
	list    list.Model
	focused bool
	keys    keybind.Map
	ace     *aceState
	drag    *dragState
}

func New(width, height int) Model {
	delegate := NewDelegate()

	l := list.New([]list.Item{}, delegate, width, height)
	l.Title = "Reminders"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	m := Model{list: l, keys: keybind.Map{}, ace: delegate.ace, drag: delegate.drag}
	m.SetDarkBackground(true)
	return m
}

// SetKeys projects the configured list/filter/textinput bindings into the
// underlying Bubbles list, paginator and filter input keymaps. Disabled
// actions receive empty bindings so they never match.
func (m *Model) SetKeys(keys keybind.Map) {
	m.keys = keys
	km := &m.list.KeyMap
	km.CursorUp = key.NewBinding(key.WithKeys(keys.Aliases("list", "up")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "up")), "up"))
	km.CursorDown = key.NewBinding(key.WithKeys(keys.Aliases("list", "down")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "down")), "down"))
	km.GoToStart = key.NewBinding(key.WithKeys(keys.Aliases("list", "first")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "first")), "first"))
	km.GoToEnd = key.NewBinding(key.WithKeys(keys.Aliases("list", "last")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "last")), "last"))
	km.PrevPage = key.NewBinding(key.WithKeys(keys.Aliases("list", "previous_page")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "previous_page")), "prev page"))
	km.NextPage = key.NewBinding(key.WithKeys(keys.Aliases("list", "next_page")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "next_page")), "next page"))
	km.Filter = key.NewBinding(key.WithKeys(keys.Aliases("list", "filter")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "filter")), "filter"))
	km.ClearFilter = key.NewBinding(key.WithKeys(keys.Aliases("list", "clear_filter")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("list", "clear_filter")), "clear filter"))
	km.AcceptWhileFiltering = key.NewBinding(key.WithKeys(keys.Aliases("filter", "accept")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("filter", "accept")), "apply filter"))
	km.CancelWhileFiltering = key.NewBinding(key.WithKeys(keys.Aliases("filter", "cancel")...), key.WithHelp(keybind.ShortKeys(keys.Aliases("filter", "cancel")), "cancel"))

	// Shortcuts not projected from the registry stay disabled: quit and
	// full-help belong to the root.
	km.Quit = key.NewBinding()
	km.ForceQuit = key.NewBinding()
	km.ShowFullHelp = key.NewBinding()
	km.CloseFullHelp = key.NewBinding()

	// Project page arrays to the paginator too.
	pk := &m.list.Paginator.KeyMap
	pk.PrevPage = key.NewBinding(key.WithKeys(keys.Aliases("list", "previous_page")...))
	pk.NextPage = key.NewBinding(key.WithKeys(keys.Aliases("list", "next_page")...))

	// Project textinput keys into the filter input.
	fi := &m.list.FilterInput.KeyMap
	projectTextinputKeys(fi, keys)
}

func projectTextinputKeys(km *textinput.KeyMap, keys keybind.Map) {
	km.CharacterForward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "character_forward")...))
	km.CharacterBackward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "character_backward")...))
	km.WordForward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "word_forward")...))
	km.WordBackward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "word_backward")...))
	km.DeleteWordBackward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_word_backward")...))
	km.DeleteWordForward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_word_forward")...))
	km.DeleteAfterCursor = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_after_cursor")...))
	km.DeleteBeforeCursor = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_before_cursor")...))
	km.DeleteCharacterBackward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_character_backward")...))
	km.DeleteCharacterForward = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "delete_character_forward")...))
	km.LineStart = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "line_start")...))
	km.LineEnd = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "line_end")...))
	km.Paste = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "paste")...))
	km.AcceptSuggestion = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "accept_suggestion")...))
	km.NextSuggestion = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "next_suggestion")...))
	km.PrevSuggestion = key.NewBinding(key.WithKeys(keys.Aliases("textinput", "previous_suggestion")...))
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
	prevID := ""
	if r, ok := m.SelectedReminder(); ok {
		prevID = r.ID
	}
	prevIndex := m.list.Index()
	listItems := make([]list.Item, len(items))
	for i, r := range items {
		listItems[i] = Item{Reminder: r}
	}
	// Re-applying an active filter synchronously keeps the filtered view
	// intact across refreshes instead of flashing empty until the async
	// filter command lands.
	if cmd := m.list.SetItems(listItems); cmd != nil {
		if msg := cmd(); msg != nil {
			m.list, _ = m.list.Update(msg)
		}
	}
	// Keep the selection on the same reminder across refreshes so external
	// deletions above the cursor don't shift the selection.
	if prevID != "" && m.SelectID(prevID) {
		return
	}
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

// SetAceLabels installs the active ace-jump labels (keyed by stable
// reminder ID) for the delegate to render. An empty or nil map clears all
// labels.
func (m *Model) SetAceLabels(labels map[string]string) {
	if len(labels) == 0 {
		m.ace.labels = nil
		m.ace.prefix = ""
		return
	}
	m.ace.labels = make(map[string]string, len(labels))
	for id, label := range labels {
		m.ace.labels[id] = label
	}
}

// SetAcePrefix updates the typed ace prefix so nonmatching labels dim.
func (m *Model) SetAcePrefix(prefix string) {
	m.ace.prefix = prefix
}

// VisibleIDs returns the stable IDs of the rows currently visible on the
// active page. With an active filter it enumerates the filtered items.
func (m Model) VisibleIDs() []string {
	items := m.list.VisibleItems()
	start, end := m.list.Paginator.GetSliceBounds(len(items))
	ids := make([]string, 0, end-start)
	for i := start; i < end && i < len(items); i++ {
		if it, ok := items[i].(Item); ok {
			ids = append(ids, it.Reminder.ID)
		}
	}
	return ids
}

// SelectID moves the selection to the item with the given stable ID among
// the rows visible on the active page (the ace-eligible rows) and reports
// whether it was found.
func (m *Model) SelectID(id string) bool {
	items := m.list.VisibleItems()
	start, end := m.list.Paginator.GetSliceBounds(len(items))
	for i := start; i < end && i < len(items); i++ {
		if it, ok := items[i].(Item); ok && it.Reminder.ID == id {
			m.list.Select(i)
			return true
		}
	}
	return false
}

// SetDragging marks the reminder currently lifted by a mouse drag. An
// empty id clears the lifted marker.
func (m *Model) SetDragging(id string) {
	m.drag.id = id
}

// listContentTopRows is the title bar plus its bottom padding; item i then
// occupies rows [listContentTopRows + i*2, +2) because the delegate is
// Height() 2 / Spacing() 0.
const listContentTopRows = 2

// IDAtContentRow returns the stable ID rendered on the given panel-content
// row (0 = the first row of the list view), or ("", false) for the title
// bar, the pagination strip, a gap, or an empty list.
func (m Model) IDAtContentRow(row int) (string, bool) {
	if row < listContentTopRows {
		return "", false
	}
	items := m.list.VisibleItems()
	start, end := m.list.Paginator.GetSliceBounds(len(items))
	i := (row - listContentTopRows) / 2
	if i < 0 || start+i >= end {
		return "", false
	}
	it, ok := items[start+i].(Item)
	if !ok {
		return "", false
	}
	return it.Reminder.ID, true
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
