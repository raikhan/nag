package reminders

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	ekreminders "github.com/BRO3886/go-eventkit/reminders"
)

// CompletedBucket is a completion-age group for the recent past, rendered
// as a section header above its reminders in the Completed view.
type CompletedBucket int

const (
	BucketToday CompletedBucket = iota
	BucketYesterday
	BucketPrevious7Days
	BucketPrevious30Days
)

// bucketDays is how many days past completion each bucket spans. The last
// entry also fixes the age beyond which sections switch to months and years.
const recentWindowDays = 30

func (b CompletedBucket) Label() string {
	switch b {
	case BucketToday:
		return "Today"
	case BucketYesterday:
		return "Yesterday"
	case BucketPrevious7Days:
		return "Previous 7 Days"
	default:
		return "Previous 30 Days"
	}
}

// BucketFor groups a completion timestamp by whole calendar days elapsed
// since it, so the boundaries match the user's wall clock rather than a
// rolling 24-hour window. Times in the future (clock skew) land in Today.
func BucketFor(completed, now time.Time) CompletedBucket {
	days := calendarDaysApart(completed, now)
	switch {
	case days <= 0:
		return BucketToday
	case days == 1:
		return BucketYesterday
	case days <= 7:
		return BucketPrevious7Days
	default:
		return BucketPrevious30Days
	}
}

// IsRecent reports whether a completion age still belongs to the day-based
// sections. Anything older is filed under its month or year instead, and
// renders its completion stamp as a full date.
func IsRecent(completed, now time.Time) bool {
	return calendarDaysApart(completed, now) <= recentWindowDays
}

// calendarDaysApart counts midnight-to-midnight days from t to ref in
// ref's location. Localising first keeps a completion timestamp recorded in
// another zone on the day the user actually finished the task.
func calendarDaysApart(t, ref time.Time) int {
	local := func(v time.Time) time.Time {
		v = v.In(ref.Location())
		return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, ref.Location())
	}
	return int(local(ref).Sub(local(t)).Hours() / 24)
}

// CompletedGroup is one section of the Completed view: a header label and
// the reminders completed inside it. An empty label means the section
// renders without a header, which is how the current month reads.
type CompletedGroup struct {
	Label string
	Items []Reminder
}

// monthKey identifies a calendar month section.
type monthKey struct {
	year  int
	month time.Month
}

// GroupCompleted splits completed reminders the way Apple Reminders does:
// the four day-based sections (Today, Yesterday, Previous 7 Days, Previous
// 30 Days), then the remaining months of the current year newest first, then
// each earlier year newest first. Newest completion comes first inside every
// section. Reminders without a completion date cannot be placed, so they are
// dropped.
func GroupCompleted(items []Reminder, now time.Time) []CompletedGroup {
	loc := now.Location()
	// Everything before this instant is too old for the day sections; the
	// month holding the boundary keeps only its remainder, which Apple
	// labels "Rest of <Month>".
	windowStart := startOfDay(now).AddDate(0, 0, -recentWindowDays)

	byBucket := make([][]Reminder, int(BucketPrevious30Days)+1)
	byMonth := make(map[monthKey][]Reminder)
	byYear := make(map[int][]Reminder)
	for _, r := range items {
		if !r.Completed || r.CompletionDate == nil {
			continue
		}
		when := r.CompletionDate.In(loc)
		switch {
		case IsRecent(*r.CompletionDate, now):
			b := BucketFor(*r.CompletionDate, now)
			byBucket[b] = append(byBucket[b], r)
		case when.Year() == now.Year():
			key := monthKey{year: when.Year(), month: when.Month()}
			byMonth[key] = append(byMonth[key], r)
		default:
			byYear[when.Year()] = append(byYear[when.Year()], r)
		}
	}

	groups := make([]CompletedGroup, 0, len(byYear)+len(byMonth)+len(byBucket))
	for b, bucketItems := range byBucket {
		groups = appendGroup(groups, CompletedBucket(b).Label(), bucketItems)
	}
	for _, key := range sortedMonthsDescending(byMonth) {
		label := key.month.String()
		if windowStart.Year() == key.year && windowStart.Month() == key.month {
			label = "Rest of " + label
		}
		groups = appendGroup(groups, label, byMonth[key])
	}
	for _, year := range sortedDescending(byYear) {
		groups = appendGroup(groups, strconv.Itoa(year), byYear[year])
	}
	return groups
}

func appendGroup(groups []CompletedGroup, label string, items []Reminder) []CompletedGroup {
	if len(items) == 0 {
		return groups
	}
	SortByCompletion(items)
	return append(groups, CompletedGroup{Label: label, Items: items})
}

func sortedMonthsDescending(byMonth map[monthKey][]Reminder) []monthKey {
	keys := make([]monthKey, 0, len(byMonth))
	for k := range byMonth {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year > keys[j].year
		}
		return keys[i].month > keys[j].month
	})
	return keys
}

func sortedDescending(byYear map[int][]Reminder) []int {
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	return years
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// FlattenGroups returns every grouped reminder in section order, which is
// the row order the Completed panel renders.
func FlattenGroups(groups []CompletedGroup) []Reminder {
	rows := make([]Reminder, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, g.Items...)
	}
	return rows
}

// CompletedReminders returns every completed reminder across all lists,
// newest completion first.
func (c *Client) CompletedReminders() ([]Reminder, error) {
	items, err := c.ek.Reminders(ekreminders.WithCompleted(true))
	if err != nil {
		return nil, err
	}
	result := make([]Reminder, 0, len(items))
	for _, r := range items {
		if !r.Completed || r.CompletionDate == nil {
			continue
		}
		result = append(result, convertReminder(r))
	}
	SortByCompletion(result)
	return result, nil
}

// SortByCompletion orders reminders by completion date, newest first.
// Reminders without one sort last.
func SortByCompletion(items []Reminder) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].CompletionDate, items[j].CompletionDate
		if a == nil || b == nil {
			return a != nil
		}
		return a.After(*b)
	})
}

// EnsureList resolves the title of the list with the given ID, re-creating
// it under title when it no longer exists. A completed reminder can outlive
// its list, so uncompleting has to restore the list before the reminder can
// return to it. It reports whether the list had to be re-created.
func (c *Client) EnsureList(id, title string) (string, bool, error) {
	lists, err := c.Lists()
	if err == nil {
		for _, l := range lists {
			if l.ID == id {
				return l.Title, false, nil
			}
		}
	}
	if title == "" {
		return "", false, fmt.Errorf("list %s no longer exists", id)
	}
	if err := c.CreateList(title, ""); err != nil {
		return "", false, err
	}
	return title, true, nil
}
