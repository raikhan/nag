// Package dateentry parses Org-style date expressions for due-date entry.
// Parse is pure: now is the captured wall clock and base is the editing
// anchor (the original due timestamp when editing, otherwise today at 09:00).
package dateentry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	offsetRe     = regexp.MustCompile(`^([+-])([+-]?)(\d+)(h|d|w|m|y)?$`)
	nthWeekdayRe = regexp.MustCompile(`^([+-])([+-]?)(\d+)(sun|mon|tue|wed|thu|fri|sat)$`)
	isoWeekRe    = regexp.MustCompile(`^(w\d{1,2}|\d{4}[ -]?w\d{1,2}(?:[ -]?[1-7])?|\d{4}-w\d{2}-[1-7])$`)
	ymdRe        = regexp.MustCompile(`^(\d{4})(?:[-/.](\d{1,2})(?:[-/.](\d{1,2}))?)?$`)
	amdRe        = regexp.MustCompile(`^(\d{1,2})[-/.](\d{1,2})(?:[-/.](\d{2}|\d{4}))?$`)
	timeRe       = regexp.MustCompile(`^(.*?)[ T](\d{1,2}):(\d{2})$`)
	dayRe        = regexp.MustCompile(`^(\d{1,2})$`)
	mdRe         = regexp.MustCompile(`^(\d{1,2})[-/.](\d{1,2})$`)
)

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

var months = map[string]time.Month{
	"january": time.January, "jan": time.January,
	"february": time.February, "feb": time.February,
	"march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"may":  time.May,
	"june": time.June, "jun": time.June,
	"july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August,
	"september": time.September, "sep": time.September, "sept": time.September,
	"october": time.October, "oct": time.October,
	"november": time.November, "nov": time.November,
	"december": time.December, "dec": time.December,
}

var singletons = map[string]int{ // offset in days from today
	"today": 0, "to": 0, "tod": 0, ".": 0,
	"tomorrow": 1, "tom": 1, "tomo": 1,
	"yesterday": -1, "yes": -1,
}

// Parse resolves one due timestamp from an Org-style expression. An empty
// expression means "no due date" (nil, nil). Invalid nonempty expressions
// return an error, never nil.
func Parse(raw string, now, base time.Time) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	lower := strings.ToLower(s)

	if strings.Contains(lower, "--") && !offsetRe.MatchString(lower) {
		return nil, fmt.Errorf("date ranges are not supported: %q", s)
	}

	// Optional trailing 24-hour time.
	hour, minute := -1, -1
	if m := timeRe.FindStringSubmatch(lower); m != nil && strings.TrimSpace(m[1]) != "" {
		h, _ := strconv.Atoi(m[2])
		mi, _ := strconv.Atoi(m[3])
		if h > 23 || mi > 59 {
			return nil, fmt.Errorf("invalid time %q", s)
		}
		hour, minute = h, mi
		lower = strings.TrimSpace(m[1])
	}

	t, err := parseDate(lower, now, base)
	if err != nil {
		return nil, err
	}
	if hour >= 0 {
		t = time.Date(t.Year(), t.Month(), t.Day(), hour, minute, 0, 0, t.Location())
	}
	if t.Year() < 1 || t.Year() > 9999 {
		return nil, fmt.Errorf("date outside years 1..9999: %q", s)
	}
	return &t, nil
}

func parseDate(s string, now, base time.Time) (time.Time, error) {
	clockHour, clockMin, _ := base.Clock() // new forms: today at 09:00; edits: original time
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, clockHour, clockMin, 0, 0, base.Location())
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), clockHour, clockMin, 0, 0, base.Location())

	if n, ok := singletons[s]; ok {
		return day(today.Year(), today.Month(), today.Day()+n), nil
	}

	if wd, ok := weekdays[s]; ok {
		return day(today.Year(), today.Month(), today.Day()+untilWeekday(today.Weekday(), wd, false)), nil
	}
	if m := regexp.MustCompile(`^next\s+([a-z]+)$`).FindStringSubmatch(s); m != nil {
		if wd, ok := weekdays[m[1]]; ok {
			return day(today.Year(), today.Month(), today.Day()+untilWeekday(today.Weekday(), wd, true)), nil
		}
	}

	switch s {
	case "eow": // Friday on or after today
		return day(today.Year(), today.Month(), today.Day()+untilWeekday(today.Weekday(), time.Friday, false)), nil
	case "eom":
		last := lastDay(now.Year(), now.Month())
		return day(now.Year(), now.Month(), last), nil
	}

	if m := offsetRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[3])
		if m[1] == "-" {
			n = -n
		}
		single := m[2] == ""
		var t time.Time
		if m[4] == "h" {
			anchor := now
			if !single {
				anchor = base
			}
			t = anchor.Add(time.Duration(n) * time.Hour)
		} else {
			anchor := today
			if !single {
				anchor = base
			}
			y, mo, dd := anchor.Year(), anchor.Month(), anchor.Day()
			switch m[4] {
			case "", "d":
				dd += n
			case "w":
				dd += n * 7
			case "m":
				y, mo, dd = addMonthsClamped(y, mo, anchor.Day(), n)
			case "y":
				y, mo, dd = addMonthsClamped(y, mo, anchor.Day(), n*12)
			}
			t = day(y, mo, dd)
		}
		return t, nil
	}

	if m := nthWeekdayRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[3])
		if n < 1 {
			return time.Time{}, fmt.Errorf("invalid weekday offset %q", s)
		}
		single := m[2] == ""
		anchor := today
		if !single {
			anchor = base
		}
		wd, ok := weekdays[m[4]]
		if !ok {
			return time.Time{}, fmt.Errorf("invalid date %q", s)
		}
		var dd int
		if m[1] == "+" {
			if anchor.Weekday() == wd {
				dd = (n - 1) * 7
			} else {
				dd = untilWeekday(anchor.Weekday(), wd, false) + (n-1)*7
			}
		} else {
			if anchor.Weekday() == wd {
				dd = -(n - 1) * 7
			} else {
				d := int(anchor.Weekday() - wd)
				if d < 0 {
					d += 7
				}
				dd = -(d + (n-1)*7)
			}
		}
		return day(anchor.Year(), anchor.Month(), anchor.Day()+dd), nil
	}

	if t, ok, err := parseISOWeek(s, base, day); ok {
		return t, err
	}

	if t, ok, err := parseYMD(s, base, day); ok {
		return t, err
	}

	if t, ok, err := parseMonthName(s, base, day); ok {
		return t, err
	}

	if m := amdRe.FindStringSubmatch(s); m != nil {
		mo, _ := strconv.Atoi(m[1])
		dd, _ := strconv.Atoi(m[2])
		if m[3] != "" {
			y := expandYear(m[3])
			if !dayInMonth(y, mo, dd) {
				return time.Time{}, fmt.Errorf("invalid date %q", s)
			}
			return day(y, time.Month(mo), dd), nil
		}
		t, ok, err := inferComplete(base.Year(), mo, dd, base, day)
		if err != nil || !ok {
			return time.Time{}, fmt.Errorf("invalid date %q", s)
		}
		return t, nil
	}

	if m := mdRe.FindStringSubmatch(s); m != nil {
		t, _, _ := inferComplete(base.Year(), atoi(m[1]), atoi(m[2]), base, day)
		return t, nil
	}

	if m := dayRe.FindStringSubmatch(s); m != nil {
		return inferDay(atoi(m[1]), base, day)
	}

	return time.Time{}, fmt.Errorf("invalid date %q", strings.TrimSpace(s))
}

// inferComplete resolves an explicit month and day with a possibly missing
// year, advancing the inferred year until the first valid occurrence on or
// after the base date. Explicit impossible dates are errors.
func inferComplete(y, mo, dd int, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, bool, error) {
	if mo < 1 || mo > 12 || dd < 1 || dd > 31 {
		return time.Time{}, false, nil
	}
	if dayInMonth(y, mo, dd) {
		t := day(y, time.Month(mo), dd)
		if sameDay(t, base) || t.After(base) {
			return t, true, nil
		}
	}
	for i := 1; i <= 100; i++ {
		y++
		if dayInMonth(y, mo, dd) {
			return day(y, time.Month(mo), dd), true, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("invalid date")
}

// inferDay resolves a bare day-of-month, advancing the inferred month until
// the first valid occurrence on or after the base date.
func inferDay(dd int, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, error) {
	if dd < 1 || dd > 31 {
		return time.Time{}, fmt.Errorf("invalid day %d", dd)
	}
	y, mo := base.Year(), base.Month()
	for range 2400 {
		if dayInMonth(y, int(mo), dd) {
			t := day(y, mo, dd)
			if !t.Before(base) {
				return t, nil
			}
		}
		y, mo = nextMonth(y, mo)
	}
	return time.Time{}, fmt.Errorf("invalid day %d", dd)
}

// parseMonthName handles "sep 15 [2027]", "22 sept 2027" and bare months
// (day from base, clamped; month/year advanced forward).
func parseMonthName(s string, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, bool, error) {
	// <day> <month> [year]
	var dayFirstRe = regexp.MustCompile(`^(\d{1,2}) ([a-z]+)(?: (\d{2}|\d{4}))?$`)
	// <month> [day [year]]
	var monthFirstRe = regexp.MustCompile(`^([a-z]+)(?: (\d{1,2}))?(?: (\d{2}|\d{4}))?$`)

	if m := dayFirstRe.FindStringSubmatch(s); m != nil {
		name := m[2]
		mo, ok := months[name]
		if !ok {
			return time.Time{}, false, nil
		}
		dd := atoi(m[1])
		y := base.Year()
		if m[3] != "" {
			y = expandYear(m[3])
			if !dayInMonth(y, int(mo), dd) {
				return time.Time{}, true, fmt.Errorf("invalid date %q", s)
			}
			return day(y, mo, dd), true, nil
		}
		t, ok, err := inferComplete(y, int(mo), dd, base, day)
		if err != nil || !ok {
			return time.Time{}, true, fmt.Errorf("invalid date %q", s)
		}
		return t, true, nil
	}

	if m := monthFirstRe.FindStringSubmatch(s); m != nil {
		mo, ok := months[m[1]]
		if !ok {
			return time.Time{}, false, nil
		}
		dd := base.Day()
		if m[2] != "" {
			dd = atoi(m[2])
		}
		y := base.Year()
		if m[3] != "" { // explicit year: exact date
			y := expandYear(m[3])
			if !dayInMonth(y, int(mo), dd) {
				return time.Time{}, true, fmt.Errorf("invalid date %q", s)
			}
			return day(y, mo, dd), true, nil
		}
		if m[2] != "" { // explicit month and day, inferred year
			t, ok, err := inferComplete(y, int(mo), dd, base, day)
			if err != nil || !ok {
				return time.Time{}, true, fmt.Errorf("invalid date %q", s)
			}
			return t, true, nil
		}
		// Bare month: base day clamped, advance to first valid on/after base.
		if dd > 31 {
			return time.Time{}, true, fmt.Errorf("invalid date %q", s)
		}
		yy, mm := y, mo
		for range 2400 {
			clamped := min(dd, lastDay(yy, mm))
			t := day(yy, mm, clamped)
			if !t.Before(base) {
				return t, true, nil
			}
			yy, mm = nextMonth(yy, mm)
		}
		return time.Time{}, true, fmt.Errorf("invalid date %q", s)
	}
	return time.Time{}, false, nil
}

// parseYMD handles four-digit-year forms "2026", "2026-10", "2026-10-04"
// plus numeric month/day forms "9/15", "9/15/27", "9/15/2027".
func parseYMD(s string, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, bool, error) {
	if m := ymdRe.FindStringSubmatch(s); m != nil {
		y := atoi(m[1])
		if y < 1 {
			return time.Time{}, false, nil
		}
		if m[2] == "" {
			t, err := completeYMD(y, base.Month(), base.Day(), base, day)
			return t, true, err
		}
		if m[3] == "" {
			t, err := completeYMD(y, time.Month(atoi(m[2])), base.Day(), base, day)
			return t, true, err
		}
		if !dayInMonth(y, atoi(m[2]), atoi(m[3])) {
			return time.Time{}, true, fmt.Errorf("invalid date %q", s)
		}
		return day(y, time.Month(atoi(m[2])), atoi(m[3])), true, nil
	}
	return time.Time{}, false, nil
}

// completeYMD fills missing parts from base, advancing the inferred month
// or year until the first valid occurrence on or after the base date.
func completeYMD(y int, mo time.Month, dd int, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, error) {
	if mo < 1 || mo > 12 {
		return time.Time{}, fmt.Errorf("invalid month %d", int(mo))
	}
	clamped := min(dd, lastDay(y, mo))
	t := day(y, mo, clamped)
	for i := 0; i < 2400 && t.Before(base); i++ {
		y, mo = nextMonth(y, mo)
		clamped = min(dd, lastDay(y, mo))
		t = day(y, mo, clamped)
	}
	return t, nil
}

// parseISOWeek handles "wN", "YYYY wN [weekday]", "YYYY-wNN-D".
func parseISOWeek(s string, base time.Time, day func(int, time.Month, int) time.Time) (time.Time, bool, error) {
	if !isoWeekRe.MatchString(s) {
		return time.Time{}, false, nil
	}

	mondayOf := func(y, week int) (time.Time, bool, error) {
		if week < 1 || week > isoWeeksInYear(y) {
			return time.Time{}, true, fmt.Errorf("week %d does not exist in ISO year %d", week, y)
		}
		// Jan 4 is always in ISO week 1; back up to its Monday.
		jan4 := time.Date(y, time.January, 4, 0, 0, 0, 0, base.Location())
		dow := mod7(int(jan4.Weekday()) - 1) // Monday=0
		return jan4.AddDate(0, 0, -dow+(week-1)*7), true, nil
	}

	switch {
	case s[0] == 'w':
		week, _ := strconv.Atoi(s[1:])
		year, _ := base.ISOWeek()
		monday, ok, err := mondayOf(year, week)
		if err != nil || !ok {
			return time.Time{}, true, err
		}
		return day(monday.Year(), monday.Month(), monday.Day()), true, nil
	default:
		m := regexp.MustCompile(`^(\d{4})[ -]?w(\d{1,2})(?:[ -]?([1-7]))?$`).FindStringSubmatch(s)
		if m == nil {
			return time.Time{}, true, fmt.Errorf("invalid date %q", s)
		}
		y := atoi(m[1])
		week := atoi(m[2])
		monday, ok, err := mondayOf(y, week)
		if err != nil || !ok {
			return time.Time{}, true, err
		}
		offset := 0
		if m[3] != "" {
			offset = atoi(m[3]) - 1
		}
		t := monday.AddDate(0, 0, offset)
		if t.Year() != y && offset > 0 {
			return time.Time{}, true, fmt.Errorf("week %d does not exist in ISO year %d", week, y)
		}
		return day(t.Year(), t.Month(), t.Day()), true, nil
	}
}

func untilWeekday(from, target time.Weekday, strictlyLater bool) int {
	d := int(target - from)
	if d < 0 {
		d += 7
	}
	if d == 0 && strictlyLater {
		d = 7
	}
	return d
}

func dayInMonth(y int, mo int, d int) bool {
	if mo < 1 || mo > 12 || d < 1 {
		return false
	}
	return d <= lastDay(y, time.Month(mo))
}

func lastDay(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func addMonthsClamped(y int, m time.Month, d, n int) (int, time.Month, int) {
	total := int(m) - 1 + n
	ny := y + total/12
	nm := time.Month(total%12 + 1)
	if total < 0 {
		for total < 0 {
			ny--
			total += 12
		}
		nm = time.Month(total + 1)
	}
	return ny, nm, min(d, lastDay(ny, nm))
}

func nextMonth(y int, m time.Month) (int, time.Month) {
	if m == time.December {
		return y + 1, time.January
	}
	return y, m + 1
}

func expandYear(v string) int {
	y := atoi(v)
	if len(v) <= 2 {
		y += 2000
	}
	return y
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func mod7(n int) int {
	return ((n % 7) + 7) % 7
}

func isoWeeksInYear(y int) int {
	dec28 := time.Date(y, time.December, 28, 0, 0, 0, 0, time.UTC)
	_, w := dec28.ISOWeek()
	return w
}
