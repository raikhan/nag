package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/styles"
)

type ConfirmAction int

const (
	ConfirmDelete ConfirmAction = iota
	ConfirmDeleteList
)

type ConfirmYesMsg struct{ Action ConfirmAction }
type ConfirmNoMsg struct{}

type ConfirmModel struct {
	keys    keybind.Map
	message string
	action  ConfirmAction
	visible bool
	width   int
	height  int
}

func NewConfirm() ConfirmModel {
	return ConfirmModel{}
}

// SetKeys installs the compiled key bindings used by this dialog.
func (m *ConfirmModel) SetKeys(keys keybind.Map) {
	m.keys = keys
}

func (m *ConfirmModel) Show(message string, action ConfirmAction) {
	m.message = message
	m.action = action
	m.visible = true
}

func (m *ConfirmModel) Hide() {
	m.visible = false
}

func (m ConfirmModel) Visible() bool {
	return m.visible
}

func (m *ConfirmModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Bind("confirm", "yes")) {
			action := m.action
			m.Hide()
			return m, func() tea.Msg { return ConfirmYesMsg{Action: action} }
		}
		if key.Matches(msg, m.keys.Bind("confirm", "no")) {
			m.Hide()
			return m, func() tea.Msg { return ConfirmNoMsg{} }
		}
	}

	return m, nil
}

func (m ConfirmModel) View() string {
	if !m.visible {
		return ""
	}

	title := styles.DialogTitleStyle.Render("Confirm")
	yes := keybind.ShortKeys(m.keys.Aliases("confirm", "yes"))
	no := keybind.ShortKeys(m.keys.Aliases("confirm", "no"))
	content := title + "\n\n" +
		m.message + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render(yes+": yes  "+no+": no")

	dialog := styles.DialogStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}
