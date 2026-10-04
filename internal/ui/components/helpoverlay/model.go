package helpoverlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Model struct {
	viewport viewport.Model
	visible  bool
	width    int
	height   int
	keys     keybind.Map
}

func New() Model {
	return Model{}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	vw := width * 60 / 100
	vh := height * 70 / 100
	if vw < 50 {
		vw = 50
	}
	if vh < 20 {
		vh = 20
	}
	m.viewport = viewport.New(viewport.WithWidth(vw-4), viewport.WithHeight(vh-4))
	m.viewport.KeyMap = m.projectViewportKeys()
	m.viewport.SetContent(m.helpContent())
}

// SetKeys stores the compiled bindings, projects keys.help into the viewport
// keymap, and regenerates the overlay content from the registry.
func (m *Model) SetKeys(keys keybind.Map) {
	m.keys = keys
	m.viewport.KeyMap = m.projectViewportKeys()
	m.viewport.SetContent(m.helpContent())
}

// projectViewportKeys maps the help scope's scroll bindings onto
// viewport.DefaultKeyMap fields.
func (m Model) projectViewportKeys() viewport.KeyMap {
	vk := viewport.DefaultKeyMap()
	vk.PageDown = newHelpBinding(m.keys, "page_down")
	vk.PageUp = newHelpBinding(m.keys, "page_up")
	vk.HalfPageUp = newHelpBinding(m.keys, "half_page_up")
	vk.HalfPageDown = newHelpBinding(m.keys, "half_page_down")
	vk.Up = newHelpBinding(m.keys, "up")
	vk.Down = newHelpBinding(m.keys, "down")
	vk.Left = newHelpBinding(m.keys, "left")
	vk.Right = newHelpBinding(m.keys, "right")
	return vk
}

func newHelpBinding(keys keybind.Map, action string) key.Binding {
	return key.NewBinding(key.WithKeys(keys.Aliases("help", action)...))
}

func (m *Model) Toggle() {
	m.visible = !m.visible
	if m.visible {
		m.viewport.GotoTop()
	}
}

func (m Model) Visible() bool {
	return m.visible
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.visible {
		return ""
	}

	content := styles.HelpOverlayStyle.
		Width(m.viewport.Width() + 4).
		Render(m.viewport.View())

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// helpContent renders one group per registry scope, one line per action,
// using the configured bindings. Disabled actions are omitted.
func (m Model) helpContent() string {
	k := styles.HelpKeyStyle
	d := styles.HelpDescStyle
	h := styles.HelpHeaderStyle

	var sb strings.Builder
	first := true
	for _, scope := range keybind.Scopes() {
		var lines []string
		for _, a := range keybind.Registry() {
			if a.Scope != scope {
				continue
			}
			aliases := m.keys.Aliases(scope, a.Name)
			if len(aliases) == 0 {
				continue
			}
			lines = append(lines, k.Render(keybind.ShortKeys(aliases))+d.Render(a.Help)+"\n")
		}
		if len(lines) == 0 {
			continue
		}
		if !first {
			sb.WriteString("\n")
		}
		first = false
		sb.WriteString(h.Render(keybind.ScopeTitle(scope)))
		sb.WriteString("\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("  %s", line))
		}
	}
	return sb.String()
}
