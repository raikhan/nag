package timeentry

import (
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
		{"+2h", Clock{}, true, true},
		{"-2h", Clock{}, true, true},
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
			got, present, err := Parse(tc.in)
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
		clock, present, err := Parse(text)
		if err != nil || !present {
			t.Fatalf("Parse(%q) = %v, %v", text, present, err)
		}
		canonical := clock.String()
		again, present, err := Parse(canonical)
		if err != nil || !present {
			t.Fatalf("reparse %q: %v, %v", canonical, present, err)
		}
		if again != clock {
			t.Fatalf("canonical round trip %q -> %q -> %v differs", text, canonical, again)
		}
	}
}
