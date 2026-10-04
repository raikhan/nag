package dialog

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDbgLayout(t *testing.T) {
	loc, _ := time.LoadLocation("Australia/Perth")
	now := time.Date(2026, time.October, 4, 13, 15, 0, 0, loc)
	m := newVisibleCreate(t, now, 60, 24)
	for i, l := range stripLines(m.View()) {
		if strings.Contains(l, " │ ") {
			fmt.Printf("line %d: %q\n", i, l)
		}
	}
}
