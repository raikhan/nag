package commands

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/ui/messages"
)

const RefreshInterval = 2 * time.Second

func AutoRefreshTick() tea.Cmd {
	return tea.Tick(RefreshInterval, func(time.Time) tea.Msg {
		return messages.TickMsg{}
	})
}
