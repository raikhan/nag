package dialog

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/BRO3886/go-eventkit"

	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/styles"
)

type CreateSubmitMsg struct {
	Input reminders.CreateReminderInput
}

type EditSubmitMsg struct {
	ID    string
	Input reminders.UpdateReminderInput
}

const fieldCount = 5 // title, notes, due date, priority, recurrence

// applyInputStyles restores V1 terminal-default input rendering: a teal
// prompt and unstyled text, applied to both focus states.
func applyInputStyles(ti *textinput.Model) {
	s := ti.Styles()
	prompt := lipgloss.NewStyle().Foreground(styles.Teal)
	s.Focused.Prompt = prompt
	s.Blurred.Prompt = prompt
	text := lipgloss.NewStyle()
	s.Focused.Text = text
	s.Blurred.Text = text
	ti.SetStyles(s)
}

type chooserPurpose int

const (
	chooserNone chooserPurpose = iota
	chooserPriority
	chooserRecurrence
)

type CreateModel struct {
	keys        keybind.Map
	titleInput  textinput.Model
	notesInput  textinput.Model
	picker      datepickerModel
	priority    int
	recurrence  []eventkit.RecurrenceRule
	selector    selectorModel
	purpose     chooserPurpose
	recurrenceE RecurrenceModel

	// dirty tracking
	dueText           string // picker text at open; empty means no due date
	dueTouched        bool
	origRules         []eventkit.RecurrenceRule
	recurrenceTouched bool

	now  time.Time
	base time.Time

	focusIndex int
	visible    bool
	listName   string
	editingID  string
	width      int
	height     int
	errText    string
}

func NewCreate() CreateModel {
	ti := textinput.New()
	ti.Placeholder = "Buy groceries"
	ti.CharLimit = 256
	ti.SetWidth(58)
	ti.Prompt = "Title:    "
	applyInputStyles(&ti)

	ni := textinput.New()
	ni.Placeholder = "Optional notes"
	ni.CharLimit = 1024
	ni.SetWidth(58)
	ni.Prompt = "Notes:    "
	applyInputStyles(&ni)

	return CreateModel{
		titleInput: ti,
		notesInput: ni,
		priority:   reminders.PriorityNone,
	}
}

// SetKeys installs the compiled key bindings used by this dialog.
func (m *CreateModel) SetKeys(keys keybind.Map) {
	m.keys = keys
	m.selector = newSelector(keys)
	m.recurrenceE = NewRecurrence(keys)
	m.picker = datePickerNew(time.Now(), todayNine(time.Now()), keys)
	projectTextInputKeys(&m.titleInput, keys)
	projectTextInputKeys(&m.notesInput, keys)
}

func (m *CreateModel) Show(listName string) {
	m.now = time.Now()
	m.base = todayNine(m.now)
	m.picker = datePickerNew(m.now, m.base, m.keys)
	m.picker.SetSize(m.width, m.height)
	m.visible = true
	m.listName = listName
	m.editingID = ""
	m.focusIndex = 0
	m.errText = ""
	m.titleInput.SetValue("")
	m.notesInput.SetValue("")
	m.priority = reminders.PriorityNone
	m.recurrence = nil
	m.origRules = nil
	m.dueText = ""
	m.dueTouched = false
	m.recurrenceTouched = false
	m.recurrenceE.Hide()
	m.focusField(0)
}

func (m *CreateModel) ShowEdit(r reminders.Reminder) {
	m.now = time.Now()
	m.base = todayNine(m.now)
	if r.DueDate != nil {
		m.base = *r.DueDate
	}
	m.picker = datePickerNew(m.now, m.base, m.keys)
	m.picker.SetSize(m.width, m.height)
	m.visible = true
	m.editingID = r.ID
	m.listName = ""
	m.focusIndex = 0
	m.errText = ""

	m.titleInput.SetValue(r.Title)
	m.notesInput.SetValue(r.Notes)

	if r.DueDate != nil {
		m.picker.SetValue(r.DueDate.Local().Format("2006-01-02 15:04"))
	} else {
		m.picker.SetValue("")
	}
	m.dueText = m.picker.Value()
	m.dueTouched = false

	m.priority = r.Priority
	m.origRules = copyRules(r.RecurrenceRules)
	m.recurrence = copyRules(r.RecurrenceRules)
	m.recurrenceTouched = false
	m.recurrenceE.Hide()
	m.focusField(0)
}

func todayNine(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
}

func (m *CreateModel) Hide() {
	m.visible = false
	m.titleInput.Blur()
	m.notesInput.Blur()
	m.picker.Blur()
}

func (m CreateModel) Visible() bool {
	return m.visible
}

func (m CreateModel) isEditing() bool {
	return m.editingID != ""
}

func (m *CreateModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.picker.SetSize(width-4, height/3)
	m.selector.SetSize(width-8, min(12, height/2))
	m.recurrenceE.SetSize(width, height)
}

func (m *CreateModel) focusField(index int) {
	m.titleInput.Blur()
	m.notesInput.Blur()
	m.picker.Blur()
	switch index {
	case 0:
		m.titleInput.Focus()
	case 1:
		m.notesInput.Focus()
	case 2:
		m.picker.Focus()
	}
}

// recurrenceLabel renders the committed recurrence choice.
func recurrenceLabel(rules []eventkit.RecurrenceRule) string {
	if id := matchPreset(rules); id != "" {
		for _, p := range presets {
			if p.id == id {
				return p.label
			}
		}
	}
	if representable(rules) {
		return "Custom"
	}
	return "Existing custom schedule"
}

// priorityOptions are the four fixed choices in default display order.
func priorityOptions() []selectorOption {
	return []selectorOption{
		{ID: "none", Label: "None"},
		{ID: "low", Label: "Low"},
		{ID: "medium", Label: "Medium"},
		{ID: "high", Label: "High"},
	}
}

func priorityFromID(id string) int {
	switch id {
	case "low":
		return reminders.PriorityLow
	case "medium":
		return reminders.PriorityMedium
	case "high":
		return reminders.PriorityHigh
	default:
		return reminders.PriorityNone
	}
}

func priorityID(p int) string {
	switch p {
	case reminders.PriorityHigh:
		return "high"
	case reminders.PriorityMedium:
		return "medium"
	case reminders.PriorityLow:
		return "low"
	default:
		return "none"
	}
}

func (m CreateModel) Update(msg tea.Msg) (CreateModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	// Inner custom-repeat editor first.
	if m.recurrenceE.Visible() {
		var cmd tea.Cmd
		m.recurrenceE, cmd = m.recurrenceE.Update(msg)
		return m, cmd
	}

	// Editor results emitted after the editor closed itself.
	switch msg := msg.(type) {
	case RecurrenceSubmitMsg:
		m.recurrence = msg.Rules
		m.recurrenceTouched = true
		m.recurrenceE.Hide()
		return m, nil
	case RecurrenceCancelMsg:
		m.recurrenceE.Hide()
		return m, nil
	}

	// Outer chooser.
	if m.selector.Visible() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			var cmd tea.Cmd
			m.selector, cmd = m.selector.Update(msg)
			return m, cmd
		case tea.PasteMsg:
			var cmd tea.Cmd
			m.selector, cmd = m.selector.Update(msg)
			return m, cmd
		}
	}

	// Chooser results consumed by this form only.
	switch msg := msg.(type) {
	case selectorSelectedMsg:
		purpose := m.purpose
		m.purpose = chooserNone
		m.selector.Close()
		switch purpose {
		case chooserPriority:
			if len(msg.IDs) == 1 {
				m.priority = priorityFromID(msg.IDs[0])
			}
		case chooserRecurrence:
			if len(msg.IDs) == 1 {
				if msg.IDs[0] == "custom" {
					base := m.base
					if due, err := m.picker.Resolve(); err == nil && due != nil {
						base = *due
					}
					m.recurrenceE.Show(m.recurrence, m.now, base)
				} else {
					for _, p := range presets {
						if p.id == msg.IDs[0] {
							m.recurrence = p.build()
							m.recurrenceTouched = true
						}
					}
				}
			}
		}
		return m, nil
	case selectorCancelledMsg:
		m.purpose = chooserNone
		m.selector.Close()
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Choice fields: open on enter/space before outer submit.
		if m.focusIndex == 3 || m.focusIndex == 4 {
			if key.Matches(msg, m.keys.Bind("choice_field", "open")) {
				m.openOuterChooser(m.focusIndex, "")
				return m, nil
			}
		}
		switch {
		case key.Matches(msg, m.keys.Bind("dialog", "cancel")):
			m.Hide()
			return m, nil
		case key.Matches(msg, m.keys.Bind("dialog", "next_field")):
			if m.focusIndex == 2 && m.picker.TryAcceptSuggestion() {
				return m, nil
			}
			m.focusIndex = (m.focusIndex + 1) % fieldCount
			m.focusField(m.focusIndex)
			return m, nil
		case key.Matches(msg, m.keys.Bind("dialog", "previous_field")):
			if m.focusIndex == 2 && m.picker.TryAcceptSuggestion() {
				return m, nil
			}
			m.focusIndex = (m.focusIndex - 1 + fieldCount) % fieldCount
			m.focusField(m.focusIndex)
			return m, nil
		case key.Matches(msg, m.keys.Bind("dialog", "submit")):
			// Ctrl+S saves the outer form; inner chooser/editor handled above.
			cmd := m.trySubmit()
			return m, cmd
		}

		// Typing on a choice field opens its chooser, seeding the query.
		if m.focusIndex == 3 || m.focusIndex == 4 {
			if len(msg.Text) > 0 && msg.Mod == 0 {
				m.openOuterChooser(m.focusIndex, msg.Text)
				paste := tea.PasteMsg{Content: msg.Text}
				var cmd tea.Cmd
				m.selector, cmd = m.selector.Update(paste)
				return m, cmd
			}
			return m, nil
		}

		if m.focusIndex == 2 {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			m.dueTouched = m.dueTouched || m.picker.Value() != m.dueText
			return m, cmd
		}
	}

	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 1:
		m.notesInput, cmd = m.notesInput.Update(msg)
	case 2:
		m.picker, cmd = m.picker.Update(msg)
		m.dueTouched = m.dueTouched || m.picker.Value() != m.dueText
	}
	return m, cmd
}

// openOuterChooser opens the shared chooser for a choice field, optionally
// seeding the query with typed text.
func (m *CreateModel) openOuterChooser(focusIndex int, seed string) {
	if focusIndex == 3 {
		m.purpose = chooserPriority
		m.selector.Open(priorityOptions(), []string{priorityID(m.priority)}, false)
	} else {
		m.purpose = chooserRecurrence
		m.selector.Open(presetOptions(), []string{matchPreset(m.recurrence)}, false)
	}
	if seed != "" {
		paste := tea.PasteMsg{Content: seed}
		m.selector, _ = m.selector.Update(paste)
	}
}

// trySubmit validates the form and emits Create/Edit submit messages.
func (m *CreateModel) trySubmit() tea.Cmd {
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		return nil
	}

	due, dueErr := m.picker.Resolve()
	if dueErr != nil {
		m.errText = dueErr.Error()
		m.focusIndex = 2
		m.focusField(2)
		return nil
	}

	if len(m.recurrence) > 0 && due == nil {
		m.errText = "Repeating reminders need a due date"
		m.focusIndex = 2
		m.focusField(2)
		return nil
	}

	m.errText = ""
	m.Hide()
	if m.isEditing() {
		return m.buildEditCmd(title, due)
	}
	input := reminders.CreateReminderInput{
		Title:           title,
		ListName:        m.listName,
		Notes:           strings.TrimSpace(m.notesInput.Value()),
		DueDate:         due,
		Priority:        m.priority,
		RecurrenceRules: m.recurrence,
	}
	return func() tea.Msg {
		return CreateSubmitMsg{Input: input}
	}
}

func (m CreateModel) buildEditCmd(title string, due *time.Time) tea.Cmd {
	id := m.editingID
	input := reminders.UpdateReminderInput{
		Title: &title,
	}

	notes := strings.TrimSpace(m.notesInput.Value())
	input.Notes = &notes

	if m.dueTouched {
		if m.picker.Value() == "" {
			input.ClearDueDate = true
		} else {
			input.DueDate = due
		}
	}

	priority := m.priority
	input.Priority = &priority

	if m.recurrenceTouched && !rulesEqual(m.recurrence, m.origRules) {
		rules := make([]eventkit.RecurrenceRule, 0, len(m.recurrence))
		rules = append(rules, m.recurrence...)
		input.RecurrenceRules = &rules
	}

	return func() tea.Msg {
		return EditSubmitMsg{ID: id, Input: input}
	}
}

func (m CreateModel) View() string {
	if !m.visible {
		return ""
	}

	dialogTitle := "New Reminder"
	footer := keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": create  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": next field  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel"
	if m.isEditing() {
		dialogTitle = "Edit Reminder"
		footer = keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": save  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": next field  " +
			keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel"
	}

	title := styles.DialogTitleStyle.Render(dialogTitle)

	// Inner chooser / custom-repeat editor replace the form content.
	if m.selector.Visible() {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			dialogBoxStyle(m.width).Render(title+"\n\n"+m.selector.View()))
	}
	if m.recurrenceE.Visible() {
		return m.recurrenceE.View()
	}

	dueLine := m.picker.Value()
	if dueLine == "" {
		dueLine = "None"
	}
	recurrenceValue := recurrenceLabel(m.recurrence)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(m.titleInput.View())
	b.WriteString("\n\n")
	b.WriteString(m.notesInput.View())
	b.WriteString("\n\n")
	if m.focusIndex == 2 {
		b.WriteString(m.picker.View())
	} else {
		b.WriteString("Due date: " + dueLine)
	}
	b.WriteString("\n\n")
	b.WriteString("Priority:   " + priorityLabel(m.priority) + "  ▸")
	b.WriteString("\n\n")
	b.WriteString("Repeat:     " + recurrenceValue + "  ▸")
	b.WriteString("\n")
	if m.errText != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(styles.Red).Render(m.errText) + "\n")
	}
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(styles.DimGray).Render(footer))

	dlg := dialogBoxStyle(m.width).Render(b.String())
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dlg,
	)
}
