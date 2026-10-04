// Package datepicker implements the due-date entry field: a text input with
// live Org-style autocomplete plus a synchronized Monday-first keyboard
// calendar. All resolutions go through internal/dateentry.
package datepicker

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/oronbz/nag/internal/dateentry"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/styles"
)

var weekdayLabels = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}

const headerFormat = "January 2006"

// Model owns the due-date text input, its autocomplete suggestions and the
// synchronized keyboard calendar.
type Model struct {
	keys  keybind.Map
	now   time.Time
	base  time.Time
	input textinput.Model

	highlight time.Time // last valid parsed date or base
	errText   string
	focused   bool
	width     int
	height    int
}

// New builds a picker with the captured clock and editing anchor. The base
// is normalized into now.Location() at local midnight so navigation, preview
// and suggestions all render in the captured civil date.
func New(now, base time.Time, keys keybind.Map) Model {
	loc := now.Location()
	base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc)
	ti := textinput.New()
	ti.Placeholder = "today, tue, +2d, sep 15, 2026-10-04"
	ti.CharLimit = 64
	ti.Prompt = "Due date: "
	ti.ShowSuggestions = true
	applyInputStyles(&ti)

	m := Model{keys: keys, now: now, base: base, input: ti, highlight: base}
	applyInputKeyMap(&m)
	return m
}

// applyInputStyles keeps the teal prompt rendering used by the other fields.
func applyInputStyles(ti *textinput.Model) {
	s := ti.Styles()
	prompt := lipgloss.NewStyle().Foreground(styles.Teal)
	s.Focused.Prompt = prompt
	s.Blurred.Prompt = prompt
	s.Focused.Text = lipgloss.NewStyle()
	s.Blurred.Text = lipgloss.NewStyle()
	ti.SetStyles(s)
}

// applyInputKeyMap rebinds the editor from keys.textinput, dropping the
// aliases reserved by the calendar scope so they move the highlight instead.
// Suggestion cycling is projected from the calendar scope onto the input's
// suggestion bindings; Tab completion is disabled (completion is the
// picker-owned calendar.complete binding handled in Update).
func applyInputKeyMap(m *Model) {
	km := textinput.DefaultKeyMap()
	reserved := map[string]bool{}
	for _, a := range []string{"left", "down", "up", "right", "complete", "next_suggestion", "previous_suggestion"} {
		for _, alias := range m.keys.Aliases("calendar", a) {
			reserved[alias] = true
		}
	}
	bind := func(b *key.Binding, action string) {
		var keep []string
		for _, alias := range m.keys.Aliases("textinput", action) {
			if !reserved[alias] {
				keep = append(keep, alias)
			}
		}
		if len(keep) == 0 {
			keep = []string{"\x00disabled"}
		}
		*b = key.NewBinding(key.WithKeys(keep...))
	}
	project := func(b *key.Binding, scope, action string) {
		*b = key.NewBinding(key.WithKeys(m.keys.Aliases(scope, action)...))
	}
	bind(&km.CharacterForward, "character_forward")
	bind(&km.CharacterBackward, "character_backward")
	bind(&km.WordForward, "word_forward")
	bind(&km.WordBackward, "word_backward")
	bind(&km.DeleteWordBackward, "delete_word_backward")
	bind(&km.DeleteWordForward, "delete_word_forward")
	bind(&km.DeleteAfterCursor, "delete_after_cursor")
	bind(&km.DeleteBeforeCursor, "delete_before_cursor")
	bind(&km.DeleteCharacterBackward, "delete_character_backward")
	bind(&km.DeleteCharacterForward, "delete_character_forward")
	bind(&km.LineStart, "line_start")
	bind(&km.LineEnd, "line_end")
	bind(&km.Paste, "paste")
	km.AcceptSuggestion = key.NewBinding(key.WithKeys("\x00disabled"))
	project(&km.NextSuggestion, "calendar", "next_suggestion")
	project(&km.PrevSuggestion, "calendar", "previous_suggestion")
	m.input.KeyMap = km
}

func (m *Model) reparse() {
	raw := m.input.Value()
	resolved, err := dateentry.Parse(raw, m.now, m.base)
	switch {
	case raw == "":
		m.highlight = m.base
		m.errText = ""
	case err != nil:
		m.errText = err.Error()
	default:
		m.highlight = *resolved
		m.errText = ""
	}
	m.updateSuggestions()
}

// updateSuggestions recomputes prefix completions for the current stem.
func (m *Model) updateSuggestions() {
	stem := strings.ToLower(strings.TrimSpace(m.input.Value()))
	var cands []string
	add := func(c string) {
		if c != stem && strings.HasPrefix(c, stem) && !contains(cands, c) {
			if _, err := dateentry.Parse(c, m.now, m.base); err == nil {
				cands = append(cands, c)
			}
		}
	}

	if stem != "" {
		// Named terms.
		for _, term := range []string{"today", "tomorrow", "yesterday"} {
			add(term)
		}
		// Weekday names.
		for _, wd := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
			add(wd[:3])
			add(wd)
			add("next " + wd[:3])
			add("next " + wd)
		}
		// eow/eom.
		add("eow")
		add("eom")
		// Month names: month-only completes to "<month> <base day clamped>".
		baseDay := m.base.Day()
		for _, mo := range []string{
			"january", "february", "march", "april", "may", "june",
			"july", "august", "september", "october", "november", "december",
		} {
			abbr := mo[:3]
			last := time.Date(m.base.Year(), timeMonth(mo), 1, 0, 0, 0, 0, m.base.Location()).
				AddDate(0, 1, -1).Day()
			day := baseDay
			if day > last {
				day = last
			}
			add(abbr + " " + itoa(day))
			add(mo)
		}
		// Numeric offset stems offer unit suffixes.
		if len(stem) >= 1 && (stem[0] == '+' || stem[0] == '-') {
			body := stem[1:]
			if body != "" && allDigits(body) {
				for _, unit := range []string{"d", "w", "m", "y"} {
					add(stem + unit)
				}
			}
		}
		// Day-first month completions retain the typed day.
		if i := strings.IndexFunc(stem, func(r rune) bool { return unicode.IsLetter(r) }); i > 0 {
			head := stem[:i]
			if allDigits(strings.TrimSpace(head)) {
				for _, mo := range []string{
					"jan", "feb", "mar", "apr", "may", "jun",
					"jul", "aug", "sep", "oct", "nov", "dec",
				} {
					if strings.HasPrefix(mo, stem[i:]) {
						add(head + mo)
					}
				}
			}
		}
	}

	m.input.SetSuggestions(cands)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func timeMonth(name string) time.Month {
	switch name[:3] {
	case "jan":
		return time.January
	case "feb":
		return time.February
	case "mar":
		return time.March
	case "apr":
		return time.April
	case "may":
		return time.May
	case "jun":
		return time.June
	case "jul":
		return time.July
	case "aug":
		return time.August
	case "sep":
		return time.September
	case "oct":
		return time.October
	case "nov":
		return time.November
	default:
		return time.December
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// SetValue replaces the text and re-resolves the highlight.
func (m *Model) SetValue(s string) {
	m.input.SetValue(s)
	m.reparse()
}

// Value returns the raw entry text.
func (m Model) Value() string { return m.input.Value() }

// Focus activates the picker.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
	m.reparse()
}

// Blur deactivates the picker.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
}

// Focused reports whether the picker is active.
func (m Model) Focused() bool { return m.focused }

// SetSize clamps the picker to the pane size.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(max(1, min(width-12, 40)))
}

// Resolve parses the current input into a due timestamp.
func (m Model) Resolve() (*time.Time, error) {
	return dateentry.Parse(m.input.Value(), m.now, m.base)
}

// Error returns the current inline parse error, if any.
func (m Model) Error() string { return m.errText }

// complete accepts the highlighted differing suggestion via the picker-owned
// calendar.complete binding.
func (m *Model) complete() {
	matched := m.input.MatchedSuggestions()
	if len(matched) == 0 {
		return
	}
	idx := m.input.CurrentSuggestionIndex()
	if idx >= len(matched) {
		return
	}
	suggestion := matched[idx]
	if suggestion == m.input.Value() {
		return // fully completed text
	}
	m.SetValue(suggestion)
}

// moveHighlight shifts the calendar highlight and rewrites the date text.
func (m *Model) moveHighlight(days int) {
	h := m.highlight.AddDate(0, 0, days)
	m.highlight = h
	m.input.SetValue(h.Format("2006-01-02"))
	m.reparse()
}

// Update handles picker keys. Calendar navigation precedes text editing only
// while the due field is focused (the outer form routes keys here).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Bind("calendar", "left")):
			m.moveHighlight(-1)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("calendar", "right")):
			m.moveHighlight(1)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("calendar", "up")):
			m.moveHighlight(-7)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("calendar", "down")):
			m.moveHighlight(7)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("calendar", "complete")):
			m.complete()
			return m, nil
		}
	}

	var cmd tea.Cmd
	before := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	// Reparse/rebuild suggestions only when the value changed: rebuilding on
	// a pure suggestion-navigation key would reset the highlighted candidate.
	if m.input.Value() != before {
		m.reparse()
	}
	return m, cmd
}

// View renders the input, inline error, resolved preview and calendar.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if m.errText != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Red).Render(m.errText))
		b.WriteString("\n")
	} else if v := strings.TrimSpace(m.input.Value()); v != "" {
		if resolved, err := dateentry.Parse(v, m.now, m.base); err == nil {
			preview := resolved.In(m.now.Location()).Format("Mon Jan 2, 2006")
			b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).Render("→ " + preview))
			b.WriteString("\n")
		}
	}

	if cands := m.input.MatchedSuggestions(); len(cands) > 0 {
		idx := m.input.CurrentSuggestionIndex()
		if idx < len(cands) {
			resolved, err := dateentry.Parse(cands[idx], m.now, m.base)
			line := cands[idx]
			if err == nil {
				line += " → " + resolved.In(m.now.Location()).Format("Jan 2, 2006")
			}
			b.WriteString(lipgloss.NewStyle().Foreground(styles.Teal).Render("⇥ " + line))
			b.WriteString("\n")
		}
	}

	b.WriteString(m.calendarView())
	return b.String()
}

// calendarView renders a Monday-first six-week grid around the highlight.
func (m Model) calendarView() string {
	shown := m.highlight
	first := time.Date(shown.Year(), shown.Month(), 1, 0, 0, 0, 0, m.base.Location())
	offset := (int(first.Weekday()) + 6) % 7 // Monday=0

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(styles.Teal).Bold(true).
		Render(first.Format(headerFormat)))
	b.WriteString("\n")

	gridStyle := lipgloss.NewStyle().Foreground(styles.White)
	dimStyle := lipgloss.NewStyle().Foreground(styles.DimGray)
	hlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(styles.Teal)

	var row strings.Builder
	for i, label := range weekdayLabels {
		row.WriteString(label)
		if i < len(weekdayLabels)-1 {
			row.WriteString(" ")
		}
	}
	b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).Render(row.String()))
	b.WriteString("\n")

	day := first.AddDate(0, 0, -offset)
	for range 6 {
		row.Reset()
		for col := range 7 {
			label := itoa(day.Day())
			if len(label) == 1 {
				label = " " + label
			}
			style := gridStyle
			if day.Month() != shown.Month() {
				style = dimStyle
			}
			if day.Year() == m.highlight.Year() && day.Month() == m.highlight.Month() &&
				day.Day() == m.highlight.Day() {
				style = hlStyle
			}
			row.WriteString(style.Render(label))
			if col < 6 {
				row.WriteString(" ")
			}
			day = day.AddDate(0, 0, 1)
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// FocusLine returns the zero-based line of the calendar highlight within the
// unframed View output, for viewport scrolling.
func (m Model) FocusLine() int {
	lines := 1 // input line
	if m.errText != "" || strings.TrimSpace(m.input.Value()) != "" {
		lines++
	}
	if len(m.input.MatchedSuggestions()) > 0 {
		lines++
	}
	// Calendar header and weekday row precede the grid.
	shown := m.highlight
	first := time.Date(shown.Year(), shown.Month(), 1, 0, 0, 0, 0, m.base.Location())
	offset := (int(first.Weekday()) + 6) % 7
	row := 0
	day := first.AddDate(0, 0, -offset)
	for i := 0; ; i++ {
		if day.Year() == shown.Year() && day.Month() == shown.Month() && day.Day() == shown.Day() {
			row = i / 7
			break
		}
		day = day.AddDate(0, 0, 1)
	}
	return lines + 2 + row
}
