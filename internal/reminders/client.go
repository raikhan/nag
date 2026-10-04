package reminders

import (
	"sync"
	"time"

	ekreminders "github.com/BRO3886/go-eventkit/reminders"
)

type Client struct {
	ek *ekreminders.Client

	// completedMu guards the completed-count memo below. The query behind
	// it walks every completed reminder in the account, so a 2s sidebar
	// poll would otherwise re-fetch an ever-growing history.
	completedMu    sync.Mutex
	completedCount int
	completedAt    time.Time
	completedValid bool
}

func New() (*Client, error) {
	ek, err := ekreminders.New()
	if err != nil {
		return nil, err
	}
	return &Client{ek: ek}, nil
}

// completedCountTTL is how long a completed count stays fresh. Completion
// changes invalidate the memo directly, so this only bounds the cost of
// completions made outside nag.
const completedCountTTL = 5 * time.Second

// CompletedCount returns the number of completed reminders across all
// lists, memoized for completedCountTTL. A failed query keeps the last
// known count so the sidebar badge never flickers to zero.
func (c *Client) CompletedCount() int {
	c.completedMu.Lock()
	defer c.completedMu.Unlock()

	if c.completedValid && time.Since(c.completedAt) < completedCountTTL {
		return c.completedCount
	}
	items, err := c.ek.Reminders(ekreminders.WithCompleted(true))
	if err != nil {
		return c.completedCount
	}
	n := 0
	for _, r := range items {
		if r.Completed {
			n++
		}
	}
	c.completedCount = n
	c.completedAt = time.Now()
	c.completedValid = true
	return n
}

// InvalidateCompletedCount drops the memo so the next sidebar poll counts
// completions again instead of waiting out the TTL.
func (c *Client) InvalidateCompletedCount() {
	c.completedMu.Lock()
	c.completedValid = false
	c.completedMu.Unlock()
}
