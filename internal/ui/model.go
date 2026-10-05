package ui

import (
	"fmt"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui/commands"
	"github.com/oronbz/nag/internal/ui/components/dialog"
	"github.com/oronbz/nag/internal/ui/components/helpoverlay"
	"github.com/oronbz/nag/internal/ui/components/listpanel"
	"github.com/oronbz/nag/internal/ui/components/reminderpanel"
	"github.com/oronbz/nag/internal/ui/components/statusbar"
	"github.com/oronbz/nag/internal/ui/messages"
	"github.com/oronbz/nag/internal/ui/styles"
)

type Panel int

const (
	PanelLists Panel = iota
	PanelReminders
)

type Model struct {
	client      *reminders.Client
	keys        keybind.Map
	aceAlphabet string
	aceTimeout  time.Duration // negative disables the dedicated expiry timer
	// aceSelectListOnJump makes a list jump behave like Enter: load the
	// list and hand focus to its reminders once they arrive.
	aceSelectListOnJump bool

	listPanel     listpanel.Model
	reminderPanel reminderpanel.Model
	statusBar     statusbar.Model
	helpOverlay   helpoverlay.Model
	createDlg     dialog.CreateModel
	createListDlg dialog.CreateListModel
	confirmDlg    dialog.ConfirmModel
	spinner       spinner.Model

	focusedPanel  Panel
	selectedList  *reminders.ReminderList
	showCompleted bool
	sortMode      reminders.SortMode
	layout        Layout
	pendingFocus  bool
	width         int
	height        int
	ready         bool

	// displayedListID records the list whose reminders are actually shown;
	// loadingReminders tracks an in-flight reminder fetch so ace targets
	// never snapshot data that is about to be replaced.
	displayedListID  string
	loadingReminders bool
	ace              aceState
	// aceGeneration counts ace snapshots; it lives outside aceState because
	// aceState resets on exit and the generation must survive to invalidate
	// stale expiry ticks.
	aceGeneration uint64

	// lists is the last loaded list catalogue, handed to the create form so
	// its list picker has options.
	lists []reminders.ReminderList
	// clock is injectable so double-click timing is deterministic in tests.
	clock func() time.Time
	// lastClickID/lastClickAt drive double-click detection.
	lastClickID string
	lastClickAt time.Time
	// dragID/dragActive/dropID track a reminder drag between panels.
	dragID     string
	dragActive bool
	dropID     string

	// createOnly is the `nag --create` mode: only the create form renders.
	createOnly bool
	// createOnlyList is the optional `--create [list]` preselection.
	createOnlyList string
	// createOpened guards the one-shot form opening in createOnly mode.
	createOpened bool
}

func NewModel(client *reminders.Client, keys keybind.Map, aceAlphabet string, aceTimeoutSeconds int64, aceSelectListOnJump bool) Model {
	m := newBaseModel(client, keys)
	m.aceAlphabet = aceAlphabet
	m.aceTimeout = time.Duration(aceTimeoutSeconds) * time.Second
	m.aceSelectListOnJump = aceSelectListOnJump
	return m
}

// NewCreateModel builds the `nag --create` model: the create form alone,
// preselecting listTitle when given. There is no ace alphabet or ace
// timeout because a lone form has nothing to jump to.
func NewCreateModel(client *reminders.Client, keys keybind.Map, listTitle string) Model {
	m := newBaseModel(client, keys)
	m.createOnly = true
	m.createOnlyList = listTitle
	return m
}

func newBaseModel(client *reminders.Client, keys keybind.Map) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SpinnerStyle

	lp := listpanel.New(30, 20)
	lp.SetFocused(true)

	m := Model{
		client:        client,
		keys:          keys,
		clock:         time.Now,
		listPanel:     lp,
		reminderPanel: reminderpanel.New(50, 20),
		statusBar:     statusbar.New(),
		helpOverlay:   helpoverlay.New(),
		createDlg:     dialog.NewCreate(),
		createListDlg: dialog.NewCreateList(),
		confirmDlg:    dialog.NewConfirm(),
		spinner:       s,
		focusedPanel:  PanelLists,
	}
	m.listPanel.SetKeys(keys)
	m.reminderPanel.SetKeys(keys)
	m.statusBar.SetKeys(keys)
	m.helpOverlay.SetKeys(keys)
	m.createDlg.SetKeys(keys)
	m.createListDlg.SetKeys(keys)
	m.confirmDlg.SetKeys(keys)
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{commands.FetchLists(m.client)}
	if !m.createOnly {
		// A lone create form has nothing to poll.
		cmds = append(cmds, commands.AutoRefreshTick())
	}
	cmds = append(cmds, m.spinner.Tick, func() tea.Msg { return tea.RequestBackgroundColor() })
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		isDark := msg.IsDark()
		m.listPanel.SetDarkBackground(isDark)
		m.reminderPanel.SetDarkBackground(isDark)
		return m, nil

	case aceTimeoutMsg:
		// Expiry fires once per activation and only cancels the snapshot that
		// scheduled it; stale timers after cancel/re-entry, and any message
		// when no timer is configured, are ignored.
		if m.ace.active && m.aceTimeout >= 0 && msg.generation == m.aceGeneration {
			m.aceExit()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.aceExit()
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resize()
		return m, nil

	case tea.MouseMsg:
		m.aceExit()
		if m.createDlg.Visible() || m.createListDlg.Visible() || m.confirmDlg.Visible() || m.helpOverlay.Visible() {
			break
		}
		mouse := msg.Mouse()
		// Motion is drag handling only: a press with the button held moves
		// the drop highlight and must never reach the panels as a hover.
		if motion, ok := msg.(tea.MouseMotionMsg); ok {
			if m.dragID != "" && motion.Button != tea.MouseNone {
				m.trackDrag(mouse.X, mouse.Y)
				return m, nil
			}
			break
		}
		target, ok := m.panelForMouse(mouse.X, mouse.Y)
		if !ok {
			break
		}
		switch msg := msg.(type) {
		case tea.MouseClickMsg:
			if msg.Button == tea.MouseLeft {
				m.setFocus(target)
				cmd := m.selectRowAt(target, mouse.Y)
				return m, cmd
			}
		case tea.MouseReleaseMsg:
			if msg.Button == tea.MouseLeft {
				cmd := m.dropDraggedReminder()
				return m, cmd
			}
		case tea.MouseWheelMsg:
			if msg.Button == tea.MouseWheelUp || msg.Button == tea.MouseWheelDown {
				var wheelCmds []tea.Cmd
				var cmd tea.Cmd
				switch target {
				case PanelLists:
					m.listPanel, cmd = m.listPanel.Update(msg)
					wheelCmds = append(wheelCmds, cmd)
					if c := m.syncSelectedList(false); c != nil {
						wheelCmds = append(wheelCmds, c)
					}
				case PanelReminders:
					m.reminderPanel, cmd = m.reminderPanel.Update(msg)
					wheelCmds = append(wheelCmds, cmd)
				}
				return m, tea.Batch(wheelCmds...)
			}
		}

	case clearInfoMsg:
		m.statusBar.ClearInfo()
		return m, nil

	case delayedRefreshMsg:
		return m, m.fetchSelectedReminders()

	case openFailedMsg:
		m.statusBar.SetError("Open failed: " + msg.err.Error())
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case list.FilterMatchesMsg:
		// A filter result changes the displayed rows: ace mode ends.
		m.aceExit()

	case messages.ListsLoadedMsg:
		// A poll that returns the same sidebar rows changes nothing on
		// screen, so an in-flight ace jump survives it.
		if !listsEqual(m.listPanel.Lists(), msg.Lists) {
			m.aceExit()
		}
		m.statusBar.ClearLoading()
		if m.createOnly && !m.createOpened && msg.Err == nil {
			m.createOpened = true
			m.lists = msg.Lists
			m.createDlg.Show(preselectedListTitle(msg.Lists, m.createOnlyList), msg.Lists)
			return m, nil
		}
		if msg.Err != nil {
			m.statusBar.SetError(msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.lists = msg.Lists
		m.listPanel.SetLists(msg.Lists)
		m.resize()
		// Populate the right panel with the initially highlighted list.
		if m.selectedList == nil {
			if c := m.syncSelectedList(false); c != nil {
				return m, c
			}
		}
		return m, nil

	case messages.RemindersLoadedMsg:
		// Ignore results for a list other than the selected one so a slow
		// older fetch cannot clobber the display, loading or focus state.
		if m.selectedList == nil || msg.ListID != m.selectedList.ID {
			return m, nil
		}
		m.loadingReminders = false
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError(msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.displayedListID = msg.ListID
		rows := msg.Reminders
		var groups []reminders.CompletedGroup
		if msg.ListID == reminders.SmartListCompleted {
			// The Completed view is ordered by completion date, so the
			// sort modes never apply to it.
			groups = reminders.GroupCompleted(msg.Reminders, time.Now())
			rows = reminders.FlattenGroups(groups)
		} else if m.sortMode != reminders.SortDefault {
			reminders.ApplySort(rows, m.sortMode)
		}
		if reminders.RemindersEqual(rows, m.reminderPanel.Reminders()) {
			// Server-side state is unchanged: a 2s poll must not churn the
			// panel. Focus requests still complete.
			if m.pendingFocus {
				m.pendingFocus = false
				m.cycleFocus(1)
			}
			return m, nil
		}
		// Only a real change to the visible rows ends the ace jump; the
		// unchanged-poll early return above keeps it alive.
		m.aceExit()
		if msg.ListID == reminders.SmartListCompleted {
			m.reminderPanel.SetGroups(groups)
		} else {
			m.reminderPanel.SetReminders(rows)
		}
		m.resize()
		if m.pendingFocus {
			m.pendingFocus = false
			m.cycleFocus(1)
		}
		return m, nil

	case messages.ReminderCreatedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			errText := "Create failed: " + msg.Err.Error()
			m.statusBar.SetError(errText)
			if m.createOnly {
				m.createDlg.ReopenAfterError(errText)
			}
			return m, nil
		}
		if m.createOnly {
			// The one-shot mode exists to add a reminder; the job is done.
			return m, tea.Quit
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("Reminder created")
		cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }))
		cmds = append(cmds, commands.FetchLists(m.client))
		if cmd := m.fetchSelectedReminders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case messages.ReminderCompletedMsg:
		if msg.Err != nil {
			m.statusBar.SetError("Toggle failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		if msg.Reminder != nil {
			m.reminderPanel.UpdateReminder(*msg.Reminder)
		}
		// Refresh lists immediately for count update; delay reminder re-fetch
		// so the user sees the toggle and can undo with Space.
		if m.showingCompleted() {
			// An uncompleted task leaves this view for good, so there is
			// no state on screen worth holding on to for the grace period.
			m.statusBar.SetInfo("Reminder uncompleted")
			return m, tea.Batch(
				commands.FetchLists(m.client),
				m.fetchSelectedReminders(),
				tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }),
			)
		}
		return m, tea.Batch(
			commands.FetchLists(m.client),
			tea.Tick(2*time.Second, func(time.Time) tea.Msg { return delayedRefreshMsg{} }),
		)

	case messages.ReminderDeletedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Delete failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("Reminder deleted")
		cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }))
		cmds = append(cmds, commands.FetchLists(m.client))
		if cmd := m.fetchSelectedReminders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case messages.TickMsg:
		batch := []tea.Cmd{commands.AutoRefreshTick(), commands.FetchLists(m.client)}
		if cmd := m.fetchSelectedReminders(); cmd != nil {
			batch = append(batch, cmd)
		}
		return m, tea.Batch(batch...)

	case dialog.CreateSubmitMsg:
		m.statusBar.SetLoading("Creating reminder...")
		return m, commands.CreateReminder(m.client, msg.Input)

	case dialog.EditSubmitMsg:
		m.statusBar.SetLoading("Updating reminder...")
		return m, commands.UpdateReminder(m.client, msg.ID, msg.Input)

	case messages.ReminderUpdatedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Update failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("Reminder updated")
		cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }))
		cmds = append(cmds, commands.FetchLists(m.client))
		if cmd := m.fetchSelectedReminders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case messages.ReminderMovedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Move failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("Moved to \"" + msg.ListTitle + "\"")
		cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }))
		cmds = append(cmds, commands.FetchLists(m.client))
		if cmd := m.fetchSelectedReminders(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case dialog.CreateListSubmitMsg:
		m.statusBar.SetLoading("Creating list...")
		return m, commands.CreateList(m.client, msg.Title, msg.Color)

	case dialog.EditListSubmitMsg:
		m.statusBar.SetLoading("Updating list...")
		return m, commands.UpdateList(m.client, msg.ID, msg.Title, msg.Color)

	case messages.ListUpdatedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Update list failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("List updated")
		return m, tea.Batch(
			commands.FetchLists(m.client),
			tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }),
		)

	case messages.ListCreatedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Create list failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("List created")
		return m, tea.Batch(
			commands.FetchLists(m.client),
			tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }),
		)

	case messages.ListDeletedMsg:
		m.statusBar.ClearLoading()
		if msg.Err != nil {
			m.statusBar.SetError("Delete list failed: " + msg.Err.Error())
			return m, nil
		}
		m.statusBar.ClearError()
		m.statusBar.SetInfo("List deleted")
		m.selectedList = nil
		m.reminderPanel.SetReminders(nil)
		m.reminderPanel.SetTitle("")
		return m, tea.Batch(
			commands.FetchLists(m.client),
			tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }),
		)

	case dialog.ConfirmYesMsg:
		switch msg.Action {
		case dialog.ConfirmDelete:
			if r, ok := m.reminderPanel.SelectedReminder(); ok {
				m.statusBar.SetLoading("Deleting reminder...")
				return m, commands.DeleteReminder(m.client, r.ID)
			}
		case dialog.ConfirmDeleteList:
			if list, ok := m.listPanel.SelectedList(); ok {
				m.statusBar.SetLoading("Deleting list...")
				return m, commands.DeleteList(m.client, list.ID)
			}
		}
		return m, nil

	case dialog.ConfirmNoMsg:
		return m, nil
	}

	// `nag --create` has no panels to route keys to. Quitting works with
	// the form open too, but only through a modified alias: a bare "q"
	// must stay typable in the title field.
	if m.createOnly {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok &&
			key.Matches(keyMsg, m.keys.Bind("global", "quit")) {
			if !m.createDlg.Visible() || keyMsg.Mod != 0 {
				return m, tea.Quit
			}
		}
	}

	// Route to dialogs/overlays first if visible
	if m.createDlg.Visible() {
		var cmd tea.Cmd
		m.createDlg, cmd = m.createDlg.Update(msg)
		if m.createOnly && !m.createDlg.Visible() {
			if cmd != nil {
				// A successful save hides the form and hands back the
				// submit command. Quitting here would drop it and the
				// reminder would never reach Reminders; ReminderCreatedMsg
				// exits this mode once the create has run.
				return m, cmd
			}
			// Esc closed the form and this mode has nothing behind it.
			return m, tea.Quit
		}
		return m, cmd
	}
	if m.createListDlg.Visible() {
		var cmd tea.Cmd
		m.createListDlg, cmd = m.createListDlg.Update(msg)
		return m, cmd
	}
	if m.confirmDlg.Visible() {
		var cmd tea.Cmd
		m.confirmDlg, cmd = m.confirmDlg.Update(msg)
		return m, cmd
	}
	if m.helpOverlay.Visible() {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(keyMsg, m.keys.Bind("help", "close")) {
			m.helpOverlay.Toggle()
			return m, nil
		}
		var cmd tea.Cmd
		m.helpOverlay, cmd = m.helpOverlay.Update(msg)
		return m, cmd
	}

	// Global keys (skip when filtering)
	filtering := m.listPanel.Filtering() || m.reminderPanel.Filtering()
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && !filtering {
		// Active ace mode captures keys before every other shortcut.
		if m.ace.active {
			cmd := m.aceHandleKey(keyMsg)
			return m, cmd
		}
		switch {
		case key.Matches(keyMsg, m.keys.Bind("global", "quit")):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Bind("global", "ace_jump")):
			cmd := m.aceStart()
			return m, cmd
		case key.Matches(keyMsg, m.keys.Bind("global", "help")):
			m.helpOverlay.Toggle()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "next_panel")):
			m.cycleFocus(1)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "previous_panel")):
			m.cycleFocus(-1)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "focus_left")):
			m.setFocus(PanelLists)
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "focus_right")):
			m.setFocus(PanelReminders)
			return m, nil
		case m.focusedPanel == PanelLists && key.Matches(keyMsg, m.keys.Bind("list", "select")):
			return m, m.syncSelectedList(true)
		case key.Matches(keyMsg, m.keys.Bind("global", "toggle_complete")):
			return m, m.handleToggleComplete()
		case key.Matches(keyMsg, m.keys.Bind("global", "new")):
			m.handleNew()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "edit")):
			m.handleEdit()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "delete")):
			m.handleDelete()
			return m, nil
		case key.Matches(keyMsg, m.keys.Bind("global", "open_in_app")):
			return m, m.handleOpenInApp()
		case key.Matches(keyMsg, m.keys.Bind("global", "sort")):
			if m.showingCompleted() {
				m.statusBar.SetInfo("Completed is always sorted by completion date")
				return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} })
			}
			m.sortMode = m.sortMode.Next()
			m.statusBar.SetSortLabel(m.sortMode.Label())
			items := m.reminderPanel.Reminders()
			reminders.ApplySort(items, m.sortMode)
			m.reminderPanel.SetReminders(items)
			m.statusBar.SetInfo("Sort: " + m.sortMode.Label())
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} })
		case key.Matches(keyMsg, m.keys.Bind("global", "show_completed")):
			if m.showingCompleted() {
				m.statusBar.SetInfo("This view only shows completed reminders")
				return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} })
			}
			m.showCompleted = !m.showCompleted
			if m.showCompleted {
				m.statusBar.SetInfo("Showing completed")
			} else {
				m.statusBar.SetInfo("Hiding completed")
			}
			return m, tea.Batch(m.fetchSelectedReminders(), tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearInfoMsg{} }))
		case key.Matches(keyMsg, m.keys.Bind("global", "refresh")):
			return m, m.handleRefresh()
		}
	}

	// Route to focused panel
	switch m.focusedPanel {
	case PanelLists:
		var cmd tea.Cmd
		m.listPanel, cmd = m.listPanel.Update(msg)
		cmds = append(cmds, cmd)
		// Sidebar navigation loads the highlighted list without Enter.
		if c := m.syncSelectedList(false); c != nil {
			cmds = append(cmds, c)
		}
	case PanelReminders:
		var cmd tea.Cmd
		m.reminderPanel, cmd = m.reminderPanel.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) viewContent() string {
	if !m.ready {
		return m.spinner.View() + " Loading..."
	}

	// `nag --create` renders the form alone: no sidebar, no panels.
	if m.createOnly {
		if !m.createOpened {
			return m.spinner.View() + " Loading lists..."
		}
		return m.createDlg.View()
	}

	if m.helpOverlay.Visible() {
		return m.helpOverlay.View()
	}
	if m.createDlg.Visible() {
		return m.createDlg.View()
	}
	if m.createListDlg.Visible() {
		return m.createListDlg.View()
	}
	if m.confirmDlg.Visible() {
		return m.confirmDlg.View()
	}

	l := m.layout
	listsPanel := m.renderPanel(m.listPanel.View(), l.ListsWidth, l.PanelHeight, m.focusedPanel == PanelLists)
	remindersPanel := m.renderPanel(m.reminderPanel.View(), l.RemindersWidth, l.PanelHeight, m.focusedPanel == PanelReminders)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, listsPanel, remindersPanel)
	return lipgloss.JoinVertical(lipgloss.Left, panels, m.statusBar.View())
}

func (m Model) View() tea.View {
	v := tea.NewView(m.viewContent())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderPanel(content string, width, height int, focused bool) string {
	style := styles.UnfocusedBorder
	if focused {
		style = styles.FocusedBorder
	}
	return style.Width(width).Height(height).MaxHeight(height + 2).Render(content)
}

func (m *Model) resize() {
	m.layout = ComputeLayout(m.width, m.height)
	// Panel border styles consume 2 columns horizontally, so the lists must
	// be sized to the inner content width, not the outer panel width.
	m.listPanel.SetSize(m.layout.ListsWidth-2, m.layout.PanelHeight)
	m.reminderPanel.SetSize(m.layout.RemindersWidth-2, m.layout.PanelHeight)
	m.helpOverlay.SetSize(m.width, m.height)
	m.createDlg.SetSize(m.width, m.height)
	m.createListDlg.SetSize(m.width, m.height)
	m.confirmDlg.SetSize(m.width, m.height)
}

func (m *Model) setFocus(panel Panel) {
	if m.focusedPanel == panel {
		return
	}
	m.listPanel.SetFocused(false)
	m.reminderPanel.SetFocused(false)

	m.focusedPanel = panel
	switch panel {
	case PanelLists:
		m.listPanel.SetFocused(true)
		m.statusBar.SetPanel(statusbar.PanelLists)
	case PanelReminders:
		m.reminderPanel.SetFocused(true)
		m.statusBar.SetPanel(statusbar.PanelReminders)
	}
}

// panelForMouse maps a terminal column to a panel. Rows outside the
// rounded border (the status bar below, the top border above) belong to
// no panel, so a wheel or click there is ignored.
func (m Model) panelForMouse(x, y int) (Panel, bool) {
	l := m.layout
	if y < 1 || y > l.PanelHeight {
		return PanelLists, false
	}
	if x < l.ListsWidth+2 {
		return PanelLists, true
	}
	return PanelReminders, true
}

func (m *Model) cycleFocus(dir int) {
	m.listPanel.SetFocused(false)
	m.reminderPanel.SetFocused(false)

	m.focusedPanel = Panel((int(m.focusedPanel) + dir + 2) % 2)

	switch m.focusedPanel {
	case PanelLists:
		m.listPanel.SetFocused(true)
		m.statusBar.SetPanel(statusbar.PanelLists)
	case PanelReminders:
		m.reminderPanel.SetFocused(true)
		m.statusBar.SetPanel(statusbar.PanelReminders)
	}
}

// syncSelectedList loads the currently highlighted sidebar list into the
// reminder panel. focusAfterLoad moves focus to the reminders panel once
// loaded (the Enter behavior); navigation passes false.
func (m *Model) syncSelectedList(focusAfterLoad bool) tea.Cmd {
	list, ok := m.listPanel.SelectedList()
	if !ok {
		return nil
	}
	if m.selectedList != nil && m.selectedList.ID == list.ID {
		if focusAfterLoad {
			m.pendingFocus = true
			return m.fetchSelectedReminders() // Enter on the open list: reload + focus
		}
		return nil
	}
	m.selectedList = &list
	m.pendingFocus = focusAfterLoad
	m.reminderPanel.SetTitle(list.Title)
	m.statusBar.SetLoading("Loading reminders...")
	return m.fetchSelectedReminders()
}

// listsEqual compares the identity and label of each sidebar row. Reminder
// counts move on every completion and must not cancel an in-flight ace jump.
func listsEqual(a, b []reminders.ReminderList) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Title != b[i].Title || a[i].Kind != b[i].Kind {
			return false
		}
	}
	return true
}

func (m *Model) fetchSelectedReminders() tea.Cmd {
	if m.selectedList == nil {
		m.loadingReminders = false
		return nil
	}
	m.loadingReminders = true
	switch m.selectedList.ID {
	case reminders.SmartListToday:
		return commands.FetchTodayReminders(m.client, m.showCompleted)
	case reminders.SmartListScheduled:
		return commands.FetchScheduledReminders(m.client, m.showCompleted)
	case reminders.SmartListCompleted:
		return commands.FetchCompletedReminders(m.client)
	default:
		return commands.FetchReminders(m.client, m.selectedList.ID, m.selectedList.Title, m.showCompleted)
	}
}

// showingCompleted reports whether the right panel currently holds the
// cross-list Completed view, whose rows are all completed reminders.
func (m Model) showingCompleted() bool {
	return m.selectedList != nil && m.selectedList.ID == reminders.SmartListCompleted
}

func (m *Model) handleToggleComplete() tea.Cmd {
	if m.focusedPanel != PanelReminders {
		return nil
	}
	r, ok := m.reminderPanel.SelectedReminder()
	if !ok {
		return nil
	}
	if m.showingCompleted() {
		// Every row here is completed, so the toggle always uncompletes,
		// and the reminder goes back to the list it was completed in.
		return commands.UncompleteFromCompleted(m.client, r.ID, r.ListID, r.ListTitle)
	}
	return commands.ToggleComplete(m.client, r.ID, r.Completed)
}

func (m *Model) handleNew() {
	switch m.focusedPanel {
	case PanelLists:
		m.createListDlg.Show()
	case PanelReminders:
		if m.selectedList == nil {
			m.statusBar.SetInfo("Select a list first")
			return
		}
		if m.selectedList.Kind == reminders.ListSmart {
			m.statusBar.SetInfo("Select a regular list to create reminders")
			return
		}
		m.createDlg.Show(m.selectedList.Title, m.lists)
	}
}

func (m *Model) handleEdit() {
	switch m.focusedPanel {
	case PanelLists:
		list, ok := m.listPanel.SelectedList()
		if !ok {
			return
		}
		if list.Kind == reminders.ListSmart || list.Kind == reminders.ListSeparator {
			m.statusBar.SetInfo("Cannot edit smart lists")
			return
		}
		m.createListDlg.ShowEdit(list.ID, list.Title, list.Color)
	case PanelReminders:
		if r, ok := m.reminderPanel.SelectedReminder(); ok {
			m.createDlg.ShowEdit(r, m.lists)
		}
	}
}

func (m *Model) handleDelete() {
	switch m.focusedPanel {
	case PanelLists:
		list, ok := m.listPanel.SelectedList()
		if !ok {
			return
		}
		if list.Kind == reminders.ListSmart || list.Kind == reminders.ListSeparator {
			m.statusBar.SetInfo("Cannot delete smart lists")
			return
		}
		m.confirmDlg.Show(
			fmt.Sprintf("Delete list \"%s\" and all its reminders?", list.Title),
			dialog.ConfirmDeleteList,
		)
	case PanelReminders:
		if r, ok := m.reminderPanel.SelectedReminder(); ok {
			m.confirmDlg.Show(
				fmt.Sprintf("Delete \"%s\"?", r.Title),
				dialog.ConfirmDelete,
			)
		}
	}
}

func (m *Model) handleRefresh() tea.Cmd {
	m.statusBar.SetLoading("Refreshing...")
	batch := []tea.Cmd{commands.FetchLists(m.client)}
	if cmd := m.fetchSelectedReminders(); cmd != nil {
		batch = append(batch, cmd)
	}
	return tea.Batch(batch...)
}

func (m *Model) handleOpenInApp() tea.Cmd {
	if m.focusedPanel != PanelReminders {
		return nil
	}
	r, ok := m.reminderPanel.SelectedReminder()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		err := exec.Command("open", "x-apple-reminderkit://REMCDReminder/"+r.ID).Run()
		if err != nil {
			return openFailedMsg{err}
		}
		return nil
	}
}

// doubleClickWindow is how long two clicks on the same row count as a
// double click.
const doubleClickWindow = 500 * time.Millisecond

// IDAtMouseRow maps a terminal row inside a panel to the stable ID of the
// row rendered there. Row 0 is the panel's top border and row
// PanelHeight+1 its bottom one, so neither can hit a row.
func (m *Model) IDAtMouseRow(panel Panel, y int) (string, bool) {
	if y < 1 || y > m.layout.PanelHeight {
		return "", false
	}
	if panel == PanelLists {
		return m.listPanel.IDAtContentRow(y - 1)
	}
	return m.reminderPanel.IDAtContentRow(y - 1)
}

// selectRowAt moves the panel selection to the row under the cursor. A
// sidebar hit also loads that list; a reminder hit arms both the drag and
// the double-click detector, and opens the editor on a double click.
func (m *Model) selectRowAt(target Panel, y int) tea.Cmd {
	id, ok := m.IDAtMouseRow(target, y)
	if !ok {
		return nil
	}
	if target == PanelLists {
		if !m.listPanel.SelectID(id) {
			return nil
		}
		if m.openDoubleClicked(PanelLists, id) {
			return nil
		}
		m.lastClickID = id
		m.lastClickAt = m.clock()
		return m.syncSelectedList(false)
	}
	if !m.reminderPanel.SelectID(id) {
		return nil
	}
	m.dragID = id
	if m.openDoubleClicked(PanelReminders, id) {
		// An editor opened, so the press is not a drag.
		m.dragID = ""
		return nil
	}
	m.lastClickID = id
	m.lastClickAt = m.clock()
	return nil
}

// openDoubleClicked opens the matching editor when the same row is clicked
// twice inside doubleClickWindow. It reports whether it opened anything.
func (m *Model) openDoubleClicked(panel Panel, id string) bool {
	if m.lastClickID != id || m.clock().Sub(m.lastClickAt) > doubleClickWindow {
		return false
	}
	m.lastClickID = ""
	if panel == PanelLists {
		list, ok := findList(m.lists, id)
		if !ok {
			return false
		}
		m.createListDlg.ShowEdit(list.ID, list.Title, list.Color)
		return true
	}
	r, ok := findReminder(m.reminderPanel.Reminders(), id)
	if !ok {
		return false
	}
	m.createDlg.ShowEdit(r, m.lists)
	return true
}

// trackDrag turns a held-button motion into a drag and recomputes the drop
// target. Only normal lists accept a reminder, so smart lists and the
// separator never highlight.
func (m *Model) trackDrag(x, y int) {
	m.dragActive = true
	m.dropID = ""
	if panel, ok := m.panelForMouse(x, y); ok && panel == PanelLists {
		if id, ok := m.IDAtMouseRow(panel, y); ok {
			if list, ok := findList(m.lists, id); ok && list.Kind == reminders.ListNormal {
				m.dropID = id
			}
		}
	}
	m.listPanel.SetDropTarget(m.dropID)
	m.reminderPanel.SetDragging(m.dragID)
}

// dropDraggedReminder moves the dragged reminder onto the highlighted list.
// A press that never moved never armed dragActive, so a plain click cannot
// move anything.
func (m *Model) dropDraggedReminder() tea.Cmd {
	dragID, active, dropID := m.dragID, m.dragActive, m.dropID
	m.dragID, m.dragActive, m.dropID = "", false, ""
	m.listPanel.SetDropTarget("")
	m.reminderPanel.SetDragging("")
	if !active || dropID == "" || dragID == "" {
		return nil
	}
	r, ok := findReminder(m.reminderPanel.Reminders(), dragID)
	if !ok || r.ListID == dropID {
		return nil
	}
	list, ok := findList(m.lists, dropID)
	if !ok {
		return nil
	}
	m.statusBar.SetLoading("Moving reminder...")
	return commands.MoveReminder(m.client, dragID, list.Title)
}

// preselectedListTitle resolves a `--create [list]` argument against the
// loaded catalogue. An unknown title yields "" so the picker opens.
func preselectedListTitle(lists []reminders.ReminderList, title string) string {
	if title == "" {
		return ""
	}
	for _, l := range lists {
		if l.Title == title {
			return l.Title
		}
	}
	return ""
}

func findList(lists []reminders.ReminderList, id string) (reminders.ReminderList, bool) {
	for _, l := range lists {
		if l.ID == id {
			return l, true
		}
	}
	return reminders.ReminderList{}, false
}

func findReminder(items []reminders.Reminder, id string) (reminders.Reminder, bool) {
	for _, r := range items {
		if r.ID == id {
			return r, true
		}
	}
	return reminders.Reminder{}, false
}

type clearInfoMsg struct{}
type delayedRefreshMsg struct{}
type openFailedMsg struct{ err error }
