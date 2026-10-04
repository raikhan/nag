package dialog

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/styles"
)

type CreateListSubmitMsg struct {
	Title string
}

type EditListSubmitMsg struct {
	ID    string
	Title string
}

type CreateListModel struct {
	keys       keybind.Map
	titleInput textinput.Model
	editingID  string
	visible    bool
	width      int
	height     int
}

func NewCreateList() CreateListModel {
	ti := textinput.New()
	ti.Placeholder = "Shopping"
	ti.CharLimit = 256
	ti.SetWidth(44)
	ti.Prompt = "Name: "
	applyInputStyles(&ti)

	return CreateListModel{titleInput: ti}
}

// SetKeys installs the compiled key bindings used by this dialog.
func (m *CreateListModel) SetKeys(keys keybind.Map) {
	m.keys = keys
	projectTextInputKeys(&m.titleInput, keys)
}

func (m *CreateListModel) Show() {
	m.visible = true
	m.editingID = ""
	m.titleInput.SetValue("")
	m.titleInput.Focus()
}

func (m *CreateListModel) ShowEdit(id, title string) {
	m.visible = true
	m.editingID = id
	m.titleInput.SetValue(title)
	m.titleInput.Focus()
}

func (m CreateListModel) isEditing() bool {
	return m.editingID != ""
}

func (m *CreateListModel) Hide() {
	m.visible = false
	m.titleInput.Blur()
}

func (m CreateListModel) Visible() bool {
	return m.visible
}

func (m *CreateListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m CreateListModel) Update(msg tea.Msg) (CreateListModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Bind("dialog", "cancel")) {
			m.Hide()
			return m, nil
		}
		if key.Matches(msg, m.keys.Bind("dialog", "submit")) {
			title := strings.TrimSpace(m.titleInput.Value())
			if title == "" {
				return m, nil
			}
			editingID := m.editingID
			m.Hide()
			if editingID != "" {
				return m, func() tea.Msg {
					return EditListSubmitMsg{ID: editingID, Title: title}
				}
			}
			return m, func() tea.Msg {
				return CreateListSubmitMsg{Title: title}
			}
		}
	}

	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

func (m CreateListModel) View() string {
	if !m.visible {
		return ""
	}

	dialogTitle := "New List"
	footer := keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": create  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel"
	if m.isEditing() {
		dialogTitle = "Edit List"
		footer = keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": save  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel"
	}

	title := styles.DialogTitleStyle.Render(dialogTitle)
	content := title + "\n\n" +
		m.titleInput.View() + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render(footer)

	dlg := styles.DialogStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dlg,
	)
}
