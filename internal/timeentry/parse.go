// Package timeentry parses clock times for the separate Time field.
// Parse is pure and allocation-light: it works on trimmed string slices
// with checked digit accumulation, never touching regexes, locations or
// dates. Absolute forms are zone-independent; relative forms (a sign, an
// amount and an hour/minute unit) resolve against the now clock captured
// when the form opened and are clock-only, wrapping at midnight without
// touching the date.
package timeentry

import (
	"fmt"
	"strings"
	"time"
)

// Clock is a wall-clock hour and minute.
type Clock struct {
	Hour, Minute int
}

// String renders lowercase 12-hour text without a leading zero: 6pm,
// 2:13pm, 12am.
func (c Clock) String() string {
	hour := c.Hour % 12
	if hour == 0 {
		hour = 12
	}
	suffix := "am"
	if c.Hour >= 12 {
		suffix = "pm"
	}
	if c.Minute == 0 {
		return fmt.Sprintf("%d%s", hour, suffix)
	}
	return fmt.Sprintf("%d:%02d%s", hour, c.Minute, suffix)
}

// Parse resolves raw text into a clock. Empty input yields present=false
// and no error; a valid clock yields present=true; anything else is an
// error. Grammar: optional English suffix (a/am/p/pm, case insensitive,
// optional space before it); 1-2 digits mean an hour; 3-4 digits mean
// HMM/HHMM; "H:MM"/"HH:MM" require two minute digits. Without a suffix
// hours are 24-hour (0..23); with a suffix hours must be 1..12 with
// midnight/noon mapped correctly; minutes are 0..59. A leading sign
// selects the relative form: 1-6 digits plus a unit ("h", "hr", "hrs",
// "m", "min", "mins"), applied to now's clock. Seconds, dates, zones and
// trailing garbage are rejected.
func Parse(raw string, now time.Time) (clock Clock, present bool, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Clock{}, false, nil
	}
	invalid := fmt.Errorf("invalid time %q", s)

	body := strings.ToLower(s)
	if body[0] == '+' || body[0] == '-' {
		return parseOffset(body, now, invalid)
	}
	isPM := false
	hasSuffix := false
	for _, suf := range []struct {
		text string
		pm   bool
	}{
		{" am", false}, {" pm", true}, {" a", false}, {" p", true},
		{"am", false}, {"pm", true}, {"a", false}, {"p", true},
	} {
		if strings.HasSuffix(body, suf.text) {
			body = strings.TrimSpace(strings.TrimSuffix(body, suf.text))
			isPM = suf.pm
			hasSuffix = true
			break
		}
	}
	if body == "" {
		return Clock{}, false, invalid
	}

	var hour, minute int
	if i := strings.IndexByte(body, ':'); i >= 0 {
		// Colon form: 1-2 hour digits, exactly two minute digits, one colon.
		if strings.IndexByte(body[i+1:], ':') >= 0 {
			return Clock{}, false, invalid
		}
		h, ok := parseDigits(body[:i], 2)
		if !ok {
			return Clock{}, false, invalid
		}
		m, ok := parseDigits(body[i+1:], 2)
		if !ok || len(body[i+1:]) != 2 {
			return Clock{}, false, invalid
		}
		hour, minute = h, m
	} else {
		// All digits: 1-2 = hour, 3 = HMM, 4 = HHMM.
		switch len(body) {
		case 1, 2:
			h, ok := parseDigits(body, 2)
			if !ok {
				return Clock{}, false, invalid
			}
			hour = h
		case 3, 4:
			h, ok := parseDigits(body[:len(body)-2], 2)
			m, ok2 := parseDigits(body[len(body)-2:], 2)
			if !ok || !ok2 {
				return Clock{}, false, invalid
			}
			hour, minute = h, m
		default:
			return Clock{}, false, invalid
		}
	}

	if minute > 59 {
		return Clock{}, false, invalid
	}
	if hasSuffix {
		if hour < 1 || hour > 12 {
			return Clock{}, false, invalid
		}
		switch {
		case isPM && hour != 12:
			hour += 12
		case !isPM && hour == 12:
			hour = 0
		}
	} else if hour > 23 {
		return Clock{}, false, invalid
	}
	return Clock{Hour: hour, Minute: minute}, true, nil
}

// parseOffset resolves a relative clock form: a sign, 1-6 ASCII digits and
// a unit ("h", "hr", "hrs", "m", "min", "mins"). The offset is applied to
// now's clock and wraps at midnight without touching the date.
func parseOffset(s string, now time.Time, invalid error) (Clock, bool, error) {
	rest := s[1:]
	n, digits := 0, 0
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c < '0' || c > '9' {
			break
		}
		digits++
		if digits > 6 {
			return Clock{}, false, invalid
		}
		n = n*10 + int(c-'0')
	}
	if digits == 0 {
		return Clock{}, false, invalid
	}
	minutes := 0
	switch rest[digits:] {
	case "h", "hr", "hrs":
		minutes = n * 60
	case "m", "min", "mins":
		minutes = n
	default:
		return Clock{}, false, invalid
	}
	if s[0] == '-' {
		minutes = -minutes
	}
	total := (now.Hour()*60 + now.Minute() + minutes) % 1440
	if total < 0 {
		total += 1440
	}
	return Clock{Hour: total / 60, Minute: total % 60}, true, nil
}

// parseDigits accumulates ASCII digits with a maximum digit count and an
// overflow guard, reporting whether the text is a valid nonnegative integer.
func parseDigits(s string, maxDigits int) (int, bool) {
	if s == "" || len(s) > maxDigits {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ParseLead resolves free-form lead-time text into a duration: one or more
// "<digits><unit>" groups summed together, where a unit is "m"/"min"/"mins",
// "h"/"hr"/"hrs" or "d"/"day"/"days". Groups carry 1-6 digits each. The
// result must be positive, because a zero or negative lead time is an alarm
// EventKit drops rather than one the user can keep.
func ParseLead(raw string) (time.Duration, error) {
	invalid := fmt.Errorf("invalid lead time %q", raw)
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, invalid
	}
	// A day is the largest unit, so this bound keeps the multiply inside
	// Duration's range.
	const maxCount = 1 << 40
	var total time.Duration
	for len(s) > 0 {
		digits := 0
		for digits < len(s) && s[digits] >= '0' && s[digits] <= '9' {
			digits++
		}
		n, ok := parseDigits(s[:digits], 6)
		if !ok {
			return 0, invalid
		}
		s = s[digits:]
		unit, width, ok := leadUnit(s)
		if !ok || n > maxCount {
			return 0, invalid
		}
		total += time.Duration(n) * unit
		s = s[width:]
	}
	if total <= 0 {
		return 0, invalid
	}
	return total, nil
}

// leadUnit matches the unit spelling at the head of s, returning its size and
// width. Longer spellings are tried first so "mins" is never read as "m"
// followed by trailing garbage.
func leadUnit(s string) (time.Duration, int, bool) {
	units := [...]struct {
		text string
		size time.Duration
	}{
		{"mins", time.Minute}, {"min", time.Minute}, {"m", time.Minute},
		{"hrs", time.Hour}, {"hr", time.Hour}, {"h", time.Hour},
		{"days", 24 * time.Hour}, {"day", 24 * time.Hour}, {"d", 24 * time.Hour},
	}
	for _, u := range units {
		if strings.HasPrefix(s, u.text) {
			return u.size, len(u.text), true
		}
	}
	return 0, 0, false
}
