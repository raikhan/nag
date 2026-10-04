package dialog

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/BRO3886/go-eventkit"
	"github.com/charmbracelet/x/ansi"

	"github.com/oronbz/nag/internal/dateentry"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/timeentry"
	"github.com/oronbz/nag/internal/ui/styles"
)

type CreateSubmitMsg struct {
	Input reminders.CreateReminderInput
}

type EditSubmitMsg struct {
	ID    string
	Input reminders.UpdateReminderInput
}

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

// formField identifies one of the six main form rows.
type formField int

const (
	fieldTitle formField = iota
	fieldNotes
	fieldDate
	fieldTime
	fieldPriority
	fieldRecurrence
)

const formFieldCount = 6

const noField formField = -1

// formMode distinguishes browsing the six rows from editing one field.
type formMode int

const (
	formBrowsing formMode = iota
	formEditing
)

type CreateModel struct {
	keys        keybind.Map
	titleInput  textinput.Model
	notesInput  textarea.Model
	timeInput   textinput.Model
	picker      datepickerModel
	priority    int
	recurrence  []eventkit.RecurrenceRule
	selector    selectorModel
	purpose     chooserPurpose
	recurrenceE RecurrenceModel

	mode     formMode
	selected formField
	active   formField
	// follow advances the form to the next row each time the user
	// finishes a field; Recurrence is the last step and rests at the menu.
	follow bool
	// choiceOwner retains which outer field owns the open selector or
	// custom editor across its close-before-result interval; input is
	// ignored while a result is pending.
	choiceOwner formField
	choiceOpen  bool

	// Committed Date/Time drafts and their parsed caches.
	committedDate string // date text or ""
	committedTime string // raw time text or ""
	timeClock     *timeentry.Clock
	// timeDraft is the live parse of the open Time editor.
	timeDraft    *timeentry.Clock
	timeDraftErr string
	// editSnapshot holds the value an open simple editor started with.
	editSnapshot string

	// Original due state at open, for patch comparison.
	origDue   *time.Time
	origRules []eventkit.RecurrenceRule

	now  time.Time
	base time.Time

	visible   bool
	listName  string
	editingID string
	width     int
	height    int
	errText   string

	// Cached pane state: rebuilt after every state change, never per frame.
	viewport   viewport.Model
	cachedView string
	tooSmall   bool
	paneW      int
	paneH      int
}

func NewCreate() CreateModel {
	ti := textinput.New()
	ti.Placeholder = "Buy groceries"
	ti.CharLimit = 256
	ti.SetWidth(58)
	ti.Prompt = "Title:    "
	applyInputStyles(&ti)

	ni := textarea.New()
	ni.Placeholder = "Optional notes"
	ni.CharLimit = 1024
	ni.Prompt = ""
	ni.SetWidth(58)
	ni.SetHeight(3)

	tmi := textinput.New()
	tmi.Placeholder = "e.g. 6pm, 14:13"
	tmi.CharLimit = 32
	tmi.SetWidth(20)
	tmi.Prompt = "Time:     "
	applyInputStyles(&tmi)

	return CreateModel{
		titleInput: ti,
		notesInput: ni,
		timeInput:  tmi,
		priority:   reminders.PriorityNone,
		active:     noField,
		selected:   fieldTitle,
		viewport:   viewport.New(viewport.WithWidth(1), viewport.WithHeight(1)),
	}
}

// SetKeys installs the compiled key bindings used by this dialog.
func (m *CreateModel) SetKeys(keys keybind.Map) {
	m.keys = keys
	m.selector = newSelector(keys)
	m.recurrenceE = NewRecurrence(keys)
	m.picker = datePickerNew(time.Now(), midnightOf(time.Now()), keys)
	projectTextInputKeys(&m.titleInput, keys, "field")
	km := textarea.DefaultKeyMap()
	km.InsertNewline = key.NewBinding(key.WithKeys(keys.Aliases("field", "notes_newline")...))
	m.notesInput.KeyMap = km
	projectTextInputKeys(&m.timeInput, keys, "field")
}

func midnightOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Show opens the create form for a list.
func (m *CreateModel) Show(listName string) {
	m.showAt(listName, time.Now())
}

func (m *CreateModel) showAt(listName string, now time.Time) {
	m.now = now
	m.base = midnightOf(now)
	m.picker = datePickerNew(m.now, m.base, m.keys)
	m.visible = true
	m.listName = listName
	m.editingID = ""
	m.resetFormState()
	// Create opens on the first row, ready to type; resetFormState
	// rendered the menu, so the cached frame is rebuilt after opening.
	m.openField(fieldTitle)
	m.rebuildPane()
}

// ShowEdit opens the edit form prefilled from an existing reminder. The
// imported due timestamp is converted to the captured local location before
// prefilling Date and Time.
func (m *CreateModel) ShowEdit(r reminders.Reminder) {
	m.showEditAt(r, time.Now())
}

func (m *CreateModel) showEditAt(r reminders.Reminder, now time.Time) {
	m.now = now
	loc := now.Location()
	if r.DueDate != nil {
		m.base = midnightOf(r.DueDate.In(loc))
	} else {
		m.base = midnightOf(now)
	}
	m.picker = datePickerNew(m.now, m.base, m.keys)
	m.visible = true
	m.editingID = r.ID
	m.listName = ""
	m.resetFormState()
	// Editing starts in the left-hand menu; follow is a create-time
	// convenience, not a default the user did not ask for here.
	m.follow = false

	m.titleInput.SetValue(r.Title)
	m.notesInput.SetValue(r.Notes)

	if r.DueDate != nil {
		local := r.DueDate.In(loc)
		dateText := local.Format("2006-01-02")
		clock := timeentry.Clock{Hour: local.Hour(), Minute: local.Minute()}
		timeText := clock.String()
		m.picker.SetValue(dateText)
		m.timeInput.SetValue(timeText)
		m.committedDate = dateText
		m.committedTime = timeText
		c := clock
		m.timeClock = &c
		due := local
		m.origDue = &due
	}
	m.priority = r.Priority
	m.setOrigRules(r.RecurrenceRules)
	m.rebuildPane()
}

// resetFormState clears interaction state shared by Show and ShowEdit.
func (m *CreateModel) resetFormState() {
	m.mode = formBrowsing
	m.selected = fieldTitle
	m.active = noField
	m.follow = true
	m.choiceOwner = noField
	m.choiceOpen = false
	m.purpose = chooserNone
	m.errText = ""
	m.priority = reminders.PriorityNone
	m.recurrence = nil
	m.origRules = nil
	m.committedDate = ""
	m.committedTime = ""
	m.timeClock = nil
	m.timeDraft = nil
	m.timeDraftErr = ""
	m.editSnapshot = ""
	m.origDue = nil
	m.titleInput.SetValue("")
	m.notesInput.SetValue("")
	m.timeInput.SetValue("")
	m.picker.SetValue("")
	m.blurAll()
	m.recurrenceE.Hide()
	m.recurrenceE.Reset()
	m.selector.Close()
	m.rebuildPane()
}

// origRules snapshot for recurrence patch comparison (set by ShowEdit).
func (m *CreateModel) setOrigRules(rules []eventkit.RecurrenceRule) {
	m.origRules = copyRules(rules)
	m.recurrence = copyRules(rules)
}

func (m *CreateModel) blurAll() {
	m.titleInput.Blur()
	m.notesInput.Blur()
	m.timeInput.Blur()
	m.picker.Blur()
}

func (m *CreateModel) Hide() {
	m.visible = false
	m.blurAll()
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
	m.rebuildPane()
}

// isSimpleField reports whether the field is edited inline (text, date or
// time editor) rather than through the shared selector.
func isSimpleField(f formField) bool {
	return f == fieldTitle || f == fieldNotes || f == fieldDate || f == fieldTime
}

// openField begins editing a field. Choice fields open the shared selector.
func (m *CreateModel) openField(f formField) {
	m.mode = formEditing
	m.active = f
	m.blurAll()
	switch f {
	case fieldTitle:
		m.editSnapshot = m.titleInput.Value()
		m.titleInput.Focus()
	case fieldNotes:
		m.editSnapshot = m.notesInput.Value()
		m.notesInput.Focus()
	case fieldDate:
		m.editSnapshot = m.picker.Value()
		m.picker.Focus()
	case fieldTime:
		m.editSnapshot = m.timeInput.Value()
		m.timeInput.Focus()
		m.updateTimeDraft()
	case fieldPriority:
		m.choiceOwner = fieldPriority
		m.choiceOpen = true
		m.openOuterChooser(fieldPriority, "")
	case fieldRecurrence:
		m.choiceOwner = fieldRecurrence
		m.choiceOpen = true
		m.openOuterChooser(fieldRecurrence, "")
	}
}

// closeField leaves editing and returns to browsing the same row.
func (m *CreateModel) closeField() {
	m.mode = formBrowsing
	m.selected = m.active
	m.active = noField
	m.blurAll()
}

// advanceAfter closes a finished field. In follow mode it opens the next
// row; follow rests at the menu once Recurrence has been finished.
func (m *CreateModel) advanceAfter(f formField) {
	m.closeField()
	if !m.follow || f == fieldRecurrence {
		return
	}
	next := (f + 1) % formFieldCount
	m.selected = next
	m.openField(next)
}

// notesEditorDoneMsg carries the result of an external $EDITOR session on
// the Notes field.
type notesEditorDoneMsg struct {
	content string
	err     error
}

// openNotesEditor suspends the TUI and edits the Notes value in $EDITOR
// (falling back to VISUAL/vi). The completion message reaches update() as
// notesEditorDoneMsg.
func (m *CreateModel) openNotesEditor() tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	f, err := os.CreateTemp("", "nag-notes-*.txt")
	if err != nil {
		m.errText = err.Error()
		return nil
	}
	path := f.Name()
	if _, err := f.WriteString(m.notesInput.Value()); err != nil {
		f.Close()
		os.Remove(path)
		m.errText = err.Error()
		return nil
	}
	f.Close()
	c := exec.Command(editor, path)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		content, rerr := os.ReadFile(path)
		os.Remove(path)
		if rerr != nil {
			return notesEditorDoneMsg{err: rerr}
		}
		return notesEditorDoneMsg{content: strings.TrimSuffix(string(content), "\n"), err: err}
	})
}

// cancelField restores the opening value and returns to browsing.
func (m *CreateModel) cancelField() {
	switch m.active {
	case fieldTitle:
		m.titleInput.SetValue(m.editSnapshot)
	case fieldNotes:
		m.notesInput.SetValue(m.editSnapshot)
	case fieldDate:
		m.picker.SetValue(m.editSnapshot)
	case fieldTime:
		m.timeInput.SetValue(m.editSnapshot)
	}
	m.errText = ""
	m.closeField()
}

// commitField validates and commits one simple field. It reports whether the
// commit succeeded; on failure the error text is set and the field stays
// open.
func (m *CreateModel) commitField(f formField) bool {
	switch f {
	case fieldTitle, fieldNotes:
		m.errText = ""
		return true
	case fieldDate:
		raw := strings.TrimSpace(m.picker.Value())
		if raw == "" {
			m.committedDate = ""
			// Clearing the date also clears the time draft.
			m.committedTime = ""
			m.timeClock = nil
			m.timeInput.SetValue("")
			m.timeDraft = nil
			m.timeDraftErr = ""
			m.errText = ""
			return true
		}
		resolved, err := dateentry.Parse(raw, m.now, m.base)
		if err != nil {
			m.errText = err.Error()
			return false
		}
		m.committedDate = resolved.Format("2006-01-02")
		m.picker.SetValue(m.committedDate)
		m.errText = ""
		return true
	case fieldTime:
		return m.commitTime(m.timeInput.Value())
	}
	return true
}

// commitTime resolves the Time field to a canonical clock and stores it. The
// input is rewritten to the resolved value, mirroring how the Date field
// canonicalizes "tom" on commit, so re-entering the field shows what is
// already committed.
func (m *CreateModel) commitTime(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	clock, present, err := timeentry.Parse(trimmed, m.now)
	if err != nil {
		m.errText = err.Error()
		return false
	}
	if present {
		m.committedTime = clock.String()
		m.timeInput.SetValue(m.committedTime)
		c := clock
		m.timeClock = &c
	} else {
		m.committedTime = ""
		m.timeInput.SetValue("")
		m.timeClock = nil
	}
	m.errText = ""
	return true
}

// updateTimeDraft refreshes the live Time editor parse; called only when the
// input value changed.
func (m *CreateModel) updateTimeDraft() {
	raw := m.timeInput.Value()
	m.timeDraft = nil
	m.timeDraftErr = ""
	if strings.TrimSpace(raw) == "" {
		return
	}
	clock, present, err := timeentry.Parse(strings.TrimSpace(raw), m.now)
	switch {
	case err != nil:
		m.timeDraftErr = err.Error()
	case present:
		c := clock
		m.timeDraft = &c
	}
}

// composeDue builds the final due timestamp exactly once from the committed
// civil date plus clock in the captured local location. Blank time with a
// date means 09:00 local. A nonexistent DST clock is rejected.
func (m *CreateModel) composeDue() (*time.Time, error) {
	if m.committedDate == "" {
		return nil, nil
	}
	day, err := dateentry.Parse(m.committedDate, m.now, m.base)
	if err != nil {
		return nil, err
	}
	hour, minute := 9, 0
	if m.committedTime != "" {
		clock, present, err := timeentry.Parse(m.committedTime, m.now)
		if err != nil {
			return nil, err
		}
		if present {
			hour, minute = clock.Hour, clock.Minute
		}
	}
	loc := m.now.Location()
	t := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)
	if t.Year() != day.Year() || t.Month() != day.Month() || t.Day() != day.Day() ||
		t.Hour() != hour || t.Minute() != minute {
		return nil, fmt.Errorf("Time does not exist on that date in the local time zone")
	}
	return &t, nil
}

// dueStateChanged reports whether the committed date/time differs from the
// original civil values.
func (m *CreateModel) dueStateChanged() bool {
	switch {
	case m.origDue == nil && m.committedDate == "":
		return false
	case m.origDue == nil:
		return true
	case m.committedDate == "":
		return true
	}
	composed, err := m.composeDue()
	return err != nil || composed == nil || !composed.Equal(*m.origDue)
}

// duePatch converts the committed state into a due patch: a composed
// timestamp, a clear request, or nothing when unchanged.
func (m *CreateModel) duePatch() (*time.Time, bool, error) {
	composed, err := m.composeDue()
	if err != nil {
		return nil, false, err
	}
	switch {
	case m.origDue == nil && composed == nil:
		return nil, false, nil
	case m.origDue == nil:
		return composed, false, nil
	case composed == nil:
		return nil, true, nil
	case composed.Equal(*m.origDue):
		return nil, false, nil
	default:
		return composed, false, nil
	}
}

// anchorDue is the custom editor anchor: the composed due when present, the
// editing base otherwise.
func (m *CreateModel) anchorDue() time.Time {
	if due, err := m.composeDue(); err == nil && due != nil {
		return *due
	}
	return m.base
}

// zoneAbbrev renders the local zone abbreviation of the selected due
// timestamp, or of the captured clock when no date is present.
func (m CreateModel) zoneAbbrev() string {
	if due, err := m.composeDue(); err == nil && due != nil {
		return due.Format("MST")
	}
	return m.now.Format("MST")
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
		return customLabel(rules)
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

// jumpHint renders the first configured alias of a jump action in brackets,
// or empty spaces when the action is disabled.
func (m CreateModel) jumpHint(action string) string {
	aliases := m.keys.Aliases("form", action)
	if len(aliases) == 0 {
		return "    "
	}
	return "[" + keybind.ShortKey(aliases[0]) + "] "
}

func (m CreateModel) fieldLabel(f formField) string {
	switch f {
	case fieldTitle:
		return "Title"
	case fieldNotes:
		return "Notes"
	case fieldDate:
		return "Date"
	case fieldTime:
		return "Time"
	case fieldPriority:
		return "Priority"
	default:
		return "Recurrence"
	}
}

func (m CreateModel) jumpAction(f formField) string {
	switch f {
	case fieldTitle:
		return "jump_title"
	case fieldNotes:
		return "jump_notes"
	case fieldDate:
		return "jump_date"
	case fieldTime:
		return "jump_time"
	case fieldPriority:
		return "jump_priority"
	default:
		return "jump_recurrence"
	}
}

// summaryValue renders the committed value of a field, with a draft preview
// for the active one.
func (m CreateModel) summaryValue(f formField) string {
	switch f {
	case fieldTitle:
		return strings.TrimSpace(m.titleInput.Value())
	case fieldNotes:
		return strings.TrimSpace(strings.ReplaceAll(m.notesInput.Value(), "\n", " "))
	case fieldDate:
		if m.mode == formEditing && m.active == fieldDate {
			if v := strings.TrimSpace(m.picker.Value()); v != "" {
				if resolved, err := dateentry.Parse(v, m.now, m.base); err == nil {
					return resolved.Format("2006-01-02")
				}
				return v
			}
			return "None"
		}
		if m.committedDate != "" {
			return m.committedDate
		}
		return "None"
	case fieldTime:
		clock := m.timeClock
		if m.mode == formEditing && m.active == fieldTime {
			clock = m.timeDraft
			if m.timeDraft != nil {
				return m.timeDraft.String() + " " + m.zoneAbbrev()
			}
			if strings.TrimSpace(m.timeInput.Value()) != "" {
				return strings.TrimSpace(m.timeInput.Value())
			}
			return "None"
		}
		if clock != nil {
			return clock.String() + " " + m.zoneAbbrev()
		}
		return "None"
	case fieldPriority:
		return priorityLabel(m.priority)
	default:
		return recurrenceLabel(m.recurrence)
	}
}

// summaryLines renders the six left-pane rows, each padded to exactly maxW
// columns so the divider column stays fixed; long values are truncated.
func (m CreateModel) summaryLines(maxW int) []string {
	selectedStyle := lipgloss.NewStyle().Foreground(styles.Teal).Bold(true).Width(maxW)
	normalStyle := lipgloss.NewStyle().Width(maxW)
	lines := make([]string, 0, formFieldCount)
	for f := formField(0); f < formFieldCount; f++ {
		marker := "  "
		style := normalStyle
		if f == m.selected || (m.mode == formEditing && f == m.active) {
			marker = "> "
			style = selectedStyle
		}
		line := marker + m.jumpHint(m.jumpAction(f)) + fmt.Sprintf("%-11s", m.fieldLabel(f)) + m.summaryValue(f)
		line = ansi.Truncate(line, maxW, "…")
		lines = append(lines, style.Render(line))
	}
	return lines
}

func (m CreateModel) Update(msg tea.Msg) (CreateModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	m2, cmd := m.update(msg)
	m2.rebuildPane()
	return m2, cmd
}

func (m CreateModel) update(msg tea.Msg) (CreateModel, tea.Cmd) {
	// Inner custom-repeat editor captures everything while visible.
	if m.recurrenceE.Visible() {
		var cmd tea.Cmd
		m.recurrenceE, cmd = m.recurrenceE.Update(msg)
		m.follow = m.recurrenceE.follow
		return m, cmd
	}

	// Result messages are consumed before visibility branches and only for
	// the pending owner.
	switch msg := msg.(type) {
	case RecurrenceSubmitMsg:
		m.recurrence = msg.Rules
		m.recurrenceE.Hide()
		m.finishChoice(true)
		return m, nil
	case RecurrenceCancelMsg:
		m.recurrenceE.Hide()
		m.finishChoice(false)
		return m, nil
	case selectorSelectedMsg:
		purpose := m.purpose
		m.purpose = chooserNone
		m.selector.Close()
		switch purpose {
		case chooserPriority:
			if len(msg.IDs) == 1 {
				m.priority = priorityFromID(msg.IDs[0])
			}
			m.finishChoice(true)
		case chooserRecurrence:
			if len(msg.IDs) == 1 && msg.IDs[0] == "custom" {
				// Switch into the custom editor; the owner stays retained
				// until its Apply/Cancel result is consumed.
				m.recurrenceE.Show(m.recurrence, m.now, m.anchorDue(), m.follow)
				m.recurrenceE.SetSize(m.paneW, m.paneH)
				return m, nil
			}
			if len(msg.IDs) == 1 {
				for _, p := range presets {
					if p.id == msg.IDs[0] {
						m.recurrence = p.build()
					}
				}
			}
			m.finishChoice(true)
		}
		return m, nil
	case selectorCancelledMsg:
		m.purpose = chooserNone
		m.selector.Close()
		m.finishChoice(false)
		return m, nil
	case notesEditorDoneMsg:
		if msg.err != nil {
			m.errText = "Editor failed: " + msg.err.Error()
			return m, nil
		}
		m.errText = ""
		m.notesInput.SetValue(msg.content)
		// An $EDITOR session finishes the Notes field, so follow moves
		// on; with follow off the field stays open as before.
		if m.follow && m.active == fieldNotes {
			m.advanceAfter(fieldNotes)
		}
		return m, nil
	}

	// The outer selector captures all input while open.
	if m.selector.Visible() {
		var cmd tea.Cmd
		m.selector, cmd = m.selector.Update(msg)
		return m, cmd
	}

	// Input is ignored while a selector/custom result is pending.
	if m.choiceOpen {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if m.mode == formBrowsing {
			switch {
			case key.Matches(keyMsg, m.keys.Bind("form", "follow_mode")):
				// Toggling only arms or disarms the mode; the cursor
				// never moves.
				m.follow = !m.follow
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "save")):
				cmd := m.trySubmit()
				return m, cmd
			case key.Matches(keyMsg, m.keys.Bind("form", "cancel")):
				m.Hide()
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "next_field")):
				m.selected = (m.selected + 1) % formFieldCount
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "previous_field")):
				m.selected = (m.selected - 1 + formFieldCount) % formFieldCount
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "edit")):
				m.openField(m.selected)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_title")):
				m.selected = fieldTitle
				m.openField(fieldTitle)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_notes")):
				m.selected = fieldNotes
				m.openField(fieldNotes)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_date")):
				m.selected = fieldDate
				m.openField(fieldDate)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_time")):
				m.selected = fieldTime
				m.openField(fieldTime)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_priority")):
				m.selected = fieldPriority
				m.openField(fieldPriority)
				return m, nil
			case key.Matches(keyMsg, m.keys.Bind("form", "jump_recurrence")):
				m.selected = fieldRecurrence
				m.openField(fieldRecurrence)
				return m, nil
			}
			// Other typed keys and paste do nothing while browsing.
			return m, nil
		}

		// Editing a simple field: intercept field controls and form.save.
		switch {
		case key.Matches(keyMsg, m.keys.Bind("form", "follow_mode")):
			// Toggling only arms or disarms the mode; the cursor never
			// moves and the open editor keeps its state.
			m.follow = !m.follow
			return m, nil
		case m.active == fieldTime && key.Matches(keyMsg, m.keys.Bind("calendar", "reset")):
			// Clearing an open draft mirrors the Date field: only the
			// draft changes here, the committed value follows on finish.
			m.timeInput.SetValue("")
			m.updateTimeDraft()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("form", "save")):
			cmd := m.trySubmit()
			return m, cmd
		case key.Matches(keyMsg, m.keys.Bind("field", "confirm")):
			if m.commitField(m.active) {
				m.advanceAfter(m.active)
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("field", "cancel")):
			m.cancelField()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("field", "external_editor")) && m.active == fieldNotes:
			if cmd := m.openNotesEditor(); cmd != nil {
				return m, cmd
			}
			return m, nil
		case keyMsg.Code == tea.KeyTab:
			// Tab/Shift-Tab never leave an open field editor; consume them
			// so no widget treats them as input.
			return m, nil
		}
	}

	// Forward everything else to the active simple editor.
	var cmd tea.Cmd
	switch m.active {
	case fieldTitle:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case fieldNotes:
		m.notesInput, cmd = m.notesInput.Update(msg)
	case fieldDate:
		m.picker, cmd = m.picker.Update(msg)
	case fieldTime:
		before := m.timeInput.Value()
		m.timeInput, cmd = m.timeInput.Update(msg)
		if m.timeInput.Value() != before {
			m.updateTimeDraft()
		}
	default:
		return m, nil
	}
	return m, cmd
}

// finishChoiceOwner returns to browsing the row that owned the consumed
// selector or custom editor result.
func (m *CreateModel) finishChoiceOwner() {
	m.choiceOpen = false
	if m.choiceOwner != noField {
		m.mode = formBrowsing
		m.selected = m.choiceOwner
	}
	m.choiceOwner = noField
	m.active = noField
	m.blurAll()
}

// finishChoice closes a consumed selector or custom-editor result. A
// confirmed choice advances in follow mode; a cancelled one rests at the
// menu, exactly like Esc in a simple field.
func (m *CreateModel) finishChoice(confirmed bool) {
	owner := m.choiceOwner
	m.finishChoiceOwner()
	if !confirmed || owner == noField || !m.follow || owner == fieldRecurrence {
		return
	}
	next := (owner + 1) % formFieldCount
	m.selected = next
	m.openField(next)
}

// openOuterChooser opens the shared chooser for a choice field.
func (m *CreateModel) openOuterChooser(f formField, seed string) {
	if f == fieldPriority {
		m.purpose = chooserPriority
		m.selector.Open(priorityOptions(), []string{priorityID(m.priority)}, false)
	} else {
		m.purpose = chooserRecurrence
		m.selector.Open(presetOptions(), []string{recurrenceChoiceID(m.recurrence)}, false)
	}
	if seed != "" {
		paste := tea.PasteMsg{Content: seed}
		m.selector, _ = m.selector.Update(paste)
	}
}

// trySubmit validates the form and emits Create/Edit submit messages.
func (m *CreateModel) trySubmit() tea.Cmd {
	if m.choiceOpen {
		return nil
	}
	// Saving from a simple editor validates and commits its draft first.
	if m.mode == formEditing && isSimpleField(m.active) {
		if !m.commitField(m.active) {
			return nil
		}
		m.closeField()
	}

	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.errText = "Title is required"
		m.selected = fieldTitle
		m.openField(fieldTitle)
		return nil
	}

	if m.committedTime != "" && m.committedDate == "" {
		m.errText = "Set a date before a time"
		m.selected = fieldDate
		m.openField(fieldDate)
		return nil
	}

	composed, err := m.composeDue()
	if err != nil {
		m.errText = err.Error()
		m.selected = fieldDate
		m.openField(fieldDate)
		return nil
	}
	due, clearDue, err := m.duePatch()
	if err != nil {
		m.errText = err.Error()
		m.selected = fieldDate
		m.openField(fieldDate)
		return nil
	}

	rulesChanged := !rulesEqual(m.recurrence, m.origRules)
	dueChanged := m.dueStateChanged()
	if len(m.recurrence) > 0 && composed == nil {
		clearingRules := clearDue && len(m.origRules) > 0
		unrelated := m.isEditing() && !rulesChanged && !dueChanged
		if !clearingRules && !unrelated {
			m.errText = "Repeating reminders need a due date"
			m.selected = fieldDate
			m.openField(fieldDate)
			return nil
		}
	}

	m.errText = ""
	m.Hide()
	if m.isEditing() {
		return m.buildEditCmd(title, due, clearDue)
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

func (m CreateModel) buildEditCmd(title string, due *time.Time, clearDue bool) tea.Cmd {
	id := m.editingID
	input := reminders.UpdateReminderInput{
		Title: &title,
	}

	notes := strings.TrimSpace(m.notesInput.Value())
	input.Notes = &notes

	if clearDue {
		input.ClearDueDate = true
	} else if due != nil {
		input.DueDate = due
	}

	priority := m.priority
	input.Priority = &priority

	rulesChanged := !rulesEqual(m.recurrence, m.origRules)
	if clearDue && len(m.origRules) > 0 {
		// Clearing the due date clears recurrence in the same save.
		empty := []eventkit.RecurrenceRule{}
		input.RecurrenceRules = &empty
	} else if rulesChanged {
		rules := make([]eventkit.RecurrenceRule, 0, len(m.recurrence))
		rules = append(rules, m.recurrence...)
		input.RecurrenceRules = &rules
	}

	return func() tea.Msg {
		return EditSubmitMsg{ID: id, Input: input}
	}
}

// rebuildPane recomputes child sizes, renders the active content into the
// viewport and applies focus scrolling. View only renders the cached frame.
func (m *CreateModel) rebuildPane() {
	if !m.visible {
		return
	}
	outerW := min(112, m.width-2)
	outerH := min(30, m.height-2)
	if m.width < 26 || m.height < 12 || outerW < 18 || outerH < 6 {
		m.tooSmall = true
		m.cachedView = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			"Terminal too small for the reminder form")
		return
	}
	m.tooSmall = false

	style := styles.DialogStyle.Width(outerW)
	frameW, frameH := style.GetFrameSize()
	contentW := outerW - frameW
	contentH := outerH - frameH

	// Measured reservation: DialogTitleStyle carries a bottom margin (2
	// lines), the footer is one line, and a sticky error adds a blank line
	// plus the error line.
	headerH, footerH := 2, 1
	errH := 0
	if m.errText != "" {
		errH = 2 // blank line + error line
	}
	bodyH := contentH - headerH - footerH - errH
	wide := contentW >= 70
	minBody := 6
	if !wide {
		minBody = 6 + 1 + 3 // summaries + gap + editor rows
	}
	if bodyH < minBody {
		m.tooSmall = true
		m.cachedView = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			"Terminal too small for the reminder form")
		return
	}

	var right string
	focus := 0
	var leftBlock string
	if wide {
		leftW := (contentW - 3) / 2
		rightW := contentW - leftW - 3
		m.paneW, m.paneH = rightW, bodyH
		right, focus = m.paneContent(rightW, bodyH)
		leftLines := m.summaryLines(leftW)
		for len(leftLines) < bodyH {
			leftLines = append(leftLines, "")
		}
		leftBlock = strings.Join(leftLines, "\n")
		sep := lipgloss.NewStyle().Width(3).Render(strings.Repeat(" │\n", bodyH))
		body := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, sep, right)
		m.renderBody(style, body, bodyH, focus, contentW)
		return
	}

	// Stacked: compact summaries above the active editor.
	m.paneW, m.paneH = contentW, bodyH
	leftLines := m.summaryLines(contentW)
	editor, editorFocus := m.paneContent(contentW, bodyH-7)
	summaryBlock := strings.Join(leftLines, "\n") + "\n\n"
	right = summaryBlock + editor
	focus = editorFocus + 7
	m.renderBody(style, right, bodyH, focus, contentW)
}

// renderBody sizes the viewport to the full body width, applies focus
// scrolling and caches the complete dialog frame.
func (m *CreateModel) renderBody(style lipgloss.Style, body string, bodyH, focus, bodyW int) {
	m.viewport.SetWidth(max(1, bodyW))
	m.viewport.SetHeight(bodyH)
	m.viewport.SetContent(body)
	m.viewport.EnsureVisible(focus, 0, max(0, bodyW-1))

	dialogTitle := "New Reminder"
	if m.isEditing() {
		dialogTitle = "Edit Reminder"
	}
	var b strings.Builder
	b.WriteString(styles.DialogTitleStyle.Render(dialogTitle))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	if m.errText != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(styles.Red).Render(m.errText))
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).Render(m.footerHints()))
	m.cachedView = lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(b.String()))
}

// hint joins nonempty hint fragments with two spaces, omitting disabled
// actions.
func hint(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "  ")
}

// followHint renders the toggle with the state the key switches to, or
// empty when the action is disabled.
func followHint(keys keybind.Map, follow bool) string {
	k := short(keys, "form", "follow_mode")
	if k == "" {
		return ""
	}
	if follow {
		return k + " follow:off"
	}
	return k + " follow:on"
}

func (m CreateModel) followHint() string { return followHint(m.keys, m.follow) }

func (m CreateModel) footerHints() string {
	if m.mode == formEditing && isSimpleField(m.active) {
		if m.active == fieldNotes {
			return hint(
				short(m.keys, "field", "confirm")+" finish",
				short(m.keys, "field", "cancel")+" cancel",
				short(m.keys, "field", "notes_newline")+" newline",
				short(m.keys, "field", "external_editor")+" editor",
				short(m.keys, "form", "save")+" save",
				m.followHint(),
			)
		}
		if m.active == fieldDate {
			return hint(
				short(m.keys, "field", "confirm")+" finish",
				short(m.keys, "field", "cancel")+" cancel",
				short(m.keys, "form", "save")+" save",
				firstShort(m.keys, "calendar", "left")+"/"+firstShort(m.keys, "calendar", "right")+" day",
				firstShort(m.keys, "calendar", "up")+"/"+firstShort(m.keys, "calendar", "down")+" week",
				firstShort(m.keys, "calendar", "month_prev")+"/"+firstShort(m.keys, "calendar", "month_next")+" month",
				firstShort(m.keys, "calendar", "reset")+" reset",
				m.followHint(),
			)
		}
		if m.active == fieldTime {
			return hint(
				short(m.keys, "field", "confirm")+" finish",
				short(m.keys, "field", "cancel")+" cancel",
				short(m.keys, "form", "save")+" save",
				firstShort(m.keys, "calendar", "reset")+" clear",
				m.followHint(),
			)
		}
		return hint(
			short(m.keys, "field", "confirm")+" finish",
			short(m.keys, "field", "cancel")+" cancel",
			short(m.keys, "form", "save")+" save",
			m.followHint(),
		)
	}
	if m.selector.Visible() || m.recurrenceE.Visible() {
		return ""
	}
	return hint(
		short(m.keys, "form", "next_field")+" next",
		short(m.keys, "form", "previous_field")+" prev",
		short(m.keys, "form", "edit")+" edit",
		short(m.keys, "form", "save")+" save",
		short(m.keys, "form", "cancel")+" cancel",
		m.followHint(),
	)
}

// short renders the configured aliases of scope.action, empty when disabled.
func short(keys keybind.Map, scope, action string) string {
	return keybind.ShortKeys(keys.Aliases(scope, action))
}

// firstShort renders the first configured alias of scope.action, empty when
// the action is disabled.
func firstShort(keys keybind.Map, scope, action string) string {
	aliases := keys.Aliases(scope, action)
	if len(aliases) == 0 {
		return ""
	}
	return keybind.ShortKey(aliases[0])
}

// paneContent renders the right-hand pane for the current state and returns
// its focus line. It sizes the active child to the actual pane dimensions.
func (m *CreateModel) paneContent(width, height int) (string, int) {
	if m.recurrenceE.Visible() {
		m.recurrenceE.SetSize(width, height)
		return m.recurrenceE.View(), m.recurrenceE.FocusLine()
	}
	if m.selector.Visible() {
		m.selector.SetSize(width, height)
		return m.selector.View(), 0
	}
	if m.mode == formEditing {
		switch m.active {
		case fieldTitle:
			m.titleInput.SetWidth(width)
			return m.titleInput.View(), 0
		case fieldNotes:
			m.notesInput.SetWidth(max(10, width-2))
			m.notesInput.SetHeight(height)
			return m.notesInput.View(), 0
		case fieldDate:
			return m.picker.View(), m.picker.FocusLine()
		case fieldTime:
			m.timeInput.SetWidth(width)
			var b strings.Builder
			b.WriteString(m.timeInput.View())
			b.WriteString("\n")
			switch {
			case m.timeDraftErr != "":
				b.WriteString(lipgloss.NewStyle().Foreground(styles.Red).Render(m.timeDraftErr))
			case m.timeDraft != nil:
				b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).
					Render("→ " + m.timeDraft.String() + " " + m.zoneAbbrev()))
			}
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).
				Render("Blank time with a date means 09:00 " + m.zoneAbbrev()))
			return b.String(), 0
		}
	}

	// Browsing: the selected value with edit/save instructions.
	name := m.fieldLabel(m.selected)
	value := m.summaryValue(m.selected)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(styles.Teal).Bold(true).Render(name))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Width(width).Render(value))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).Render(hint(
		short(m.keys, "form", "edit")+" edit",
		short(m.keys, "form", "save")+" save",
		short(m.keys, "form", "cancel")+" cancel form",
	)))
	return b.String(), 0
}

// View renders the cached dialog frame.
func (m CreateModel) View() string {
	if !m.visible {
		return ""
	}
	return m.cachedView
}
