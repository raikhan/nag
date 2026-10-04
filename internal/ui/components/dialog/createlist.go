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
	Color string
}

type EditListSubmitMsg struct {
	ID    string
	Title string
	Color string
}

// listColorPalette is the fixed swatch row: index 0 is "no colour" and
// renders as a dim ○; indices 1..12 are ● in their own hue.
var listColorPalette = []string{
	"", "#F15840", "#F6A52F", "#FBD93B", "#5ECE65", "#84C0FC", "#2E7FFA",
	"#5C59E1", "#F15F7A", "#CC82F1", "#C5A879", "#747D86", "#E5B8AF",
}

// listFocus selects which part of the dialog receives keys.
type listFocus int

const (
	focusName listFocus = iota
	focusColor
)

type CreateListModel struct {
	keys       keybind.Map
	titleInput textinput.Model
	editingID  string
	colorIdx   int
	focus      listFocus
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

	return CreateListModel{titleInput: ti, focus: focusName}
}

// SetKeys installs the compiled key bindings used by this dialog.
func (m *CreateListModel) SetKeys(keys keybind.Map) {
	m.keys = keys
	// The dialog scope owns tab/shift+tab/j/k and enter/esc; reserving it
	// keeps those keys from ever reaching the text input.
	projectTextInputKeys(&m.titleInput, keys, "dialog")
}

func (m *CreateListModel) Show() {
	m.visible = true
	m.editingID = ""
	m.titleInput.SetValue("")
	// A new list keeps EventKit's default colour until the user picks one.
	m.colorIdx = 0
	m.setFocus(focusName)
}

func (m *CreateListModel) ShowEdit(id, title, color string) {
	m.visible = true
	m.editingID = id
	m.titleInput.SetValue(title)
	m.colorIdx = colorIndex(color)
	m.setFocus(focusName)
}

// colorIndex maps a stored hex to its palette slot, falling back to the
// no-colour entry for anything outside the palette.
func colorIndex(color string) int {
	for i, c := range listColorPalette {
		if c != "" && c == color {
			return i
		}
	}
	return 0
}

// currentColor resolves the selected swatch to a hex string; index 0 means
// "no colour" and stays empty so the stored hue is never overwritten.
func (m CreateListModel) currentColor() string {
	return listColorPalette[m.colorIdx]
}

func (m *CreateListModel) setFocus(f listFocus) {
	m.focus = f
	if f == focusName {
		m.titleInput.Focus()
		return
	}
	m.titleInput.Blur()
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

func (m CreateListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m CreateListModel) Update(msg tea.Msg) (CreateListModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Tab/Shift-Tab move between the name input and the colour row
		// before anything else, so j/k typing still works in the input.
		if key.Matches(msg, m.keys.Bind("dialog", "next_field")) ||
			key.Matches(msg, m.keys.Bind("dialog", "previous_field")) {
			if m.focus == focusName {
				m.setFocus(focusColor)
			} else {
				m.setFocus(focusName)
			}
			return m, nil
		}
		// On the colour row the calendar arrows walk the swatches,
		// matching how the datepicker consumes its own scope. Clamped,
		// never wrapped.
		if m.focus == focusColor {
			switch {
			case key.Matches(msg, m.keys.Bind("calendar", "left")):
				if m.colorIdx > 0 {
					m.colorIdx--
				}
				return m, nil
			case key.Matches(msg, m.keys.Bind("calendar", "right")):
				if m.colorIdx < len(listColorPalette)-1 {
					m.colorIdx++
				}
				return m, nil
			}
		}
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
			color := m.currentColor()
			m.Hide()
			if editingID != "" {
				return m, func() tea.Msg {
					return EditListSubmitMsg{ID: editingID, Title: title, Color: color}
				}
			}
			return m, func() tea.Msg {
				return CreateListSubmitMsg{Title: title, Color: color}
			}
		}
	}

	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

// colorRow renders one cell per palette entry, separated by a space. The
// focused-and-selected cell is a bold ringed glyph; everything else uses the
// unfocused glyph. 13 cells fit well inside the 74-wide dialog.
func (m CreateListModel) colorRow() string {
	cells := make([]string, 0, len(listColorPalette))
	for i, hex := range listColorPalette {
		selected := i == m.colorIdx
		glyph := "●"
		if i == 0 {
			glyph = "○"
		}
		style := lipgloss.NewStyle()
		switch {
		case i == 0:
			style = style.Foreground(styles.DimGray)
		default:
			style = style.Foreground(lipgloss.Color(hex))
		}
		if selected {
			glyph = "◉"
			if i == 0 {
				glyph = "◌"
			}
			style = style.Bold(true)
		}
		cells = append(cells, style.Render(glyph))
	}
	return strings.Join(cells, " ")
}

func (m CreateListModel) View() string {
	if !m.visible {
		return ""
	}

	dialogTitle := "New List"
	footer := keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": create  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": focus  " +
		keybind.ShortKeys(m.keys.Aliases("calendar", "left")) + "/" +
		keybind.ShortKeys(m.keys.Aliases("calendar", "right")) + ": colour"
	if m.isEditing() {
		dialogTitle = "Edit List"
		footer = keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": save  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": focus  " +
			keybind.ShortKeys(m.keys.Aliases("calendar", "left")) + "/" +
			keybind.ShortKeys(m.keys.Aliases("calendar", "right")) + ": colour"
	}

	title := styles.DialogTitleStyle.Render(dialogTitle)
	content := title + "\n\n" +
		m.titleInput.View() + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render("Colour: ") + m.colorRow() + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render(footer)

	dlg := styles.DialogStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dlg,
	)
}
