package reminders

import (
	"reflect"
	"testing"
	"time"
)

func at(t *testing.T, year int, month time.Month, day, hour int) time.Time {
	t.Helper()
	return time.Date(year, month, day, hour, 0, 0, 0, time.Local)
}

func TestBucketForBoundaries(t *testing.T) {
	now := at(t, 2026, time.October, 5, 12)

	tests := []struct {
		name string
		at   time.Time
		want CompletedBucket
	}{
		{"earlier today", at(t, 2026, time.October, 5, 0), BucketToday},
		{"just before midnight", at(t, 2026, time.October, 4, 23), BucketYesterday},
		{"two days back starts previous 7 days", at(t, 2026, time.October, 3, 9), BucketPrevious7Days},
		{"seven days back stays in previous 7 days", at(t, 2026, time.September, 28, 9), BucketPrevious7Days},
		{"eight days back starts previous 30 days", at(t, 2026, time.September, 27, 9), BucketPrevious30Days},
		{"thirty days back is the last recent day", at(t, 2026, time.September, 5, 9), BucketPrevious30Days},
		{"thirty one days back leaves the recent window", at(t, 2026, time.September, 4, 9), BucketPrevious30Days},
		{"clock skew into the future", at(t, 2026, time.October, 6, 1), BucketToday},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BucketFor(tt.at, now); got != tt.want {
				t.Fatalf("BucketFor = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucketLabels(t *testing.T) {
	want := []string{"Today", "Yesterday", "Previous 7 Days", "Previous 30 Days"}
	for i, label := range want {
		if got := CompletedBucket(i).Label(); got != label {
			t.Fatalf("bucket %d label = %q, want %q", i, got, label)
		}
	}
}

// TestIsRecentWindow pins the boundary where the view stops using the
// day-based sections and starts filing by month and year.
func TestIsRecentWindow(t *testing.T) {
	now := at(t, 2026, time.October, 5, 12)
	if !IsRecent(at(t, 2026, time.September, 5, 23), now) {
		t.Fatal("30 days back must stay in the recent window")
	}
	if IsRecent(at(t, 2026, time.September, 4, 1), now) {
		t.Fatal("31 days back must leave the recent window")
	}
}

// completed builds a reminder completed at t and labelled by id.
func completed(id string, t time.Time) Reminder {
	when := t
	return Reminder{ID: id, Title: id, Completed: true, CompletionDate: &when}
}

func TestGroupCompletedSectionsInOrder(t *testing.T) {
	// Mid-September: the 30-day window reaches back into August, so that
	// month only keeps its remainder.
	now := at(t, 2026, time.September, 15, 12)
	items := []Reminder{
		completed("today-late", at(t, 2026, time.September, 15, 9)),
		completed("today-early", at(t, 2026, time.September, 15, 1)),
		completed("yesterday", at(t, 2026, time.September, 14, 20)),
		completed("last-week", at(t, 2026, time.September, 10, 20)),
		completed("last-month", at(t, 2026, time.August, 26, 20)),
		completed("aug-remainder", at(t, 2026, time.August, 10, 8)),
		completed("july", at(t, 2026, time.July, 4, 8)),
		completed("march", at(t, 2026, time.March, 25, 8)),
		completed("last-year", at(t, 2025, time.December, 2, 8)),
		completed("ancient", at(t, 2018, time.February, 8, 14)),
		// An open reminder and one with no completion date cannot be
		// placed in a section, so both are left out entirely.
		{ID: "open", Title: "open"},
		{ID: "undated", Title: "undated", Completed: true},
	}

	groups := GroupCompleted(items, now)

	var labels []string
	var ids []string
	for _, g := range groups {
		labels = append(labels, g.Label)
		for _, r := range g.Items {
			ids = append(ids, r.ID)
		}
	}
	want := []string{
		"Today", "Yesterday", "Previous 7 Days", "Previous 30 Days",
		"Rest of August", "July", "March", "2025", "2018",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("section labels = %v, want %v", labels, want)
	}
	wantIDs := []string{
		"today-late", "today-early", "yesterday", "last-week", "last-month",
		"aug-remainder", "july", "march", "last-year", "ancient",
	}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("row order = %v, want %v", ids, wantIDs)
	}

	flat := FlattenGroups(groups)
	if len(flat) != len(ids) {
		t.Fatalf("flattened rows = %d, want %d", len(flat), len(ids))
	}
	for i, r := range flat {
		if r.ID != ids[i] {
			t.Fatalf("flattened row %d = %s, want %s", i, r.ID, ids[i])
		}
	}
}

// TestGroupCompletedWholeMonthHasNoRestOfPrefix covers the months the
// 30-day window does not reach: they carry their bare name.
func TestGroupCompletedWholeMonthHasNoRestOfPrefix(t *testing.T) {
	now := at(t, 2026, time.October, 5, 12)
	groups := GroupCompleted([]Reminder{
		completed("august", at(t, 2026, time.August, 10, 8)),
	}, now)
	if len(groups) != 1 || groups[0].Label != "August" {
		t.Fatalf("sections = %+v, want a single bare August", groups)
	}
}

func TestGroupCompletedSkipsEmptySections(t *testing.T) {
	if groups := GroupCompleted(nil, at(t, 2026, time.October, 5, 12)); len(groups) != 0 {
		t.Fatalf("no completed reminders produced %d sections, want 0", len(groups))
	}
}

func TestSortByCompletionPutsUndatedLast(t *testing.T) {
	items := []Reminder{
		{ID: "undated", Completed: true},
		completed("old", at(t, 2026, time.September, 1, 8)),
		completed("new", at(t, 2026, time.October, 5, 8)),
	}
	SortByCompletion(items)
	if items[0].ID != "new" || items[1].ID != "old" || items[2].ID != "undated" {
		t.Fatalf("order = %s, %s, %s", items[0].ID, items[1].ID, items[2].ID)
	}
}
