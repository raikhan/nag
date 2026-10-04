package dateentry

import (
	"testing"
	"time"
)

func TestParseDateEntry(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc) // Sunday
	base := time.Date(2026, time.October, 4, 9, 0, 0, 0, loc)

	mk := func(y int, m time.Month, d, h, mi int) time.Time {
		return time.Date(y, m, d, h, mi, 0, 0, loc)
	}

	cases := []struct {
		name    string
		in      string
		want    *time.Time
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"today", "today", new(mk(2026, time.October, 4, 9, 0)), false},
		{"to alias", "to", new(mk(2026, time.October, 4, 9, 0)), false},
		{"tod alias", "tod", new(mk(2026, time.October, 4, 9, 0)), false},
		{"dot", ".", new(mk(2026, time.October, 4, 9, 0)), false},
		{"tomorrow", "tomorrow", new(mk(2026, time.October, 5, 9, 0)), false},
		{"tom alias", "tom", new(mk(2026, time.October, 5, 9, 0)), false},
		{"tomo alias", "tomo", new(mk(2026, time.October, 5, 9, 0)), false},
		{"yesterday", "yesterday", new(mk(2026, time.October, 3, 9, 0)), false},
		{"yes alias", "yes", new(mk(2026, time.October, 3, 9, 0)), false},
		{"case insensitive", "TODAY", new(mk(2026, time.October, 4, 9, 0)), false},
		{"weekday tue", "tue", new(mk(2026, time.October, 6, 9, 0)), false},
		{"weekday full", "tuesday", new(mk(2026, time.October, 6, 9, 0)), false},
		{"weekday fri", "fri", new(mk(2026, time.October, 9, 9, 0)), false},
		{"next tue from sunday", "next tue", new(mk(2026, time.October, 6, 9, 0)), false},
		{"eow", "eow", new(mk(2026, time.October, 9, 9, 0)), false},
		{"eom", "eom", new(mk(2026, time.October, 31, 9, 0)), false},
		{"plus 2d", "+2d", new(mk(2026, time.October, 6, 9, 0)), false},
		{"plus bare days", "+4", new(mk(2026, time.October, 8, 9, 0)), false},
		{"plus zero", "+0", new(mk(2026, time.October, 4, 9, 0)), false},
		{"plus 7w", "+7w", new(mk(2026, time.November, 22, 9, 0)), false},
		{"plus 2h", "+2h", new(mk(2026, time.October, 4, 15, 15)), false},
		{"minus 1m", "-1m", new(mk(2026, time.September, 4, 9, 0)), false},
		{"plus 1y", "+1y", new(mk(2027, time.October, 4, 9, 0)), false},
		{"nth weekday plus", "+2tue", new(mk(2026, time.October, 13, 9, 0)), false},
		{"nth weekday minus", "-1fri", new(mk(2026, time.October, 2, 9, 0)), false},
		{"iso week bare", "w41", new(mk(2026, time.October, 5, 9, 0)), false},
		{"iso week explicit", "2026 w41 2", new(mk(2026, time.October, 6, 9, 0)), false},
		{"iso week dashed", "2026-w41-3", new(mk(2026, time.October, 7, 9, 0)), false},
		{"iso week monday default", "2026w41", new(mk(2026, time.October, 5, 9, 0)), false},
		{"iso year boundary", "2027 w1 1", new(mk(2027, time.January, 4, 9, 0)), false},
		{"ymd", "2025-03-15", new(mk(2025, time.March, 15, 9, 0)), false},
		{"ymd with time", "2025-03-15 14:30", new(mk(2025, time.March, 15, 14, 30)), false},
		{"md", "9/15", new(mk(2027, time.September, 15, 9, 0)), false},
		{"md this year", "10/15", new(mk(2026, time.October, 15, 9, 0)), false},
		{"mdy short", "9/15/27", new(mk(2027, time.September, 15, 9, 0)), false},
		{"mdy full", "9/15/2027", new(mk(2027, time.September, 15, 9, 0)), false},
		{"day alone future", "15", new(mk(2026, time.October, 15, 9, 0)), false},
		{"day alone rolls month", "1", new(mk(2026, time.November, 1, 9, 0)), false},
		{"month day", "sep 15", new(mk(2027, time.September, 15, 9, 0)), false},
		{"month day this month", "oct 20", new(mk(2026, time.October, 20, 9, 0)), false},
		{"month day year", "sep 15 2027", new(mk(2027, time.September, 15, 9, 0)), false},
		{"day month year", "22 sept 2027", new(mk(2027, time.September, 22, 9, 0)), false},
		{"day month", "22 sept", new(mk(2027, time.September, 22, 9, 0)) /* inferred year, first on/after base */, false},
		{"explicit time on singleton", "today 23:59", new(mk(2026, time.October, 4, 23, 59)), false},
		{"month clamp single", "-1m", new(mk(2026, time.September, 4, 9, 0)), false},
		{"invalid empty time", "today 25:00", nil, true},
		{"invalid garbage", "not a date", nil, true},
		{"invalid month day", "feb 30 2026", nil, true},
		{"invalid md", "13/40 2026", nil, true},
		{"range rejected", "2026-10-01--2026-10-05", nil, true},
		{"iso week overflow", "2026 w60", nil, true},
		{"offset overflow", "+50000y", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in, now, base)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.in, err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("Parse(%q) = %v, want nil", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want %v", tc.in, tc.want)
			}
			if !got.Equal(*tc.want) {
				t.Fatalf("Parse(%q) = %v, want %v", tc.in, got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}

	// Edit base keeps the original clock for date-only expressions.
	editBase := time.Date(2026, time.October, 6, 14, 30, 0, 0, loc)
	got, err := Parse("+2d", now, editBase)
	if err != nil || !got.Equal(mk(2026, time.October, 6, 14, 30)) {
		t.Fatalf("date-only edit base clock = %v, err %v; want Oct 6 14:30", got, err)
	}

	// Double signs anchor to base.
	pastBase := time.Date(2026, time.September, 20, 11, 0, 0, 0, loc)
	got, err = Parse("++5d", now, pastBase)
	if err != nil || !got.Equal(mk(2026, time.September, 25, 11, 0)) {
		t.Fatalf("++5d = %v, err %v; want Sep 25 11:00", got, err)
	}
	got, err = Parse("--1m", now, pastBase)
	if err != nil || !got.Equal(mk(2026, time.August, 20, 11, 0)) {
		t.Fatalf("--1m = %v, err %v; want Aug 20 11:00", got, err)
	}

	// Weekday anchors accept the same day; next-weekday adds seven.
	tue := time.Date(2026, time.October, 6, 13, 15, 0, 0, loc)
	got, err = Parse("tue", tue, time.Date(2026, time.October, 6, 9, 0, 0, 0, loc))
	if err != nil || !got.Equal(mk(2026, time.October, 6, 9, 0)) {
		t.Fatalf("tue on Tuesday = %v, err %v; want same day", got, err)
	}
	got, err = Parse("next tue", tue, base)
	if err != nil || !got.Equal(mk(2026, time.October, 13, 9, 0)) {
		t.Fatalf("next tue on Tuesday = %v, err %v; want Oct 13", got, err)
	}

	// Month/year clamping: Oct 31 base +1 month (double sign) -> Nov 30.
	got, err = Parse("++1m", now, time.Date(2026, time.October, 31, 9, 0, 0, 0, loc))
	if err != nil || !got.Equal(mk(2026, time.November, 30, 9, 0)) {
		t.Fatalf("Oct 31 ++1m = %v, err %v; want Nov 30", got, err)
	}

	// Feb 29 infers the next valid year.
	got, err = Parse("feb 29", now, base)
	if err != nil || !got.Equal(mk(2028, time.February, 29, 9, 0)) {
		t.Fatalf("feb 29 = %v, err %v; want Feb 29 2028", got, err)
	}
}
