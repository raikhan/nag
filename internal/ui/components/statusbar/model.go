package statusbar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Panel int

const (
	PanelLists Panel = iota
	PanelReminders
)

type Model struct {
	panel     Panel
	sortLabel string
	errMsg    string
	infoMsg   string
	loading   string
	keys      keybind.Map
}

func New() Model {
	return Model{}
}

// SetKeys stores the compiled bindings; panel hints are derived from them at
// render time so configured keys are always shown.
func (m *Model) SetKeys(keys keybind.Map) {
	m.keys = keys
}

func (m *Model) SetPanel(panel Panel) {
	m.panel = panel
}

func (m *Model) SetError(msg string) {
	m.errMsg = msg
}

func (m *Model) ClearError() {
	m.errMsg = ""
}

func (m *Model) SetLoading(msg string) {
	m.loading = msg
}

func (m *Model) ClearLoading() {
	m.loading = ""
}

func (m *Model) SetSortLabel(label string) {
	m.sortLabel = label
}

func (m *Model) SetInfo(msg string) {
	m.infoMsg = msg
}

func (m *Model) ClearInfo() {
	m.infoMsg = ""
}

type hint struct {
	key  string
	desc string
}

var (
	brandStyle = lipgloss.NewStyle().
			Background(styles.Teal).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)
	keyStyle  = lipgloss.NewStyle().Foreground(styles.Teal).Bold(true)
	dimStyle  = lipgloss.NewStyle().Foreground(styles.DimGray)
	errStyle  = lipgloss.NewStyle().Foreground(styles.Red)
	infoStyle = lipgloss.NewStyle().Foreground(styles.Green)
)

// hint renders one hint entry from the configured aliases of
// scope.action; disabled actions are skipped.
func (m Model) hint(scope, action, desc string) (hint, bool) {
	aliases := m.keys.Aliases(scope, action)
	if len(aliases) == 0 {
		return hint{}, false
	}
	return hint{keybind.ShortKeys(aliases), desc}, true
}

// hintsFor builds the hint list for the focused panel from the registry and
// the configured bindings.
func (m Model) hintsFor() []hint {
	var hints []hint
	add := func(scope, action, desc string) {
		if h, ok := m.hint(scope, action, desc); ok {
			hints = append(hints, h)
		}
	}
	add("list", "up", "up")
	add("list", "down", "down")
	add("global", "next_panel", "panel")
	if m.panel == PanelReminders {
		add("global", "toggle_complete", "toggle")
		add("global", "new", "new")
		add("global", "edit", "edit")
		add("global", "delete", "delete")
		sortDesc := "sort"
		if m.sortLabel != "" {
			sortDesc = "sort: " + m.sortLabel
		}
		add("global", "sort", sortDesc)
		add("global", "show_completed", "completed")
	} else {
		add("global", "new", "new")
		add("global", "edit", "edit")
		add("global", "delete", "delete")
	}
	add("list", "filter", "filter")
	add("global", "help", "help")
	add("global", "quit", "quit")
	return hints
}

func (m Model) View() string {
	brand := brandStyle.Render("NAG")

	hints := m.hintsFor()

	var parts []string
	for _, h := range hints {
		parts = append(parts, keyStyle.Render(h.key)+" "+h.desc)
	}
	hintsStr := strings.Join(parts, "  ")

	var rightText string
	if m.errMsg != "" {
		rightText = "  " + errStyle.Render(m.errMsg)
	} else if m.infoMsg != "" {
		rightText = "  " + infoStyle.Render(m.infoMsg)
	} else if m.loading != "" {
		rightText = "  " + dimStyle.Render(m.loading)
	}

	return brand + " " + dimStyle.Render("│") + " " + hintsStr + rightText
}
