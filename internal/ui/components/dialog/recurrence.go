package dialog

import (
	"sort"
	"strconv"
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

// RecurrenceSubmitMsg carries a valid custom schedule draft.
type RecurrenceSubmitMsg struct{ Rules []eventkit.RecurrenceRule }

// RecurrenceCancelMsg closes the editor without changing committed recurrence.
type RecurrenceCancelMsg struct{}

type selPurpose int

const (
	spNone selPurpose = iota
	spFrequency
	spWeeklyDays
	spMonthlyMode
	spMonthlyDays
	spOrdinal
	spDayKind
	spYearlyMonths
	spYearlyPattern
	spYearlyWeekday
	spEnd
)

type fieldKind int

const (
	fFrequency fieldKind = iota
	fInterval
	fWeeklyDays
	fMonthlyMode
	fMonthlyDays
	fOrdinal
	fDayKind
	fYearlyMonths
	fYearlyPattern
	fYearlyOrdinal
	fYearlyWeekday
	fEnd
	fEndValue
)

const (
	endNever = iota
	endOnDate
	endAfter
)

const (
	monthlyEach = iota
	monthlyOnThe
)

type choiceField struct {
	kind     fieldKind
	name     string
	value    string
	isChoice bool
}

// RecurrenceModel is the nested Custom repeat editor: Frequency, Interval,
// frequency-specific controls, End repeat and its conditional input.
type RecurrenceModel struct {
	keys     keybind.Map
	selector selectorModel
	purpose  selPurpose

	visible bool
	width   int
	height  int
	now     time.Time
	base    time.Time

	frequency    eventkit.RecurrenceFrequency
	intervalIn   textinput.Model
	weeklyDays   []bool // Monday..Sunday
	monthlyMode  int
	monthlyDays  []bool // 1..31 plus Last day
	ordinal      int    // 1..5 or -1
	dayKindV     dayKind
	dayKindWd    eventkit.Weekday
	yearlyMonths []bool // January..December
	yearlyOn     bool   // false = Same day
	endMode      int
	endCount     string
	endPicker    datepickerModel
	endDateOn    bool

	errText string
	banner  string

	focusIndex int
}

// NewRecurrence builds the editor with the given compiled bindings.
func NewRecurrence(keys keybind.Map) RecurrenceModel {
	interval := textinput.New()
	interval.Placeholder = "1"
	interval.CharLimit = 6
	interval.Prompt = "Every: "
	applyInputStyles(&interval)

	return RecurrenceModel{
		keys:         keys,
		selector:     newSelector(keys),
		intervalIn:   interval,
		weeklyDays:   make([]bool, 7),
		monthlyDays:  make([]bool, 32),
		yearlyMonths: make([]bool, 12),
	}
}

// SetKeys installs the compiled bindings.
func (m *RecurrenceModel) SetKeys(keys keybind.Map) {
	m.keys = keys
	m.selector.SetKeys(keys)
}

func (m *RecurrenceModel) initIntervalKeys() {
	projectTextInputKeys(&m.intervalIn, m.keys)
}

// Show opens the editor prefilled from rules. base is the editing anchor
// (the reminder's due date when known, otherwise today at 09:00). Multiple
// or unrepresentable rules start a replacement draft and show a banner.
func (m *RecurrenceModel) Show(rules []eventkit.RecurrenceRule, now, base time.Time) {
	m.visible = true
	m.now, m.base = now, base
	m.errText = ""
	m.purpose = spNone
	m.endPicker = datePickerNew(now, base, m.keys)
	m.initIntervalKeys()
	m.intervalIn.SetValue("1")

	m.resetDraft()
	prefilled := prefillFields(rules)
	if prefilled != nil {
		m.applyPrefill(*prefilled)
		if !rulesEqual(rules, m.draftRules()) {
			// draft semantics differ; keep prefill anyway
			_ = rules
		}
	} else if len(rules) > 0 {
		m.banner = "Applying replaces the existing custom schedule"
	} else {
		m.banner = ""
	}
	m.focusIndex = 0
	m.blurAll()
	m.focusField()
}

// resetDraft starts a fresh Custom draft: Daily, interval 1, Never.
func (m *RecurrenceModel) resetDraft() {
	m.frequency = eventkit.FrequencyDaily
	m.weeklyDays = make([]bool, 7)
	m.monthlyDays = make([]bool, 32)
	m.yearlyMonths = make([]bool, 12)
	m.monthlyMode = monthlyEach
	m.yearlyOn = false
	m.endMode = endNever
	m.endCount = ""
	m.endDateOn = false
	m.banner = ""
	anchor := m.anchor()
	m.initFrequencySpecific(anchor)
}

func (m *RecurrenceModel) anchor() time.Time {
	if !m.base.IsZero() {
		return m.base
	}
	return m.now
}

// initFrequencySpecific initializes new controls from the anchor.
func (m *RecurrenceModel) initFrequencySpecific(anchor time.Time) {
	switch anchor.Weekday() {
	case time.Sunday:
		m.weeklyDays[6] = true
	default:
		m.weeklyDays[int(anchor.Weekday())-1] = true
	}
	day := anchor.Day()
	if day > 31 {
		day = 31
	}
	m.monthlyDays[day-1] = true
	m.ordinal = (day-1)/7 + 1
	m.dayKindV = dayKindSpecific
	m.dayKindWd = weekdayFromGo(anchor.Weekday())
	m.yearlyMonths[int(anchor.Month())-1] = true
	m.yearlyOn = false
}

func weekdayFromGo(w time.Weekday) eventkit.Weekday {
	if w == time.Sunday {
		return eventkit.Sunday // 1
	}
	return eventkit.Weekday(int(w) + 1) // Monday=1 -> 2 … Saturday=6 -> 7
}

func (m *RecurrenceModel) applyPrefill(p prefill) {
	m.frequency = p.frequency
	m.intervalIn.SetValue(strconv.Itoa(p.interval))
	m.endMode = p.endMode
	switch p.endMode {
	case endOnDate:
		if !p.end.IsZero() {
			m.endPicker.SetValue(p.end.Local().Format("2006-01-02"))
			m.endDateOn = true
		}
	case endAfter:
		m.endCount = strconv.Itoa(p.count)
	}
	switch p.frequency {
	case eventkit.FrequencyWeekly:
		for i := range m.weeklyDays {
			m.weeklyDays[i] = false
		}
		for _, d := range p.weekly {
			m.weeklyDays[d] = true
		}
	case eventkit.FrequencyMonthly:
		if p.each {
			m.monthlyMode = monthlyEach
			for i := range m.monthlyDays {
				m.monthlyDays[i] = false
			}
			for _, d := range p.days {
				if d == -1 {
					m.monthlyDays[31] = true
				} else {
					m.monthlyDays[d-1] = true
				}
			}
		} else {
			m.monthlyMode = monthlyOnThe
			m.ordinal = p.ordinal
			m.dayKindV = p.kind
			m.dayKindWd = p.weekday
		}
	case eventkit.FrequencyYearly:
		for i := range m.yearlyMonths {
			m.yearlyMonths[i] = false
		}
		for _, mo := range p.months {
			m.yearlyMonths[mo-1] = true
		}
		m.yearlyOn = p.patternOn
		if p.patternOn {
			m.ordinal = p.ordinal
			m.dayKindWd = p.weekday
		}
	}
}

// prefill describes a draft decoded from an original rule.
type prefill struct {
	frequency eventkit.RecurrenceFrequency
	interval  int
	weekly    []int // Mon..Sun indexes
	each      bool
	days      []int
	ordinal   int
	kind      dayKind
	weekday   eventkit.Weekday
	months    []int
	patternOn bool
	endMode   int
	end       time.Time
	count     int
}

// prefillFields decodes exactly one supported rule into editor values.
func prefillFields(rules []eventkit.RecurrenceRule) *prefill {
	if len(rules) != 1 || !representable(rules) {
		return nil
	}
	r := rules[0]
	p := prefill{interval: max(1, r.Interval), endMode: endNever}
	if r.End != nil {
		if r.End.EndDate != nil {
			p.endMode = endOnDate
			p.end = *r.End.EndDate
		} else if r.End.OccurrenceCount > 0 {
			p.endMode = endAfter
			p.count = r.End.OccurrenceCount
		}
	}
	p.frequency = r.Frequency
	switch r.Frequency {
	case eventkit.FrequencyDaily:
		return &p
	case eventkit.FrequencyWeekly:
		for _, d := range r.DaysOfTheWeek {
			idx := weekdayIndex(d.DayOfTheWeek)
			p.weekly = append(p.weekly, idx)
		}
		return &p
	case eventkit.FrequencyMonthly:
		if len(r.DaysOfTheMonth) > 0 {
			p.each = true
			p.days = append([]int(nil), r.DaysOfTheMonth...)
			return &p
		}
		o := onThePattern(r)
		if o == nil {
			return nil
		}
		p.each = false
		p.ordinal = o.ordinal
		p.kind = o.kind
		if len(o.weekdays) == 1 {
			p.weekday = o.weekdays[0]
		}
		return &p
	case eventkit.FrequencyYearly:
		p.months = append([]int(nil), r.MonthsOfTheYear...)
		if len(r.DaysOfTheWeek) == 1 {
			p.patternOn = true
			p.ordinal = r.DaysOfTheWeek[0].WeekNumber
			p.weekday = r.DaysOfTheWeek[0].DayOfTheWeek
		}
		return &p
	}
	return nil
}

func weekdayIndex(w eventkit.Weekday) int {
	if w == eventkit.Sunday {
		return 6
	}
	return int(w) - 2 // Monday(2)..Friday(6) -> 0..4; Saturday(7) -> 5
}

// Hide closes the editor without emitting anything.
func (m *RecurrenceModel) Hide() {
	m.visible = false
}

func (m RecurrenceModel) Visible() bool { return m.visible }

// SetSize accepts pane dimensions from the containing form; the whole
// terminal is never used here.
func (m *RecurrenceModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.endPicker.SetSize(width, height)
	if m.selector.Visible() {
		m.selector.SetSize(width, height)
	}
}

// draftRules composes the eventkit rule from the current draft.
func (m RecurrenceModel) draftRules() []eventkit.RecurrenceRule {
	rule := eventkit.RecurrenceRule{Frequency: m.frequency, Interval: m.interval()}
	switch m.frequency {
	case eventkit.FrequencyWeekly:
		rule.DaysOfTheWeek = nil
		for i, on := range m.weeklyDays {
			if on {
				wd := eventkit.Weekday(i + 2) // Monday=0 -> eventkit Monday(2)
				if i == 6 {
					wd = eventkit.Sunday
				}
				rule.DaysOfTheWeek = append(rule.DaysOfTheWeek,
					eventkit.RecurrenceDayOfWeek{DayOfTheWeek: wd, WeekNumber: 0})
			}
		}
	case eventkit.FrequencyMonthly:
		if m.monthlyMode == monthlyEach {
			var days []int
			for i, on := range m.monthlyDays {
				if on {
					if i == 31 {
						days = append(days, -1)
					} else {
						days = append(days, i+1)
					}
				}
			}
			rule.DaysOfTheMonth = sortedDayIDs(days)
		} else {
			rule.DaysOfTheWeek, rule.SetPositions = m.composeOnThe()
		}
	case eventkit.FrequencyYearly:
		for i, on := range m.yearlyMonths {
			if on {
				rule.MonthsOfTheYear = append(rule.MonthsOfTheYear, i+1)
			}
		}
		sort.Ints(rule.MonthsOfTheYear)
		if m.yearlyOn {
			rule.DaysOfTheWeek = []eventkit.RecurrenceDayOfWeek{
				{DayOfTheWeek: m.dayKindWd, WeekNumber: m.ordinal},
			}
		}
	}
	switch m.endMode {
	case endOnDate:
		t := endOfDayLocal(m.endPickerValue())
		rule.End = &eventkit.RecurrenceEnd{EndDate: &t}
	case endAfter:
		n, err := strconv.Atoi(strings.TrimSpace(m.endCount))
		if err == nil && n >= 1 {
			rule.End = &eventkit.RecurrenceEnd{OccurrenceCount: n}
		}
	}
	return []eventkit.RecurrenceRule{rule}
}

func (m RecurrenceModel) interval() int {
	n, err := strconv.Atoi(strings.TrimSpace(m.intervalIn.Value()))
	if err != nil || n < 1 {
		return 0
	}
	return n
}

// composeOnThe builds the DaysOfTheWeek/SetPositions pair for the monthly
// "On the" mode. A single weekday carries its ordinal in WeekNumber; grouped
// Day/Weekday/Weekend use WeekNumber 0 with all their days and
// SetPositions=[ordinal].
func (m RecurrenceModel) composeOnThe() ([]eventkit.RecurrenceDayOfWeek, []int) {
	switch m.dayKindV {
	case dayKindSpecific:
		return []eventkit.RecurrenceDayOfWeek{{DayOfTheWeek: m.dayKindWd, WeekNumber: m.ordinal}}, nil
	case dayKindDay:
		var days []eventkit.RecurrenceDayOfWeek
		for wd := eventkit.Sunday; wd <= eventkit.Saturday; wd++ {
			days = append(days, eventkit.RecurrenceDayOfWeek{DayOfTheWeek: wd, WeekNumber: 0})
		}
		return days, []int{m.ordinal}
	case dayKindWeekday:
		var days []eventkit.RecurrenceDayOfWeek
		for wd := eventkit.Monday; wd <= eventkit.Friday; wd++ {
			days = append(days, eventkit.RecurrenceDayOfWeek{DayOfTheWeek: wd, WeekNumber: 0})
		}
		return days, []int{m.ordinal}
	case dayKindWeekend:
		return []eventkit.RecurrenceDayOfWeek{
			{DayOfTheWeek: eventkit.Saturday, WeekNumber: 0},
			{DayOfTheWeek: eventkit.Sunday, WeekNumber: 0},
		}, []int{m.ordinal}
	}
	return nil, nil
}

// fields lists the active editor fields in tab order.
func (m RecurrenceModel) fields() []fieldKind {
	fs := []fieldKind{fFrequency, fInterval}
	switch m.frequency {
	case eventkit.FrequencyWeekly:
		fs = append(fs, fWeeklyDays)
	case eventkit.FrequencyMonthly:
		fs = append(fs, fMonthlyMode)
		if m.monthlyMode == monthlyEach {
			fs = append(fs, fMonthlyDays)
		} else {
			fs = append(fs, fOrdinal, fDayKind)
		}
	case eventkit.FrequencyYearly:
		fs = append(fs, fYearlyMonths, fYearlyPattern)
		if m.yearlyOn {
			fs = append(fs, fYearlyOrdinal, fYearlyWeekday)
		}
	}
	fs = append(fs, fEnd)
	if m.endMode != endNever {
		fs = append(fs, fEndValue)
	}
	return fs
}

func (m *RecurrenceModel) blurAll() {
	m.intervalIn.Blur()
	m.endPicker.Blur()
}

func (m *RecurrenceModel) focusField() {
	m.blurAll()
	fs := m.fields()
	if m.focusIndex >= len(fs) {
		m.focusIndex = len(fs) - 1
	}
	switch fs[m.focusIndex] {
	case fInterval:
		m.intervalIn.Focus()
	case fEndValue:
		if m.endMode == endOnDate {
			m.endPicker.Focus()
		}
	}
}

func (m *RecurrenceModel) openChooser(kind fieldKind) {
	m.purpose = purposeFor(kind)
	m.selector.Open(m.chooserOptions(kind), m.chooserSelected(kind), m.chooserMulti(kind))
}

func purposeFor(kind fieldKind) selPurpose {
	switch kind {
	case fFrequency:
		return spFrequency
	case fWeeklyDays:
		return spWeeklyDays
	case fMonthlyMode:
		return spMonthlyMode
	case fMonthlyDays:
		return spMonthlyDays
	case fOrdinal, fYearlyOrdinal:
		return spOrdinal
	case fDayKind:
		return spDayKind
	case fYearlyMonths:
		return spYearlyMonths
	case fYearlyPattern:
		return spYearlyPattern
	case fYearlyWeekday:
		return spYearlyWeekday
	case fEnd:
		return spEnd
	}
	return spNone
}

func (m RecurrenceModel) chooserMulti(kind fieldKind) bool {
	switch kind {
	case fWeeklyDays, fMonthlyDays, fYearlyMonths:
		return true
	}
	return false
}

func (m RecurrenceModel) chooserSelected(kind fieldKind) []string {
	switch kind {
	case fFrequency:
		return []string{frequencyID(m.frequency)}
	case fWeeklyDays:
		var sel []string
		for i, on := range m.weeklyDays {
			if on {
				sel = append(sel, strconv.Itoa(i))
			}
		}
		return sel
	case fMonthlyMode:
		if m.monthlyMode == monthlyEach {
			return []string{"each"}
		}
		return []string{"onthe"}
	case fMonthlyDays:
		var sel []string
		for i, on := range m.monthlyDays {
			if on {
				if i == 31 {
					sel = append(sel, "-1")
				} else {
					sel = append(sel, strconv.Itoa(i+1))
				}
			}
		}
		return sel
	case fOrdinal, fYearlyOrdinal:
		return []string{strconv.Itoa(m.ordinal)}
	case fDayKind:
		if m.dayKindV == dayKindSpecific {
			return []string{"wd" + strconv.Itoa(int(m.dayKindWd))}
		}
		return []string{strconv.Itoa(int(m.dayKindV))}
	case fYearlyMonths:
		var sel []string
		for i, on := range m.yearlyMonths {
			if on {
				sel = append(sel, strconv.Itoa(i+1))
			}
		}
		return sel
	case fYearlyPattern:
		if m.yearlyOn {
			return []string{"onthe"}
		}
		return []string{"sameday"}
	case fYearlyWeekday:
		return []string{"wd" + strconv.Itoa(int(m.dayKindWd))}
	case fEnd:
		return []string{strconv.Itoa(m.endMode)}
	}
	return nil
}

func frequencyID(f eventkit.RecurrenceFrequency) string {
	switch f {
	case eventkit.FrequencyWeekly:
		return "weekly"
	case eventkit.FrequencyMonthly:
		return "monthly"
	case eventkit.FrequencyYearly:
		return "yearly"
	default:
		return "daily"
	}
}

func (m RecurrenceModel) chooserOptions(kind fieldKind) []selectorOption {
	switch kind {
	case fFrequency:
		return []selectorOption{
			{ID: "daily", Label: "Daily"},
			{ID: "weekly", Label: "Weekly"},
			{ID: "monthly", Label: "Monthly"},
			{ID: "yearly", Label: "Yearly"},
		}
	case fWeeklyDays:
		return []selectorOption{
			{ID: "0", Label: "Monday"}, {ID: "1", Label: "Tuesday"}, {ID: "2", Label: "Wednesday"},
			{ID: "3", Label: "Thursday"}, {ID: "4", Label: "Friday"}, {ID: "5", Label: "Saturday"},
			{ID: "6", Label: "Sunday"},
		}
	case fMonthlyMode:
		return []selectorOption{
			{ID: "each", Label: "Each"},
			{ID: "onthe", Label: "On the"},
		}
	case fMonthlyDays:
		opts := make([]selectorOption, 0, 32)
		for d := 1; d <= 31; d++ {
			opts = append(opts, selectorOption{ID: strconv.Itoa(d), Label: strconv.Itoa(d)})
		}
		opts = append(opts, selectorOption{ID: "-1", Label: "Last day"})
		return opts
	case fOrdinal, fYearlyOrdinal:
		return []selectorOption{
			{ID: "1", Label: "First"}, {ID: "2", Label: "Second"}, {ID: "3", Label: "Third"},
			{ID: "4", Label: "Fourth"}, {ID: "5", Label: "Fifth"}, {ID: "-1", Label: "Last"},
		}
	case fDayKind:
		opts := []selectorOption{
			{ID: strconv.Itoa(int(dayKindDay)), Label: "Day"},
			{ID: strconv.Itoa(int(dayKindWeekday)), Label: "Weekday"},
			{ID: strconv.Itoa(int(dayKindWeekend)), Label: "Weekend day"},
		}
		specific := make([]selectorOption, 0, 7)
		for _, wd := range []eventkit.Weekday{eventkit.Monday, eventkit.Tuesday, eventkit.Wednesday,
			eventkit.Thursday, eventkit.Friday, eventkit.Saturday, eventkit.Sunday} {
			specific = append(specific, selectorOption{ID: "wd" + strconv.Itoa(int(wd)), Label: wd.String()})
		}
		return append(specific, opts...)
	case fYearlyMonths:
		months := []string{"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"}
		opts := make([]selectorOption, len(months))
		for i, name := range months {
			opts[i] = selectorOption{ID: strconv.Itoa(i + 1), Label: name}
		}
		return opts
	case fYearlyPattern:
		return []selectorOption{
			{ID: "sameday", Label: "Same day"},
			{ID: "onthe", Label: "On the"},
		}
	case fYearlyWeekday:
		opts := make([]selectorOption, 0, 7)
		for _, wd := range []eventkit.Weekday{eventkit.Monday, eventkit.Tuesday, eventkit.Wednesday,
			eventkit.Thursday, eventkit.Friday, eventkit.Saturday, eventkit.Sunday} {
			opts = append(opts, selectorOption{ID: "wd" + strconv.Itoa(int(wd)), Label: wd.String()})
		}
		return opts
	case fEnd:
		return []selectorOption{
			{ID: "0", Label: "Never"},
			{ID: "1", Label: "On date"},
			{ID: "2", Label: "After occurrences"},
		}
	}
	return nil
}

// applyChooserResult consumes a selector commit for the active purpose.
func (m *RecurrenceModel) applyChooserResult(ids []string) {
	if len(ids) == 0 {
		return
	}
	fs := m.fields()
	if m.focusIndex < len(fs) {
		switch fs[m.focusIndex] {
		case fFrequency:
			m.setFrequency(ids[0])
		case fWeeklyDays:
			for i := range m.weeklyDays {
				m.weeklyDays[i] = false
			}
			for _, id := range ids {
				idx, _ := strconv.Atoi(id)
				if idx >= 0 && idx < 7 {
					m.weeklyDays[idx] = true
				}
			}
		case fMonthlyMode:
			if ids[0] == "onthe" {
				m.setMonthlyMode(monthlyOnThe)
			} else {
				m.setMonthlyMode(monthlyEach)
			}
		case fMonthlyDays:
			for i := range m.monthlyDays {
				m.monthlyDays[i] = false
			}
			for _, id := range ids {
				d, _ := strconv.Atoi(id)
				if d == -1 {
					m.monthlyDays[31] = true
				} else if d >= 1 && d <= 31 {
					m.monthlyDays[d-1] = true
				}
			}
		case fOrdinal, fYearlyOrdinal:
			o, _ := strconv.Atoi(ids[0])
			if o == -1 || (o >= 1 && o <= 5) {
				m.ordinal = o
			}
		case fDayKind:
			if strings.HasPrefix(ids[0], "wd") {
				m.dayKindV = dayKindSpecific
				wd, _ := strconv.Atoi(strings.TrimPrefix(ids[0], "wd"))
				m.dayKindWd = eventkit.Weekday(wd)
			} else {
				k, _ := strconv.Atoi(ids[0])
				m.dayKindV = dayKind(k)
			}
		case fYearlyMonths:
			for i := range m.yearlyMonths {
				m.yearlyMonths[i] = false
			}
			for _, id := range ids {
				mo, _ := strconv.Atoi(id)
				if mo >= 1 && mo <= 12 {
					m.yearlyMonths[mo-1] = true
				}
			}
		case fYearlyPattern:
			m.yearlyOn = ids[0] == "onthe"
			if m.yearlyOn {
				anchor := m.anchor()
				m.ordinal = (anchor.Day()-1)/7 + 1
				m.dayKindWd = weekdayFromGo(anchor.Weekday())
			}
		case fYearlyWeekday:
			wd, _ := strconv.Atoi(strings.TrimPrefix(ids[0], "wd"))
			m.dayKindWd = eventkit.Weekday(wd)
		case fEnd:
			mode, _ := strconv.Atoi(ids[0])
			m.setEndMode(mode)
		}
	}
}

// setFrequency changes frequency, retaining interval and end while
// re-initializing frequency-specific controls from the anchor.
func (m *RecurrenceModel) setFrequency(id string) {
	var f eventkit.RecurrenceFrequency
	switch id {
	case "weekly":
		f = eventkit.FrequencyWeekly
	case "monthly":
		f = eventkit.FrequencyMonthly
	case "yearly":
		f = eventkit.FrequencyYearly
	default:
		f = eventkit.FrequencyDaily
	}
	if f != m.frequency {
		m.frequency = f
		anchor := m.anchor()
		if f == eventkit.FrequencyMonthly {
			m.monthlyMode = monthlyEach
			for i := range m.monthlyDays {
				m.monthlyDays[i] = false
			}
			day := min(anchor.Day(), 31)
			m.monthlyDays[day-1] = true
			m.ordinal = (day-1)/7 + 1
			m.dayKindWd = weekdayFromGo(anchor.Weekday())
		}
		if f == eventkit.FrequencyWeekly {
			for i := range m.weeklyDays {
				m.weeklyDays[i] = false
			}
			m.initFrequencySpecific(anchor)
		}
		if f == eventkit.FrequencyYearly {
			for i := range m.yearlyMonths {
				m.yearlyMonths[i] = false
			}
			m.yearlyMonths[int(anchor.Month())-1] = true
			m.yearlyOn = false
		}
	}
}

func (m *RecurrenceModel) setMonthlyMode(mode int) {
	if m.monthlyMode == mode {
		return
	}
	m.monthlyMode = mode
	if mode == monthlyOnThe {
		anchor := m.anchor()
		m.ordinal = (anchor.Day()-1)/7 + 1
		m.dayKindWd = weekdayFromGo(anchor.Weekday())
	} else {
		for i := range m.monthlyDays {
			m.monthlyDays[i] = false
		}
		day := min(m.anchor().Day(), 31)
		m.monthlyDays[day-1] = true
	}
	m.focusField()
}

func (m *RecurrenceModel) setEndMode(mode int) {
	if m.endMode == mode {
		return
	}
	m.endMode = mode
	// On date / After occurrences require explicit entry: no defaults.
	m.endPicker.SetValue("")
	m.endCount = ""
	m.endDateOn = false
	for i, f := range m.fields() {
		if f == fEndValue {
			m.focusIndex = i
			break
		}
	}
	m.focusField()
}

func (m RecurrenceModel) endPickerValue() time.Time {
	t, err := m.endPicker.Resolve()
	if err != nil || t == nil {
		return time.Time{}
	}
	return *t
}

// apply validates the draft and emits RecurrenceSubmitMsg. The pointer
// receiver keeps validation error and focus changes visible in the pane.
func (m *RecurrenceModel) apply() tea.Cmd {
	if n := m.interval(); n < 1 {
		m.errText = "Interval must be at least 1"
		m.focusField()
		return nil
	}
	switch m.frequency {
	case eventkit.FrequencyWeekly:
		if !anyTrue(m.weeklyDays) {
			m.errText = "Choose at least one weekday"
			return nil
		}
	case eventkit.FrequencyMonthly:
		if m.monthlyMode == monthlyEach && !anyTrue(m.monthlyDays) {
			m.errText = "Choose at least one day of the month"
			return nil
		}
	case eventkit.FrequencyYearly:
		if !anyTrue(m.yearlyMonths) {
			m.errText = "Choose at least one month"
			return nil
		}
	}
	switch m.endMode {
	case endOnDate:
		t, err := m.endPicker.Resolve()
		if err != nil || t == nil {
			m.errText = "Enter an end date"
			return nil
		}
		if !m.base.IsZero() && t.Before(dateOnly(m.base)) {
			m.errText = "End date cannot be before the due date"
			return nil
		}
	case endAfter:
		n, err := strconv.Atoi(strings.TrimSpace(m.endCount))
		if err != nil || n < 1 {
			m.errText = "Occurrences must be at least 1"
			return nil
		}
	}
	rules := m.draftRules()
	m.errText = ""
	m.visible = false
	return func() tea.Msg { return RecurrenceSubmitMsg{Rules: rules} }
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func anyTrue(v []bool) bool {
	for _, b := range v {
		if b {
			return true
		}
	}
	return false
}

// Update routes keys: the inner chooser first, then the editor.
func (m RecurrenceModel) Update(msg tea.Msg) (RecurrenceModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	// Selector results.
	switch msg := msg.(type) {
	case selectorSelectedMsg:
		m.purpose = spNone
		m.selector.Close()
		m.applyChooserResult(msg.IDs)
		return m, nil
	case selectorCancelledMsg:
		m.purpose = spNone
		m.selector.Close()
		return m, nil
	}

	if m.selector.Visible() {
		var cmd tea.Cmd
		m.selector, cmd = m.selector.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		fs := m.fields()
		if m.focusIndex < len(fs) && isEditorChoiceField(fs[m.focusIndex]) &&
			key.Matches(msg, m.keys.Bind("choice_field", "open")) {
			m.openChooser(fs[m.focusIndex])
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Bind("dialog", "cancel")):
			m.visible = false
			return m, func() tea.Msg { return RecurrenceCancelMsg{} }
		case key.Matches(msg, m.keys.Bind("dialog", "submit")):
			cmd := m.apply()
			if cmd != nil {
				m.visible = false
				m.errText = ""
			}
			return m, cmd
		case key.Matches(msg, m.keys.Bind("dialog", "next_field")):
			m.focusIndex = (m.focusIndex + 1) % len(m.fields())
			m.focusField()
			return m, nil
		case key.Matches(msg, m.keys.Bind("dialog", "previous_field")):
			m.focusIndex = mod(m.focusIndex-1, len(m.fields()))
			m.focusField()
			return m, nil
		case key.Matches(msg, m.keys.Bind("choice_field", "open")):
			// Enter on numeric/date fields applies a valid draft.
			if m.focusIndex < len(fs) {
				cmd := m.apply()
				if cmd != nil {
					m.visible = false
					m.errText = ""
				}
				return m, cmd
			}
			return m, nil
		}

		// End-date picker gets calendar navigation and typing.
		if m.focusIndex < len(fs) && fs[m.focusIndex] == fEndValue && m.endMode == endOnDate {
			var cmd tea.Cmd
			m.endPicker, cmd = m.endPicker.Update(msg)
			return m, cmd
		}
		// Typing on interval/count fields.
		if m.focusIndex < len(fs) {
			switch fs[m.focusIndex] {
			case fInterval:
				var cmd tea.Cmd
				m.intervalIn, cmd = m.intervalIn.Update(msg)
				return m, cmd
			case fEndValue:
				if m.endMode == endAfter {
					m.editEndCount(msg)
					return m, nil
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func mod(n, k int) int {
	return ((n % k) + k) % k
}

// editEndCount edits the occurrence-count input as plain digits.
func (m *RecurrenceModel) editEndCount(msg tea.KeyPressMsg) {
	if msg.Code == tea.KeyBackspace {
		r := []rune(m.endCount)
		if len(r) > 0 {
			m.endCount = string(r[:len(r)-1])
		}
		return
	}
	if len(msg.Text) > 0 {
		for _, r := range msg.Text {
			if r < '0' || r > '9' {
				return
			}
		}
		if len(m.endCount) < 9 {
			m.endCount += msg.Text
		}
	}
}

func isEditorChoiceField(kind fieldKind) bool {
	switch kind {
	case fFrequency, fWeeklyDays, fMonthlyMode, fMonthlyDays, fOrdinal,
		fDayKind, fYearlyMonths, fYearlyPattern, fYearlyOrdinal, fYearlyWeekday, fEnd:
		return true
	}
	return false
}

// View renders unframed pane content for the containing form's right pane.
// When the inner chooser is open its content fills the same pane.
func (m RecurrenceModel) View() string {
	if !m.visible {
		return ""
	}
	if m.selector.Visible() {
		return m.selector.View()
	}
	title := "Custom Repeat"
	if m.banner != "" {
		title += " — " + m.banner
	}
	var b strings.Builder
	b.WriteString(styles.DialogTitleStyle.Render(title))
	b.WriteString("\n\n")

	fs := m.fields()
	for i, f := range fs {
		prompt := fieldPrompt(f)
		value := m.fieldValue(f)
		line := prompt + value
		if i == m.focusIndex {
			line = lipgloss.NewStyle().Foreground(styles.Teal).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		if f == fEndValue && m.endMode == endOnDate && i == m.focusIndex {
			b.WriteString(m.endPicker.View())
			b.WriteString("\n")
		}
		if m.errText != "" && i == m.focusIndex {
			b.WriteString(lipgloss.NewStyle().Foreground(styles.Red).Render(m.errText))
			b.WriteString("\n")
		}
	}
	footer := keybind.ShortKeys(m.keys.Aliases("dialog", "submit")) + ": apply  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "next_field")) + ": next  " +
		keybind.ShortKeys(m.keys.Aliases("dialog", "cancel")) + ": cancel"
	b.WriteString(lipgloss.NewStyle().Foreground(styles.DimGray).Render(footer))
	return b.String()
}

// FocusLine returns the zero-based line of the active field (with its error)
// within the unframed View output.
func (m RecurrenceModel) FocusLine() int {
	if !m.visible || m.selector.Visible() {
		return 0
	}
	line := 2 // title + blank
	fs := m.fields()
	for i, f := range fs {
		if i == m.focusIndex {
			return line
		}
		line++
		if f == fEndValue && m.endMode == endOnDate && i == m.focusIndex {
			line += strings.Count(m.endPicker.View(), "\n") + 1
		}
	}
	return 0
}

func fieldPrompt(f fieldKind) string {
	switch f {
	case fFrequency:
		return "Frequency: "
	case fInterval:
		return "Interval:  "
	case fWeeklyDays:
		return "Days:      "
	case fMonthlyMode:
		return "Mode:      "
	case fMonthlyDays:
		return "Days:      "
	case fOrdinal, fYearlyOrdinal:
		return "Ordinal:   "
	case fDayKind:
		return "On:        "
	case fYearlyMonths:
		return "Months:    "
	case fYearlyPattern:
		return "Pattern:   "
	case fYearlyWeekday:
		return "Weekday:   "
	case fEnd:
		return "End:       "
	case fEndValue:
		return "Value:     "
	}
	return ""
}

func (m RecurrenceModel) fieldValue(f fieldKind) string {
	switch f {
	case fFrequency:
		return frequencyLabel(m.frequency)
	case fInterval:
		return m.intervalIn.View()
	case fWeeklyDays:
		return boolSetLabel(m.weeklyDays, weekdayNames)
	case fMonthlyMode:
		if m.monthlyMode == monthlyEach {
			return "Each"
		}
		return "On the"
	case fMonthlyDays:
		var vals []string
		for i, on := range m.monthlyDays {
			if on {
				if i == 31 {
					vals = append(vals, "Last")
				} else {
					vals = append(vals, strconv.Itoa(i+1))
				}
			}
		}
		return strings.Join(vals, ", ")
	case fOrdinal, fYearlyOrdinal:
		return ordinalLabel(m.ordinal)
	case fDayKind:
		if m.dayKindV == dayKindSpecific {
			return m.dayKindWd.String()
		}
		switch m.dayKindV {
		case dayKindDay:
			return "Day"
		case dayKindWeekday:
			return "Weekday"
		default:
			return "Weekend day"
		}
	case fYearlyMonths:
		var vals []string
		for i, on := range m.yearlyMonths {
			if on {
				vals = append(vals, monthNames[i])
			}
		}
		return strings.Join(vals, ", ")
	case fYearlyPattern:
		if m.yearlyOn {
			return "On the"
		}
		return "Same day"
	case fYearlyWeekday:
		return m.dayKindWd.String()
	case fEnd:
		switch m.endMode {
		case endOnDate:
			return "On date"
		case endAfter:
			return "After occurrences"
		default:
			return "Never"
		}
	case fEndValue:
		if m.endMode == endOnDate {
			return ""
		}
		return strings.TrimSpace(m.endCount)
	}
	return ""
}

var weekdayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
var monthNames = []string{"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

func boolSetLabel(flags []bool, names []string) string {
	var vals []string
	for i, on := range flags {
		if on && i < len(names) {
			vals = append(vals, names[i])
		}
	}
	return strings.Join(vals, ", ")
}

func frequencyLabel(f eventkit.RecurrenceFrequency) string {
	switch f {
	case eventkit.FrequencyWeekly:
		return "Weekly"
	case eventkit.FrequencyMonthly:
		return "Monthly"
	case eventkit.FrequencyYearly:
		return "Yearly"
	default:
		return "Daily"
	}
}

func ordinalLabel(o int) string {
	switch o {
	case 1:
		return "First"
	case 2:
		return "Second"
	case 3:
		return "Third"
	case 4:
		return "Fourth"
	case 5:
		return "Fifth"
	case -1:
		return "Last"
	}
	return strconv.Itoa(o)
}

// Priority label mapping reused by the outer form.
func priorityLabel(p int) string {
	switch p {
	case reminders.PriorityHigh:
		return "High"
	case reminders.PriorityMedium:
		return "Medium"
	case reminders.PriorityLow:
		return "Low"
	default:
		return "None"
	}
}
