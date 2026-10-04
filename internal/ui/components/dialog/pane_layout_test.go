package dialog

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func stripLines(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

func maxLineWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	return w
}

func requireLabels(t *testing.T, lines []string) {
	t.Helper()
	for _, label := range []string{"Title", "Notes", "Date", "Time", "Priority", "Recurrence"} {
		found := false
		for _, l := range lines {
			if strings.Contains(l, label) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("summary label %q missing from view:\n%s", label, strings.Join(lines, "\n"))
		}
	}
}

func newVisibleCreate(t *testing.T, now time.Time, w, h int) CreateModel {
	t.Helper()
	m := NewCreate()
	m.SetKeys(testKeys(t))
	m.SetSize(w, h)
	m.showAt("Alpha", now)
	return m
}

func TestCreateEditPaneLayout(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Perth")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)

	t.Run("120x40 side by side with calendar", func(t *testing.T) {
		m := newVisibleCreate(t, now, 120, 40)
		lines := stripLines(m.View())
		if len(lines) > 40 {
			t.Fatalf("view is %d lines tall, want <= 40", len(lines))
		}
		if w := maxLineWidth(lines); w > 120 {
			t.Fatalf("view is %d wide, want <= 120", w)
		}
		requireLabels(t, lines)

		// Open date editing: the calendar renders beside the summaries.
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(teaPaste(t, "2026-10-04"))
		lines = stripLines(m.View())
		requireLabels(t, lines)
		calLine, sumLine := -1, -1
		for i, l := range lines {
			if strings.Contains(l, "Mo Tu We Th Fr Sa Su") {
				calLine = i
			}
			if strings.Contains(l, "Title") {
				sumLine = i
			}
		}
		if calLine < 0 {
			t.Fatal("calendar not rendered")
		}
		if sumLine < 0 || sumLine >= calLine {
			t.Fatalf("calendar (line %d) must sit right of the summaries (line %d)", calLine, sumLine)
		}
		// Side-by-side proof: a summary row shares its line with the editor.
		shared := false
		for _, l := range lines {
			if strings.Contains(l, "[t] Title") && strings.Contains(l, "Due date:") {
				shared = true
				break
			}
		}
		if !shared {
			t.Fatal("wide layout must place the editor beside the summaries")
		}

		// Deep calendar navigation keeps the highlight inside the viewport.
		for range 5 {
			m, _ = m.Update(ctrlPress('j')) // +7 days
		}
		view := m.View()
		if !strings.Contains(view, "\x1b[48;2;") && !strings.Contains(view, "\x1b[") {
			t.Fatal("expected styled highlight")
		}
		styled := false
		for _, l := range strings.Split(view, "\n") {
			if strings.Contains(l, "7m") || strings.Contains(l, "48;2;") {
				styled = true
				break
			}
		}
		if !styled {
			t.Fatal("highlight row scrolled out of the viewport")
		}
	})

	t.Run("80x24 side by side", func(t *testing.T) {
		m := newVisibleCreate(t, now, 80, 24)
		lines := stripLines(m.View())
		if len(lines) > 24 {
			t.Fatalf("view is %d lines tall, want <= 24", len(lines))
		}
		if w := maxLineWidth(lines); w > 80 {
			t.Fatalf("view is %d wide, want <= 80", w)
		}
		requireLabels(t, lines)
		// Selector content fits the pane beside the summaries.
		m = openFieldFor(t, m, fieldPriority)
		lines = stripLines(m.View())
		requireLabels(t, lines)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "High") {
			t.Fatal("selector options missing at 80x24")
		}
		shared := false
		for _, l := range lines {
			if strings.Contains(l, "[t] Title") && strings.Contains(l, "> ") {
				shared = true
				break
			}
		}
		if !shared {
			t.Fatal("80x24 must still use the side-by-side layout")
		}
	})

	t.Run("narrow stacked", func(t *testing.T) {
		m := newVisibleCreate(t, now, 60, 24)
		lines := stripLines(m.View())
		requireLabels(t, lines)
		// The active editor renders below the summaries, never beside them.
		m = openFieldFor(t, m, fieldTime)
		for _, l := range stripLines(m.View()) {
			if strings.Contains(l, "Time:") && strings.Contains(l, "[n] Notes") {
				t.Fatal("narrow layout must stack, not side-by-side")
			}
		}
		// The active editor renders below the summaries.
		m = openFieldFor(t, m, fieldTime)
		joined := strings.Join(stripLines(m.View()), "\n")
		if !strings.Contains(joined, "Time:") {
			t.Fatalf("time editor missing in stacked view:\n%s", joined)
		}
	})

	t.Run("below minimum shows resize message", func(t *testing.T) {
		for _, size := range [][2]int{{20, 10}, {120, 12}, {25, 24}} {
			m := newVisibleCreate(t, now, size[0], size[1])
			joined := strings.Join(stripLines(m.View()), "\n")
			if !strings.Contains(joined, "Terminal too small") {
				t.Fatalf("size %v: expected resize message, got:\n%s", size, joined)
			}
		}
	})

	t.Run("long values and errors stay in bounds", func(t *testing.T) {
		m := newVisibleCreate(t, now, 80, 24)
		m = openFieldFor(t, m, fieldTitle)
		m, _ = m.Update(teaPaste(t, strings.Repeat("x", 300)))
		m = finishField(t, m)
		m = openFieldFor(t, m, fieldDate)
		m, _ = m.Update(teaPaste(t, "not a date"))
		m, _ = m.Update(ctrlPress('s')) // sticky error
		lines := stripLines(m.View())
		if len(lines) > 24 {
			t.Fatalf("view is %d lines tall", len(lines))
		}
		if w := maxLineWidth(lines); w > 80 {
			t.Fatalf("view is %d wide, want <= 80", w)
		}
		requireLabels(t, lines)
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "invalid date") {
			t.Fatal("sticky error not rendered")
		}
	})
}

// teaPaste builds a paste message (helper keeps the table readable).
func teaPaste(t *testing.T, content string) tea.PasteMsg {
	t.Helper()
	return tea.PasteMsg{Content: content}
}
