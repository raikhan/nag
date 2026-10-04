package datepicker

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/oronbz/nag/internal/keybind"
)

func testKeys(t *testing.T) keybind.Map {
	t.Helper()
	m, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile defaults: %v", err)
	}
	return m
}

// renderGrid sets the picker value, renders the view and returns the six
// stripped calendar grid rows.
func renderGrid(t *testing.T, m Model, value string) (header string, rows []string) {
	t.Helper()
	m.Focus()
	m.SetValue(value)
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	var weekdayIdx = -1
	for i, l := range lines {
		if l == "Mo Tu We Th Fr Sa Su" {
			weekdayIdx = i
			break
		}
	}
	if weekdayIdx < 0 {
		t.Fatalf("weekday header not found in view:\n%s", ansi.Strip(m.View()))
	}
	for i := weekdayIdx - 1; i >= 0; i-- {
		if lines[i] != "" {
			header = lines[i]
			break
		}
	}
	for i := weekdayIdx + 1; i <= weekdayIdx+6; i++ {
		rows = append(rows, lines[i])
	}
	return header, rows
}

// cells splits a stripped grid row into its seven two-character cells.
func cells(row string) ([]string, error) {
	if len(row) != 7*3-1 {
		return nil, fmt.Errorf("row %q has width %d, want 20", row, len(row))
	}
	out := make([]string, 7)
	for c := range 7 {
		out[c] = row[c*3 : c*3+2]
	}
	return out, nil
}

func fmtDay(d time.Time) string {
	s := fmt.Sprintf("%d", d.Day())
	if len(s) == 1 {
		s = " " + s
	}
	return s
}

// checkGrid verifies every one of the 42 cells against an independent
// reference grid and that the highlighted cell carries the highlight style at
// its expected visible column.
func checkGrid(t *testing.T, m Model, rawView string, shown time.Time) {
	t.Helper()
	loc := m.base.Location()
	first := time.Date(shown.Year(), shown.Month(), 1, 0, 0, 0, 0, loc)
	offset := (int(first.Weekday()) + 6) % 7

	// Extract raw calendar rows (between weekday header and end of view).
	lines := strings.Split(rawView, "\n")
	var gridStart = -1
	for i, l := range lines {
		if ansi.Strip(l) == "Mo Tu We Th Fr Sa Su" {
			gridStart = i
			break
		}
	}
	if gridStart < 0 {
		t.Fatalf("weekday header not found")
	}

	for w := range 6 {
		rowRaw := lines[gridStart+1+w]
		got, err := cells(ansi.Strip(rowRaw))
		if err != nil {
			t.Fatalf("week %d: %v", w, err)
		}
		for c := range 7 {
			day := first.AddDate(0, 0, -offset+w*7+c)
			if got[c] != fmtDay(day) {
				t.Errorf("week %d col %d: got %q, want %q (%s)", w, c, got[c], fmtDay(day), day.Format("2006-01-02"))
			}
		}
	}

	// Highlight: the only cell whose escape sequence starts at the highlight's
	// visible column, and whose visible content is the highlight day.
	hl := m.highlight
	hlDay := first.AddDate(0, 0, -offset)
	found := false
	for d := 0; d < 42; d++ {
		if sameCivilDate(hlDay, hl) {
			// Verify the style starts exactly at this cell in its row.
			rowIdx := gridStart + 1 + d/7
			col := (d % 7) * 3
			rowRaw := lines[rowIdx]
			if !styledCellAt(rowRaw, col, fmtDay(hl)) {
				t.Errorf("highlight %s not styled at visible column %d in %q", hl.Format("2006-01-02"), col, ansi.Strip(rowRaw))
			}
			found = true
			break
		}
		hlDay = hlDay.AddDate(0, 0, 1)
	}
	if !found {
		t.Errorf("highlight %s not present in the six-week grid", hl.Format("2006-01-02"))
	}
}

func sameCivilDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// styledCellAt reports whether the raw row carries a style escape sequence
// starting at the given visible column, wrapping the expected content.
func styledCellAt(row string, visibleCol int, content string) bool {
	runes := []rune(row)
	for i := range runes {
		if runes[i] != 0x1b {
			continue
		}
		if ansi.StringWidth(string(runes[:i])) == visibleCol {
			rest := string(runes[i:])
			stripped := ansi.Strip(rest)
			if strings.HasPrefix(stripped, content) {
				return true
			}
		}
	}
	return false
}

func TestCalendarWeekdayColumns(t *testing.T) {
	keys := testKeys(t)
	perth, err := time.LoadLocation("Australia/Perth")
	if err != nil {
		t.Fatalf("load Perth: %v", err)
	}
	now := time.Date(2026, 10, 4, 13, 15, 0, 0, perth)

	t.Run("october 2026 leading days", func(t *testing.T) {
		m := New(now, now, keys)
		header, rows := renderGrid(t, m, "2026-10-04")
		if header != "October 2026" {
			t.Errorf("header = %q", header)
		}
		want := "28 29 30  1  2  3  4"
		if rows[0] != want {
			t.Errorf("first row = %q, want %q", rows[0], want)
		}
		// Highlighted October 4 sits in the Sunday column (index 6).
		if got := rows[0][6*3 : 6*3+2]; got != " 4" {
			t.Errorf("sunday cell = %q, want %q", got, " 4")
		}
		checkGrid(t, m, m.View(), now)
	})

	t.Run("february 2024 leap", func(t *testing.T) {
		feb := time.Date(2024, 2, 15, 9, 0, 0, 0, perth)
		m := New(now, feb, keys)
		m.SetValue("2024-02-29")
		checkGrid(t, m, m.View(), feb)
		_, rows := renderGrid(t, m, "2024-02-29")
		// Feb 2024 starts on a Tuesday (offset 1): 29 30 31  1 ...
		if rows[0] != "29 30 31  1  2  3  4" {
			t.Errorf("first row = %q", rows[0])
		}
		// Day 29 must appear in the fifth week under Thursday.
		last := rows[4]
		c, err := cells(last)
		if err != nil {
			t.Fatal(err)
		}
		if c[3] != "29" {
			t.Errorf("leap day cell = %q, want 29 under Thursday", c[3])
		}
	})

	t.Run("september 2025 monday start", func(t *testing.T) {
		sep := time.Date(2025, 9, 10, 9, 0, 0, 0, perth)
		m := New(now, sep, keys)
		m.SetValue("2025-09-01")
		_, rows := renderGrid(t, m, "2025-09-01")
		if rows[0] != " 1  2  3  4  5  6  7" {
			t.Errorf("first row = %q, want 1..7 Monday-first", rows[0])
		}
		checkGrid(t, m, m.View(), sep)
	})

	t.Run("december 2026 year crossing", func(t *testing.T) {
		dec := time.Date(2026, 12, 15, 9, 0, 0, 0, perth)
		m := New(now, dec, keys)
		m.SetValue("2026-12-25")
		_, rows := renderGrid(t, m, "2026-12-25")
		c, err := cells(rows[5])
		if err != nil {
			t.Fatal(err)
		}
		// Last week must reach into January 2027 (Jan 4..10).
		if c[0] != " 4" || c[6] != "10" {
			t.Errorf("final row %q should be January 2027 4..10", rows[5])
		}
		checkGrid(t, m, m.View(), dec)
	})
}

// TestCalendarKeybinds covers the ctrl+arrow/pgup/pgdn month navigation and
// the ctrl+d reset on the due date picker.
func TestCalendarKeybinds(t *testing.T) {
	keys := testKeys(t)
	perth, err := time.LoadLocation("Australia/Perth")
	if err != nil {
		t.Fatalf("load Perth: %v", err)
	}
	now := time.Date(2026, 10, 4, 13, 15, 0, 0, perth)
	civil := func(y int, mo time.Month, d int) time.Time {
		return time.Date(y, mo, d, 0, 0, 0, 0, perth)
	}

	t.Run("ctrl arrows move day and week", func(t *testing.T) {
		m := New(now, now, keys)
		m.Focus()
		m.SetValue("2026-10-04")
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl})
		if !sameCivilDate(m.highlight, civil(2026, 10, 5)) {
			t.Fatalf("ctrl+right highlight = %v", m.highlight)
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
		if !sameCivilDate(m.highlight, civil(2026, 10, 11)) {
			t.Fatalf("ctrl+down highlight = %v", m.highlight)
		}
		if m.input.Value() != "2026-10-11" {
			t.Fatalf("ctrl+down wrote %q", m.input.Value())
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
		if !sameCivilDate(m.highlight, civil(2026, 10, 4)) {
			t.Fatalf("ctrl+up highlight = %v", m.highlight)
		}
	})

	t.Run("month moves clamp the day", func(t *testing.T) {
		m := New(now, now, keys)
		m.Focus()
		m.SetValue("2026-10-31")
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModCtrl})
		if !sameCivilDate(m.highlight, civil(2026, 9, 30)) || m.input.Value() != "2026-09-30" {
			t.Fatalf("ctrl+pgup = %v %q", m.highlight, m.input.Value())
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown, Mod: tea.ModCtrl})
		if m.input.Value() != "2026-10-30" {
			t.Fatalf("ctrl+pgdown from Sep 30 = %q", m.input.Value())
		}
		// Oct 31 forward one month lands on Nov 30.
		m.SetValue("2026-10-31")
		m, _ = m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
		if !sameCivilDate(m.highlight, civil(2026, 11, 30)) || m.input.Value() != "2026-11-30" {
			t.Fatalf("ctrl+n from Oct 31 = %v %q", m.highlight, m.input.Value())
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
		if m.input.Value() != "2026-10-30" {
			t.Fatalf("ctrl+p from Nov 30 = %q", m.input.Value())
		}
	})

	t.Run("ctrl+d clears and restores the base highlight", func(t *testing.T) {
		base := civil(2026, 10, 4)
		m := New(now, base, keys)
		m.Focus()
		m.SetValue("2026-12-25")
		m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
		if m.input.Value() != "" {
			t.Fatalf("ctrl+d left %q", m.input.Value())
		}
		if !sameCivilDate(m.highlight, base) {
			t.Fatalf("ctrl+d highlight = %v, want base %v", m.highlight, base)
		}
	})

	t.Run("down and up cycle suggestions", func(t *testing.T) {
		m := New(now, now, keys)
		m.Focus()
		m.SetValue("to")
		before := m.input.CurrentSuggestionIndex()
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		after := m.input.CurrentSuggestionIndex()
		if after == before {
			t.Fatalf("down did not cycle suggestions (%d)", after)
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		if m.input.CurrentSuggestionIndex() != before {
			t.Fatalf("up did not cycle back (%d)", m.input.CurrentSuggestionIndex())
		}
	})
}
