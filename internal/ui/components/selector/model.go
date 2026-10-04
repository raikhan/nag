package selector

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
	"github.com/oronbz/nag/internal/ui/styles"
)

// Option is one selectable choice in the shared fuzzy popup.
type Option struct {
	ID          string
	Label       string
	Description string
}

// SelectedMsg is emitted on confirm with the selected option IDs.
type SelectedMsg struct{ IDs []string }

// CancelledMsg is emitted when the chooser is dismissed without a choice.
type CancelledMsg struct{}

// item adapts an Option to the Bubbles list item interface.
type item struct {
	opt      Option
	selected bool
}

func (i item) Title() string {
	if i.selected {
		return "☑ " + i.opt.Label
	}
	return i.opt.Label
}

func (i item) Description() string { return i.opt.Description }

func (i item) FilterValue() string {
	v := i.opt.Label
	if i.opt.Description != "" {
		v += " " + i.opt.Description
	}
	return v
}

// purpose identifies why the chooser is open; it is owned by the containing
// editor and retained until its SelectedMsg/CancelledMsg is consumed.
type purpose int

const (
	purposeClosed purpose = iota
	purposeOpen
)

// Model is the shared fuzzy chooser built on a Bubbles list plus a separate
// always-focused query input.
type Model struct {
	keys      keybind.Map
	list      list.Model
	query     textinput.Model
	options   []Option
	committed map[string]bool
	multi     bool
	purpose   purpose
	init      bool
	width     int
	height    int
}

// lineDelegate renders each option as a single line; the highlighted row is
// the selection indicator for single-select choosers.
type lineDelegate struct{}

func (d lineDelegate) Height() int  { return 1 }
func (d lineDelegate) Spacing() int { return 0 }
func (d lineDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d lineDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok {
		return
	}
	style := lipgloss.NewStyle()
	if index == m.Index() {
		style = style.Foreground(styles.Teal).Bold(true)
	}
	fmt.Fprint(w, style.Render(it.Title()))
}

// New builds a chooser with the given compiled bindings.
func New(keys keybind.Map) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter…"
	ti.Prompt = "> "
	ti.CharLimit = 64
	ti.Focus()

	l := list.New(nil, lineDelegate{}, 40, 12)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)
	l.SetFilteringEnabled(false)
	l.InfiniteScrolling = false
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)
	l.KeyMap.Filter.SetEnabled(false)
	l.KeyMap.ClearFilter.SetEnabled(false)
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)

	return Model{keys: keys, list: l, query: ti, committed: map[string]bool{}, init: true}
}

// SetKeys installs the compiled bindings.
func (m *Model) SetKeys(keys keybind.Map) {
	m.keys = keys
}

// Open snapshots the selection and shows the chooser. Empty option lists are
// inert; re-opening resets the query and the current selection.
func (m *Model) Open(options []Option, selectedIDs []string, multi bool) {
	m.options = append([]Option(nil), options...)
	m.multi = multi
	m.committed = map[string]bool{}
	for _, id := range selectedIDs {
		m.committed[id] = true
	}

	items := make([]list.Item, len(m.options))
	for i, o := range m.options {
		items[i] = item{opt: o, selected: m.multi && m.committed[o.ID]}
	}
	m.list.SetItems(items)
	m.list.SetFilterText("")
	m.query.SetValue("")

	if !multi {
		for i, o := range m.options {
			if m.committed[o.ID] {
				m.list.Select(i)
				break
			}
		}
	}
	if m.width > 0 {
		m.list.SetSize(m.width, max(3, m.height-2))
	}
	m.purpose = purposeOpen
}

// Visible reports whether the chooser is open.
func (m Model) Visible() bool { return m.init && m.purpose == purposeOpen }

func (m *Model) close() {
	m.purpose = purposeClosed
}

// Close hides the chooser without emitting anything; the containing editor
// calls it when it consumes a result message.
func (m *Model) Close() {
	m.close()
}

// SetSize clamps the chooser to the available area.
func (m *Model) SetSize(width, height int) {
	if !m.init {
		return
	}
	m.width, m.height = width, height
	m.query.SetWidth(width - 4)
	m.list.SetSize(width, max(3, height-2))
}

// SelectID returns the currently highlighted option, if any.
func (m Model) SelectedOption() (Option, bool) {
	if len(m.list.VisibleItems()) == 0 {
		return Option{}, false
	}
	if it, ok := m.list.SelectedItem().(item); ok {
		return it.opt, true
	}
	return Option{}, false
}

// Update handles chooser keys. Navigation and confirm/cancel are consumed
// here; typing and paste edit the query and re-rank matches synchronously.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Bind("selector", "down")):
			m.list.CursorDown()
			return m, nil
		case key.Matches(msg, m.keys.Bind("selector", "up")):
			m.list.CursorUp()
			return m, nil
		case key.Matches(msg, m.keys.Bind("selector", "toggle")) && m.multi:
			if it, ok := m.list.SelectedItem().(item); ok {
				if m.committed[it.opt.ID] {
					delete(m.committed, it.opt.ID)
				} else {
					m.committed[it.opt.ID] = true
				}
				m.refreshSelection()
			}
			return m, nil
		case key.Matches(msg, m.keys.Bind("selector", "confirm")):
			if len(m.list.VisibleItems()) == 0 {
				return m, nil // zero results: confirm does nothing
			}
			ids := m.selectedIDs()
			m.close()
			return m, func() tea.Msg { return SelectedMsg{IDs: ids} }
		case key.Matches(msg, m.keys.Bind("selector", "cancel")):
			m.close()
			return m, func() tea.Msg { return CancelledMsg{} }
		}

		// Everything else edits the query (typing, paste, editing keys).
		var cmd tea.Cmd
		m.query, cmd = m.query.Update(msg)
		m.applyQuery()
		return m, cmd
	case tea.PasteMsg:
		// Pasted text is query text, exactly once.
		var cmd tea.Cmd
		m.query, cmd = m.query.Update(msg)
		m.applyQuery()
		return m, cmd
	}
	return m, nil
}

func (m *Model) applyQuery() {
	q := m.query.Value()
	m.list.SetFilterText(q)
	if q == "" && !m.multi {
		for i, o := range m.options {
			if m.committed[o.ID] {
				m.list.Select(i)
				break
			}
		}
	}
}

func (m *Model) selectedIDs() []string {
	if !m.multi {
		if it, ok := m.list.SelectedItem().(item); ok {
			return []string{it.opt.ID}
		}
		return nil
	}
	ids := make([]string, 0, len(m.committed))
	for _, o := range m.options {
		if m.committed[o.ID] {
			ids = append(ids, o.ID)
		}
	}
	return ids
}

func (m *Model) refreshSelection() {
	items := make([]list.Item, len(m.options))
	for i, o := range m.options {
		items[i] = item{opt: o, selected: m.multi && m.committed[o.ID]}
	}
	m.list.SetItems(items)
	m.applyQuery()
}

// View renders the query line, the ranked matches and a status footer.
func (m Model) View() string {
	if !m.Visible() {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.query.View())
	b.WriteString("\n")
	body := m.list.View()
	b.WriteString(body)
	if len(m.list.VisibleItems()) == 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(styles.DimGray).Render("No matches"))
	}
	hint := m.hintLine()
	if hint != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(styles.DimGray).Render(hint))
	}
	return b.String()
}

func (m Model) hintLine() string {
	confirm := keybind.ShortKeys(m.keys.Aliases("selector", "confirm"))
	cancel := keybind.ShortKeys(m.keys.Aliases("selector", "cancel"))
	if m.multi {
		toggle := keybind.ShortKeys(m.keys.Aliases("selector", "toggle"))
		return toggle + ": toggle  " + confirm + ": done  " + cancel + ": cancel"
	}
	return confirm + ": select  " + cancel + ": cancel"
}
