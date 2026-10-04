package dialog

import (
	"sort"
	"time"

	"github.com/BRO3886/go-eventkit"
)

// recurrence presets, in the plan's display order.
type presetDef struct {
	id    string
	label string
	build func() []eventkit.RecurrenceRule
}

var presets = []presetDef{
	{"none", "None", func() []eventkit.RecurrenceRule { return nil }},
	{"daily", "Daily", func() []eventkit.RecurrenceRule { return []eventkit.RecurrenceRule{eventkit.Daily(1)} }},
	{"weekdays", "Weekdays", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Weekly(1, eventkit.Monday, eventkit.Tuesday,
			eventkit.Wednesday, eventkit.Thursday, eventkit.Friday)}
	}},
	{"weekends", "Weekends", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Weekly(1, eventkit.Saturday, eventkit.Sunday)}
	}},
	{"weekly", "Weekly", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Weekly(1)}
	}},
	{"biweekly", "Every 2 weeks", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Weekly(2)}
	}},
	{"monthly", "Monthly", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Monthly(1)}
	}},
	{"quarterly", "Every 3 months", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Monthly(3)}
	}},
	{"semiannual", "Every 6 months", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Monthly(6)}
	}},
	{"yearly", "Yearly", func() []eventkit.RecurrenceRule {
		return []eventkit.RecurrenceRule{eventkit.Yearly(1)}
	}},
}

// presetOptions returns the fuzzy chooser options for the recurrence field.
func presetOptions() []selectorOption {
	opts := make([]selectorOption, 0, len(presets)+1)
	for _, p := range presets {
		opts = append(opts, selectorOption{ID: p.id, Label: p.label})
	}
	opts = append(opts, selectorOption{ID: "custom", Label: "Custom…"})
	return opts
}

// matchPreset reports which preset a rule list equals exactly, or "".
// Unconstrained weekly/monthly/yearly rules (no day/month constraints and no
// end) match their preset so they inherit the due date's day/month.
func matchPreset(rules []eventkit.RecurrenceRule) string {
	if len(rules) == 0 {
		return "none"
	}
	if len(rules) != 1 {
		return ""
	}
	r := rules[0]
	clean := r
	clean.End = nil
	simple := len(clean.DaysOfTheWeek) == 0 && len(clean.DaysOfTheMonth) == 0 &&
		len(clean.MonthsOfTheYear) == 0 && len(clean.SetPositions) == 0
	if simple {
		switch {
		case r.Frequency == eventkit.FrequencyDaily && r.Interval == 1:
			return "daily"
		case r.Frequency == eventkit.FrequencyWeekly && r.Interval == 1:
			return "weekly"
		case r.Frequency == eventkit.FrequencyWeekly && r.Interval == 2:
			return "biweekly"
		case r.Frequency == eventkit.FrequencyMonthly && r.Interval == 1:
			return "monthly"
		case r.Frequency == eventkit.FrequencyMonthly && r.Interval == 3:
			return "quarterly"
		case r.Frequency == eventkit.FrequencyMonthly && r.Interval == 6:
			return "semiannual"
		case r.Frequency == eventkit.FrequencyYearly && r.Interval == 1:
			return "yearly"
		}
	}
	if r.End == nil && r.Frequency == eventkit.FrequencyWeekly && r.Interval == 1 && r.SetPositions == nil {
		if weekdaySetMatches(clean.DaysOfTheWeek, []eventkit.Weekday{
			eventkit.Monday, eventkit.Tuesday, eventkit.Wednesday, eventkit.Thursday, eventkit.Friday}) {
			return "weekdays"
		}
		if weekdaySetMatches(clean.DaysOfTheWeek, []eventkit.Weekday{eventkit.Saturday, eventkit.Sunday}) {
			return "weekends"
		}
	}
	return ""
}

// weekdaySetMatches checks an unconstrained (WeekNumber 0) day set equality.
func weekdaySetMatches(got []eventkit.RecurrenceDayOfWeek, want []eventkit.Weekday) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[eventkit.Weekday]bool{}
	for _, d := range got {
		if d.WeekNumber != 0 {
			return false
		}
		set[d.DayOfTheWeek] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// representable reports whether exactly one rule can be faithfully edited by
// the custom editor's controls.
func representable(rules []eventkit.RecurrenceRule) bool {
	if len(rules) != 1 {
		return false
	}
	r := rules[0]
	switch r.Frequency {
	case eventkit.FrequencyDaily:
		return len(r.DaysOfTheWeek) == 0 && len(r.DaysOfTheMonth) == 0 &&
			len(r.MonthsOfTheYear) == 0 && len(r.SetPositions) == 0
	case eventkit.FrequencyWeekly:
		return len(r.DaysOfTheMonth) == 0 && len(r.MonthsOfTheYear) == 0 && len(r.SetPositions) == 0
	case eventkit.FrequencyMonthly:
		if len(r.MonthsOfTheYear) != 0 {
			return false
		}
		if len(r.DaysOfTheMonth) != 0 {
			return len(r.DaysOfTheWeek) == 0 && len(r.SetPositions) == 0
		}
		if len(r.DaysOfTheWeek) != 0 {
			return onThePattern(r) != nil
		}
		return true
	case eventkit.FrequencyYearly:
		if len(r.DaysOfTheMonth) != 0 || len(r.SetPositions) != 0 {
			return false
		}
		if len(r.MonthsOfTheYear) == 0 {
			return false
		}
		if len(r.DaysOfTheWeek) == 0 {
			return true
		}
		return len(r.DaysOfTheWeek) == 1 && r.DaysOfTheWeek[0].WeekNumber != 0
	default:
		return false
	}
}

// onThePattern decodes a monthly rule whose only constraint is
// DaysOfTheWeek, returning (kind, ordinal, weekdays) or nil when mixed
// week numbers make it unrepresentable.
func onThePattern(r eventkit.RecurrenceRule) *onThe {
	var kinds []eventkit.Weekday
	var wn int
	for i, d := range r.DaysOfTheWeek {
		if i == 0 {
			wn = d.WeekNumber
		}
		if d.WeekNumber != wn {
			return nil
		}
		kinds = append(kinds, d.DayOfTheWeek)
	}
	ordinal := wn
	if len(r.SetPositions) == 1 {
		ordinal = r.SetPositions[0]
	}
	o := onThe{ordinal: ordinal, weekdays: kinds}
	switch {
	case wn != 0 && len(kinds) == 1:
		return &o // nth weekday
	case wn == 0 && len(r.SetPositions) == 1 && len(kinds) == 7:
		o.kind = dayKindDay
		return &o
	case wn == 0 && len(r.SetPositions) == 1 && weekdaySetMatches(r.DaysOfTheWeek, []eventkit.Weekday{
		eventkit.Monday, eventkit.Tuesday, eventkit.Wednesday, eventkit.Thursday, eventkit.Friday}):
		o.kind = dayKindWeekday
		return &o
	case wn == 0 && len(r.SetPositions) == 1 && weekdaySetMatches(r.DaysOfTheWeek, []eventkit.Weekday{
		eventkit.Saturday, eventkit.Sunday}):
		o.kind = dayKindWeekend
		return &o
	default:
		return nil
	}
}

type onThe struct {
	kind     dayKind // 0 = specific weekday
	ordinal  int
	weekdays []eventkit.Weekday
}

type dayKind int

const (
	dayKindSpecific dayKind = iota
	dayKindDay
	dayKindWeekday
	dayKindWeekend
)

// rulesEqual compares two rule lists semantically: nil and empty constraint
// slices are equivalent and day/month sets are order-independent.
func rulesEqual(a, b []eventkit.RecurrenceRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !ruleEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func ruleEqual(a, b eventkit.RecurrenceRule) bool {
	if a.Frequency != b.Frequency || a.Interval != b.Interval {
		return false
	}
	if !sameIntSet(a.DaysOfTheMonth, b.DaysOfTheMonth) ||
		!sameIntSet(a.MonthsOfTheYear, b.MonthsOfTheYear) ||
		!sameIntSet(a.SetPositions, b.SetPositions) {
		return false
	}
	if len(a.DaysOfTheWeek) != len(b.DaysOfTheWeek) {
		return false
	}
	mapDOW := func(d []eventkit.RecurrenceDayOfWeek) map[[2]int]bool {
		m := map[[2]int]bool{}
		for _, x := range d {
			m[[2]int{int(x.DayOfTheWeek), x.WeekNumber}] = true
		}
		return m
	}
	ma, mb := mapDOW(a.DaysOfTheWeek), mapDOW(b.DaysOfTheWeek)
	for k := range ma {
		if !mb[k] {
			return false
		}
	}
	if (a.End == nil) != (b.End == nil) {
		return false
	}
	if a.End != nil {
		if a.End.OccurrenceCount != b.End.OccurrenceCount {
			return false
		}
		if (a.End.EndDate == nil) != (b.End.EndDate == nil) {
			return false
		}
		if a.End.EndDate != nil && !a.End.EndDate.Equal(*b.End.EndDate) {
			return false
		}
	}
	return true
}

func sameIntSet(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[int]int{}
	for _, x := range a {
		set[x]++
	}
	for _, x := range b {
		set[x]--
	}
	for _, c := range set {
		if c != 0 {
			return false
		}
	}
	return true
}

// copyRules deep-copies a rule list including End pointers.
func copyRules(rules []eventkit.RecurrenceRule) []eventkit.RecurrenceRule {
	if rules == nil {
		return nil
	}
	out := make([]eventkit.RecurrenceRule, len(rules))
	for i, r := range rules {
		r.DaysOfTheWeek = append([]eventkit.RecurrenceDayOfWeek(nil), r.DaysOfTheWeek...)
		r.DaysOfTheMonth = append([]int(nil), r.DaysOfTheMonth...)
		r.MonthsOfTheYear = append([]int(nil), r.MonthsOfTheYear...)
		r.SetPositions = append([]int(nil), r.SetPositions...)
		if r.End != nil {
			e := *r.End
			if e.EndDate != nil {
				d := *e.EndDate
				e.EndDate = &d
			}
			r.End = &e
		}
		out[i] = r
	}
	return out
}

// sortedDayIDs sorts day ids (1..31 and -1) ascending with -1 last.
func sortedDayIDs(ids []int) []int {
	out := append([]int(nil), ids...)
	sort.Slice(out, func(i, j int) bool {
		if out[i] == -1 {
			return false
		}
		if out[j] == -1 {
			return true
		}
		return out[i] < out[j]
	})
	return out
}

// endOfDayLocal stores a local end-of-day timestamp on the chosen date.
func endOfDayLocal(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}
