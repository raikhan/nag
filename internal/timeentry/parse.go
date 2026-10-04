// Package timeentry parses clock times for the separate Time field.
// Parse is pure and allocation-light: it works on trimmed string slices
// with checked digit accumulation, never touching regexes, locations or
// dates.
package timeentry

import (
	"fmt"
	"strings"
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
// midnight/noon mapped correctly; minutes are 0..59. Seconds, signed
// offsets, dates, zones and trailing garbage are rejected.
func Parse(raw string) (clock Clock, present bool, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Clock{}, false, nil
	}
	invalid := fmt.Errorf("invalid time %q", s)

	body := strings.ToLower(s)
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
