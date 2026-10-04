package listpanel

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Item struct {
	List reminders.ReminderList
}

func (i Item) Title() string       { return i.List.Title }
func (i Item) Description() string { return fmt.Sprintf("%d reminders", i.List.Count) }
func (i Item) FilterValue() string {
	if i.List.Kind == reminders.ListSeparator {
		return ""
	}
	return i.List.Title
}

// aceState holds the current ace-jump overlay: a label per stable list ID
// and the typed prefix used to dim nonmatching labels.
type aceState struct {
	labels map[string]string
	prefix string
}

// Delegate renders list items with smart list styling and separators.
type Delegate struct {
	ace *aceState
}

func newDelegate() Delegate {
	return Delegate{ace: &aceState{}}
}

func (d Delegate) Height() int  { return 2 }
func (d Delegate) Spacing() int { return 0 }
func (d Delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d Delegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(Item)
	if !ok {
		return
	}

	rl := item.List

	if rl.Kind == reminders.ListSeparator {
		sep := styles.ReminderDimStyle.Render(strings.Repeat("─", m.Width()))
		_, _ = fmt.Fprint(w, "\n"+sep)
		return
	}

	selected := index == m.Index()

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(styles.Teal).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(styles.DimGray)
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(styles.White)
		descStyle = lipgloss.NewStyle().Foreground(styles.DimGray)
	}

	// Smart lists get an icon
	title := rl.Title
	if rl.Kind == reminders.ListSmart {
		switch rl.ID {
		case reminders.SmartListToday:
			title = "◉ " + title
		case reminders.SmartListScheduled:
			title = "▦ " + title
		}
	}

	// Ace-jump label, if any. It consumes width (before MaxWidth
	// truncation), never an extra row, and dims when it no longer
	// matches the typed prefix.
	var label string
	if d.ace != nil && d.ace.labels != nil {
		if lab, ok := d.ace.labels[rl.ID]; ok {
			if d.ace.prefix != "" && !strings.HasPrefix(lab, d.ace.prefix) {
				label = lipgloss.NewStyle().Foreground(styles.DimGray).Render("["+lab+"]") + " "
			} else {
				label = lipgloss.NewStyle().Foreground(styles.Teal).Bold(true).Render("["+lab+"]") + " "
			}
		}
	}

	line1 := label + titleStyle.Render(title)
	line2 := descStyle.Render(fmt.Sprintf("%d reminders", rl.Count))

	var style lipgloss.Style
	if selected {
		style = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(styles.Teal).
			PaddingLeft(1)
	} else {
		style = lipgloss.NewStyle().PaddingLeft(2)
	}

	style = style.MaxWidth(m.Width())
	_, _ = fmt.Fprint(w, style.Render(line1+"\n"+line2))
}

type Model struct {
	list    list.Model
	focused bool
	keys    keybind.Map
	ace     *aceState
}

func New(width, height int) Model {
	d := newDelegate()
	l := list.New([]list.Item{}, d, width, height)
	l.Title = "Lists"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	m := Model{list: l, keys: keybind.Map{}, ace: d.ace}
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
	// full-help belong to the root, and the default page aliases
	// (b/u/f/d) conflict with root actions.
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

func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

func (m *Model) SetLists(lists []reminders.ReminderList) {
	prevIndex := m.list.Index()
	items := make([]list.Item, len(lists))
	for i, l := range lists {
		items[i] = Item{List: l}
	}
	// Re-applying an active filter synchronously keeps the filtered view
	// intact across refreshes instead of flashing empty until the async
	// filter command lands.
	if cmd := m.list.SetItems(items); cmd != nil {
		if msg := cmd(); msg != nil {
			m.list, _ = m.list.Update(msg)
		}
	}
	if prevIndex > 0 && prevIndex < len(items) {
		m.list.Select(prevIndex)
	}
	m.normalizeSelection(dirNone)
}

func (m Model) SelectedList() (reminders.ReminderList, bool) {
	item, ok := m.list.SelectedItem().(Item)
	if !ok {
		return reminders.ReminderList{}, false
	}
	// Don't select separators
	if item.List.Kind == reminders.ListSeparator {
		return reminders.ReminderList{}, false
	}
	return item.List, true
}

// Lists returns the sidebar rows currently loaded, in display order.
func (m Model) Lists() []reminders.ReminderList {
	items := m.list.Items()
	result := make([]reminders.ReminderList, 0, len(items))
	for _, item := range items {
		if li, ok := item.(Item); ok {
			result = append(result, li.List)
		}
	}
	return result
}

func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// SetAceLabels installs the active ace-jump labels (keyed by stable list
// ID) for the delegate to render. An empty or nil map clears all labels.
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

// VisibleIDs returns the stable IDs of the selectable rows currently
// visible on the active page, skipping separators. With an active filter it
// enumerates the filtered items.
func (m Model) VisibleIDs() []string {
	items := m.list.VisibleItems()
	start, end := m.list.Paginator.GetSliceBounds(len(items))
	ids := make([]string, 0, end-start)
	for i := start; i < end && i < len(items); i++ {
		if it, ok := items[i].(Item); ok && it.List.Kind != reminders.ListSeparator {
			ids = append(ids, it.List.ID)
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
		if it, ok := items[i].(Item); ok && it.List.ID == id && it.List.Kind != reminders.ListSeparator {
			m.list.Select(i)
			return true
		}
	}
	return false
}

// selDir records the movement direction of the last navigation event so
// selection normalization can pick the nearest selectable row in that
// direction.
type selDir int

const (
	dirNone selDir = iota
	dirPrev
	dirNext
)

func (m Model) navigationDir(msg tea.KeyPressMsg) selDir {
	switch {
	case key.Matches(msg, m.keys.Bind("list", "up")),
		key.Matches(msg, m.keys.Bind("list", "first")),
		key.Matches(msg, m.keys.Bind("list", "previous_page")),
		key.Matches(msg, m.keys.Bind("list", "half_page_up")):
		return dirPrev
	case key.Matches(msg, m.keys.Bind("list", "down")),
		key.Matches(msg, m.keys.Bind("list", "last")),
		key.Matches(msg, m.keys.Bind("list", "next_page")),
		key.Matches(msg, m.keys.Bind("list", "half_page_down")):
		return dirNext
	}
	return dirNone
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		if _, ok := msg.(tea.MouseMsg); !ok {
			return m, nil
		}
	}
	if !m.Filtering() {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			// Half-page movement is owned here: Bubbles has no
			// corresponding binding.
			if key.Matches(keyMsg, m.keys.Bind("list", "half_page_up")) {
				return m.halfPage(dirPrev)
			}
			if key.Matches(keyMsg, m.keys.Bind("list", "half_page_down")) {
				return m.halfPage(dirNext)
			}
		}
	}

	var dir selDir
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		dir = m.navigationDir(msg)
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			dir = dirPrev
		case tea.MouseWheelDown:
			dir = dirNext
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if _, ok := msg.(tea.KeyPressMsg); ok {
		m.normalizeSelection(dir)
		return m, cmd
	}
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		// Wheel-style single-step movement: Bubbles lists do not handle
		// mouse wheel messages themselves.
		switch wheel.Button {
		case tea.MouseWheelUp:
			m.list.CursorUp()
			m.normalizeSelection(dir)
		case tea.MouseWheelDown:
			m.list.CursorDown()
			m.normalizeSelection(dir)
		}
		return m, cmd
	}
	return m, cmd
}

// halfPage moves the selection by half a page in the given direction, then
// normalizes so no separator ends up selected.
func (m Model) halfPage(dir selDir) (Model, tea.Cmd) {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		return m, nil
	}
	step := max(1, m.list.Paginator.PerPage/2)
	target := m.list.Index()
	if dir == dirPrev {
		target -= step
	} else {
		target += step
	}
	target = min(max(target, 0), len(items)-1)
	m.list.Select(target)
	m.normalizeSelection(dir)
	return m, nil
}

// normalizeSelection ensures the current selection is never a separator
// item. It scans at most the visible-item count, picks the nearest
// selectable row in the movement direction (falling back to the opposite
// direction), and prefers following rows when no direction is known. Empty
// or separator-only item sets leave the selection unchanged and
// SelectedList returns false, making actions no-ops.
func (m *Model) normalizeSelection(dir selDir) {
	items := m.list.VisibleItems()
	n := len(items)
	if n == 0 {
		return
	}
	cur := m.list.Index()
	if cur >= 0 && cur < n && selectable(items[cur]) {
		return
	}
	if dir == dirNone {
		if i := scanSelectable(items, cur, 1); i >= 0 {
			m.list.Select(i)
			return
		}
		if i := scanSelectable(items, cur, -1); i >= 0 {
			m.list.Select(i)
		}
		return
	}
	if i := scanSelectable(items, cur, dirStep(dir)); i >= 0 {
		m.list.Select(i)
		return
	}
	if i := scanSelectable(items, cur, -dirStep(dir)); i >= 0 {
		m.list.Select(i)
	}
}

func dirStep(dir selDir) int {
	if dir == dirPrev {
		return -1
	}
	return 1
}

// scanSelectable walks from index+step, capped at the visible-item count,
// and returns the first selectable index or -1.
func scanSelectable(items []list.Item, from, step int) int {
	i := from
	for range len(items) {
		i += step
		if i < 0 || i >= len(items) {
			return -1
		}
		if selectable(items[i]) {
			return i
		}
	}
	return -1
}

func selectable(item list.Item) bool {
	it, ok := item.(Item)
	return ok && it.List.Kind != reminders.ListSeparator
}

func (m Model) View() string {
	return m.list.View()
}
