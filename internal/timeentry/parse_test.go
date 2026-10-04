package timeentry

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// The captured clock documents the intended editing context; the parser
// itself is zone-independent.
var capturedNow, _ = time.Parse("2006-01-02 15:04 MST", "2026-10-04 13:15 AWST")

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func TestParseTimeEntry(t *testing.T) {
	_ = mustLocation(t, "Australia/Perth")

	cases := []struct {
		in      string
		want    Clock
		present bool
		wantErr bool
	}{
		{"", Clock{}, false, false},
		{"   ", Clock{}, false, false},
		{"6p", Clock{18, 0}, true, false},
		{"6pm", Clock{18, 0}, true, false},
		{"6 p", Clock{18, 0}, true, false},
		{"6 pm", Clock{18, 0}, true, false},
		{"6P", Clock{18, 0}, true, false},
		{"6PM", Clock{18, 0}, true, false},
		{"6a", Clock{6, 0}, true, false},
		{"6am", Clock{6, 0}, true, false},
		{"12a", Clock{0, 0}, true, false},
		{"12am", Clock{0, 0}, true, false},
		{"12p", Clock{12, 0}, true, false},
		{"12pm", Clock{12, 0}, true, false},
		{"1413", Clock{14, 13}, true, false},
		{"14:13", Clock{14, 13}, true, false},
		{"9:05", Clock{9, 5}, true, false},
		{"615p", Clock{18, 15}, true, false},
		{"930a", Clock{9, 30}, true, false},
		{"6", Clock{6, 0}, true, false},
		{"23", Clock{23, 0}, true, false},
		{"0", Clock{0, 0}, true, false},
		{"0900", Clock{9, 0}, true, false},
		{"2:13pm", Clock{14, 13}, true, false},
		// Invalid.
		{"24:00", Clock{}, true, true},
		{"24", Clock{}, true, true},
		{"1260", Clock{}, true, true},
		{"13pm", Clock{}, true, true},
		{"13a", Clock{}, true, true},
		{"6:7", Clock{}, true, true},
		{"6:075", Clock{}, true, true},
		{"12345", Clock{}, true, true},
		{"6:13:00", Clock{}, true, true},
		{"6:13pm UTC", Clock{}, true, true},
		{"2026-10-04", Clock{}, true, true},
		{"oct 4", Clock{}, true, true},
		{"6pmx", Clock{}, true, true},
		{"p", Clock{}, true, true},
		{"a", Clock{}, true, true},
		{"pm", Clock{}, true, true},
		{"1 30", Clock{}, true, true},
		{"six", Clock{}, true, true},
		{"6h", Clock{}, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, present, err := Parse(tc.in, capturedNow)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", tc.in, got)
				}
				if !strings.Contains(err.Error(), "invalid time") {
					t.Fatalf("Parse(%q) error = %v", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.in, err)
			}
			if present != tc.present {
				t.Fatalf("Parse(%q) present = %v, want %v", tc.in, present, tc.present)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseRelativeTimeEntry covers the signed offset forms, which resolve
// against the captured clock and wrap at midnight without moving the date.
func TestParseRelativeTimeEntry(t *testing.T) {
	cases := []struct {
		in      string
		want    Clock
		wantErr bool
	}{
		{"+2h", Clock{15, 15}, false},
		{"-2h", Clock{11, 15}, false},
		{"+75min", Clock{14, 30}, false},
		{"+12m", Clock{13, 27}, false},
		{"-90min", Clock{11, 45}, false},
		{"+13h", Clock{2, 15}, false}, // wraps past midnight
		{"+1h30m", Clock{}, true},
		{"+2 h", Clock{}, true},
		{"+2s", Clock{}, true},
		{"+", Clock{}, true},
		{"+9999999h", Clock{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, present, err := Parse(tc.in, capturedNow)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", tc.in, got)
				}
				if present {
					t.Fatalf("Parse(%q) present = true, want false on error", tc.in)
				}
				return
			}
			if err != nil || !present {
				t.Fatalf("Parse(%q) = %v, %v", tc.in, present, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestClockString(t *testing.T) {
	cases := []struct {
		clock Clock
		want  string
	}{
		{Clock{18, 0}, "6pm"},
		{Clock{14, 13}, "2:13pm"},
		{Clock{0, 0}, "12am"},
		{Clock{12, 0}, "12pm"},
		{Clock{9, 5}, "9:05am"},
		{Clock{0, 30}, "12:30am"},
		{Clock{23, 59}, "11:59pm"},
	}
	for _, tc := range cases {
		if got := tc.clock.String(); got != tc.want {
			t.Errorf("Clock(%d:%02d).String() = %q, want %q", tc.clock.Hour, tc.clock.Minute, got, tc.want)
		}
	}
}

func TestParseTimeEntryCanonicalRoundTrip(t *testing.T) {
	for _, text := range []string{"6p", "6pm", "12a", "12p", "1413", "14:13", "615p", "9:05", "0", "23"} {
		clock, present, err := Parse(text, capturedNow)
		if err != nil || !present {
			t.Fatalf("Parse(%q) = %v, %v", text, present, err)
		}
		canonical := clock.String()
		again, present, err := Parse(canonical, capturedNow)
		if err != nil || !present {
			t.Fatalf("reparse %q: %v, %v", canonical, present, err)
		}
		if again != clock {
			t.Fatalf("canonical round trip %q -> %q -> %v differs", text, canonical, again)
		}
	}
}
func TestParseLead(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"45m", 45 * time.Minute, false},
		{"45min", 45 * time.Minute, false},
		{"45mins", 45 * time.Minute, false},
		{"2h30m", 150 * time.Minute, false},
		{"1h", time.Hour, false},
		{"2hr30min", 150 * time.Minute, false},
		{"3d", 72 * time.Hour, false},
		{"3days", 72 * time.Hour, false},
		{" 2h30m ", 150 * time.Minute, false},
		// The app's 1 week row seeds as 168h; "w" is not in the grammar.
		{"168h", 7 * 24 * time.Hour, false},
		{"0m", 0, true},
		{"0h0m", 0, true},
		{"", 0, true},
		{"   ", 0, true},
		{"-5m", 0, true},
		{"bogus", 0, true},
		{"m", 0, true},
		{"5", 0, true},
		{"5x", 0, true},
		{"1234567m", 0, true},
		{"999999d", 0, true},
	}
	for _, c := range cases {
		got, err := ParseLead(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("ParseLead(%q) = %v, want error", c.in, got)
			}
			if want := "invalid lead time " + strconv.Quote(c.in); err.Error() != want {
				t.Fatalf("ParseLead(%q) error = %q, want %q", c.in, err.Error(), want)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLead(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseLead(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
