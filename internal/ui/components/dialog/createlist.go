package dialog

import (
	"strings"
	"unicode"

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
	// No scope is reserved: Update matches the dialog's own bindings —
	// submit, cancel, the focus switches and the colour arrows — before
	// anything reaches the input, so left/right/ctrl+arrows still move the
	// caret while typing a name.
	projectTextInputKeys(&m.titleInput, keys)
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
		if m.isFocusSwitch(msg, "next_field") || m.isFocusSwitch(msg, "previous_field") {
			if m.focus == focusName {
				m.setFocus(focusColor)
			} else {
				m.setFocus(focusName)
			}
			return m, nil
		}
		// On the colour row the arrows walk the swatches. Clamped, never
		// wrapped, and never forwarded to the name input.
		if m.focus == focusColor {
			switch {
			case key.Matches(msg, m.keys.Bind("dialog", "color_prev")):
				if m.colorIdx > 0 {
					m.colorIdx--
				}
				return m, nil
			case key.Matches(msg, m.keys.Bind("dialog", "color_next")):
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

// isFocusSwitch reports whether the key moves focus between the name field
// and the colour row. A bare printable alias (the default "j"/"k") only
// counts while the colour row is focused, so the name field keeps it as
// text; any modified alias always moves focus.
func (m CreateListModel) isFocusSwitch(msg tea.KeyPressMsg, action string) bool {
	if !key.Matches(msg, m.keys.Bind("dialog", action)) {
		return false
	}
	for _, a := range m.keys.Aliases("dialog", action) {
		if a == msg.String() && isPrintableAlias(a) {
			return m.focus == focusColor
		}
	}
	return true
}

// isPrintableAlias reports whether an alias is one printable rune, i.e.
// something a text input would receive as text.
func isPrintableAlias(alias string) bool {
	r := []rune(alias)
	return len(r) == 1 && unicode.IsPrint(r[0])
}

// colorCell renders one swatch. Index 0 is the dim "no colour" circle, the
// selected entry is a bold ring, and the entry the cursor is on gets teal
// brackets. Every cell is exactly three columns wide so the row never
// shifts horizontally as the cursor moves.
func (m CreateListModel) colorCell(i int) string {
	glyph, hue := "●", lipgloss.Color(listColorPalette[i])
	if i == 0 {
		glyph, hue = "○", styles.DimGray
	}
	style := lipgloss.NewStyle().Foreground(hue)
	if i == m.colorIdx {
		glyph, style = "◉", style.Bold(true)
		if i == 0 {
			glyph = "◌"
		}
	}
	left, right := " ", " "
	if m.focus == focusColor && i == m.colorIdx {
		left, right = "[", "]"
	}
	bracket := lipgloss.NewStyle().Foreground(styles.Teal)
	return bracket.Render(left) + style.Render(glyph) + bracket.Render(right)
}

// colorRow renders one cell per palette entry, separated by a space.
func (m CreateListModel) colorRow() string {
	cells := make([]string, 0, len(listColorPalette))
	for i := range listColorPalette {
		cells = append(cells, m.colorCell(i))
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
		firstShort(m.keys, "dialog", "color_prev") + "/" + firstShort(m.keys, "dialog", "color_next") + ": colour"
	if m.isEditing() {
		dialogTitle = "Edit List"
		footer = keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": save  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": focus  " +
			firstShort(m.keys, "dialog", "color_prev") + "/" + firstShort(m.keys, "dialog", "color_next") + ": colour"
	}

	title := styles.DialogTitleStyle.Render(dialogTitle)
	content := title + "\n\n" +
		m.titleInput.View() + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render("Colour:  ") + m.colorRow() + "\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render("Selected: ") + m.colorCell(m.colorIdx) + "\n\n" +
		lipgloss.NewStyle().Foreground(styles.DimGray).Render(footer)

	dlg := styles.DialogStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dlg,
	)
}
