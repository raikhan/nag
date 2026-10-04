package reminders

import (
	"testing"
	"time"
)

// TestAlarmsEqual covers the order-insensitive alarm comparison the panel
// refresh and the edit form's change detection both rely on.
func TestAlarmsEqual(t *testing.T) {
	at := func(h, m int) *time.Time {
		v := time.Date(2026, time.October, 5, h, m, 0, 0, time.UTC)
		return &v
	}

	t.Run("nil and empty are equal", func(t *testing.T) {
		if !AlarmsEqual(nil, []Alarm{}) {
			t.Fatal("nil and empty alarm sets must compare equal")
		}
		if !AlarmsEqual([]Alarm{}, nil) {
			t.Fatal("empty and nil alarm sets must compare equal")
		}
	})

	t.Run("order is ignored", func(t *testing.T) {
		a := []Alarm{
			{RelativeOffset: -30 * time.Minute},
			{AbsoluteDate: at(9, 0)},
			{RelativeOffset: -time.Hour},
		}
		b := []Alarm{
			{RelativeOffset: -time.Hour},
			{RelativeOffset: -30 * time.Minute},
			{AbsoluteDate: at(9, 0)},
		}
		if !AlarmsEqual(a, b) {
			t.Fatalf("reordered sets must compare equal: %+v vs %+v", a, b)
		}
	})

	t.Run("different offsets differ", func(t *testing.T) {
		a := []Alarm{{RelativeOffset: -30 * time.Minute}}
		b := []Alarm{{RelativeOffset: -15 * time.Minute}}
		if AlarmsEqual(a, b) {
			t.Fatal("different offsets must not compare equal")
		}
	})

	t.Run("absolute and relative never match", func(t *testing.T) {
		a := []Alarm{{AbsoluteDate: at(9, 0)}}
		b := []Alarm{{RelativeOffset: -30 * time.Minute}}
		if AlarmsEqual(a, b) {
			t.Fatal("an absolute alarm must not equal a relative one")
		}
	})

	t.Run("different instants differ", func(t *testing.T) {
		if AlarmsEqual([]Alarm{{AbsoluteDate: at(9, 0)}}, []Alarm{{AbsoluteDate: at(9, 30)}}) {
			t.Fatal("different absolute instants must not compare equal")
		}
	})

	t.Run("length is compared", func(t *testing.T) {
		if AlarmsEqual([]Alarm{{RelativeOffset: -time.Hour}}, nil) {
			t.Fatal("a non-empty set must not equal nil")
		}
	})
}

// TestRemindersEqualSeesAlarmChanges guards the panel repaint: a poll that
// only changes an alert must count as a change, or the edit form would show
// None for a reminder that does have an alert.
func TestRemindersEqualSeesAlarmChanges(t *testing.T) {
	base := []Reminder{{
		ID:    "r",
		Title: "Buy milk",
		Alarms: []Alarm{
			{RelativeOffset: -30 * time.Minute},
		},
	}}
	changed := []Reminder{{
		ID:     "r",
		Title:  "Buy milk",
		Alarms: []Alarm{{RelativeOffset: -15 * time.Minute}},
	}}
	if RemindersEqual(base, changed) {
		t.Fatal("reminders differing only in alarms must not compare equal")
	}
	if !RemindersEqual(base, []Reminder{{
		ID:     "r",
		Title:  "Buy milk",
		Alarms: []Alarm{{RelativeOffset: -30 * time.Minute}},
	}}) {
		t.Fatal("identical alarms must compare equal")
	}
}
