package helpoverlay

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/keybind"
)

func testKeys(t *testing.T) keybind.Map {
	t.Helper()
	keys, err := keybind.Compile(keybind.Defaults())
	if err != nil {
		t.Fatalf("compile keys: %v", err)
	}
	return keys
}

func press(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// TestSetSizePreservesScrollOffset guards against relayout snapping the overlay
// back to the top. Layout re-runs on every data refresh (the auto-refresh tick
// fires every couple of seconds) while the overlay stays open.
func TestSetSizePreservesScrollOffset(t *testing.T) {
	m := New()
	m.SetKeys(testKeys(t))
	m.SetSize(120, 40)
	m.Toggle()

	for range 5 {
		m, _ = m.Update(press('j'))
	}
	before := m.viewport.YOffset()
	if before == 0 {
		t.Fatal("scrolling did not move the viewport")
	}

	m.SetSize(130, 40)

	if got := m.viewport.YOffset(); got != before {
		t.Fatalf("relayout reset scroll offset: got %d, want %d", got, before)
	}
}
