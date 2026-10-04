package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// aceTarget identifies one jumpable visible row by its panel and stable ID.
type aceTarget struct {
	panel Panel
	id    string
	label string
}

// aceState is the root-owned ace-jump mode state. It snapshots the eligible
// visible rows when ace starts and tracks the typed label prefix.
type aceState struct {
	active  bool
	prefix  string
	targets []aceTarget
}

// aceLabels assigns deterministic, prefix-free labels to n targets using the
// configured alphabet: single letters when the alphabet covers every target,
// otherwise the minimum equal-length base-N digit strings.
func aceLabels(n int, alphabet string) []string {
	base := len([]rune(alphabet))
	letters := []rune(alphabet)
	digits := 1
	for pow := base; pow < n; pow *= base {
		digits++
	}
	labels := make([]string, n)
	for i := range labels {
		x := i
		buf := make([]rune, digits)
		for d := digits - 1; d >= 0; d-- {
			buf[d] = letters[x%base]
			x /= base
		}
		labels[i] = string(buf)
	}
	return labels
}

// aceTargets snapshots eligible rows: sidebar visible items top-to-bottom,
// then the current list's visible reminders only when the displayed reminder
// data belongs to the selected list and no fetch is in flight.
func (m *Model) aceTargets() []aceTarget {
	var targets []aceTarget
	for _, id := range m.listPanel.VisibleIDs() {
		targets = append(targets, aceTarget{panel: PanelLists, id: id})
	}
	if m.selectedList != nil && !m.loadingReminders && m.displayedListID == m.selectedList.ID {
		for _, id := range m.reminderPanel.VisibleIDs() {
			targets = append(targets, aceTarget{panel: PanelReminders, id: id})
		}
	}
	return targets
}

// aceStart enters ace mode, labelling every eligible visible row. With no
// targets it stays in normal mode and reports why.
func (m *Model) aceStart() {
	targets := m.aceTargets()
	if len(targets) == 0 {
		m.statusBar.SetInfo("No jump targets")
		return
	}
	labels := aceLabels(len(targets), m.aceAlphabet)
	sidebar := make(map[string]string)
	reminders := make(map[string]string)
	for i := range targets {
		targets[i].label = labels[i]
		switch targets[i].panel {
		case PanelLists:
			sidebar[targets[i].id] = labels[i]
		case PanelReminders:
			reminders[targets[i].id] = labels[i]
		}
	}
	m.listPanel.SetAceLabels(sidebar)
	m.reminderPanel.SetAceLabels(reminders)
	m.ace = aceState{active: true, targets: targets}
}

// aceExit leaves ace mode, clearing every delegate label. The selection is
// left unchanged unless a target was matched.
func (m *Model) aceExit() {
	if !m.ace.active {
		return
	}
	m.ace = aceState{}
	m.listPanel.SetAceLabels(nil)
	m.reminderPanel.SetAceLabels(nil)
}

// aceSetPrefix narrows the typed prefix, dimming nonmatching labels.
func (m *Model) aceSetPrefix(prefix string) {
	m.ace.prefix = prefix
	m.listPanel.SetAcePrefix(prefix)
	m.reminderPanel.SetAcePrefix(prefix)
}

// aceHandleKey consumes one key press while ace is active. Alphabet keys
// extend the prefix (a complete label jumps), backspace removes one letter
// (empty exits), cancel exits without changing the selection, and every
// other key does nothing.
func (m *Model) aceHandleKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, m.keys.Bind("ace", "cancel")) {
		m.aceExit()
		return nil
	}
	if key.Matches(msg, m.keys.Bind("ace", "backspace")) {
		if m.ace.prefix == "" {
			m.aceExit()
			return nil
		}
		runes := []rune(m.ace.prefix)
		m.aceSetPrefix(string(runes[:len(runes)-1]))
		return nil
	}
	if len(msg.Text) == 1 {
		ch := rune(msg.Text[0])
		if strings.ContainsRune(m.aceAlphabet, ch) {
			candidate := m.ace.prefix + string(ch)
			for _, t := range m.ace.targets {
				if t.label == candidate {
					return m.aceExecute(t)
				}
			}
			for _, t := range m.ace.targets {
				if strings.HasPrefix(t.label, candidate) {
					m.aceSetPrefix(candidate)
					return nil
				}
			}
			// Invalid extension: keep the previous prefix.
		}
	}
	return nil
}

// aceExecute jumps to the matched target and exits ace mode. Selecting a
// sidebar target loads that list while the sidebar stays focused; selecting
// a reminder target only moves the selection.
func (m *Model) aceExecute(t aceTarget) tea.Cmd {
	m.aceExit()
	switch t.panel {
	case PanelReminders:
		if !m.reminderPanel.SelectID(t.id) {
			return nil
		}
		m.setFocus(PanelReminders)
		return nil
	case PanelLists:
		if !m.listPanel.SelectID(t.id) {
			return nil
		}
		list, ok := m.listPanel.SelectedList()
		if !ok {
			return nil
		}
		m.setFocus(PanelLists)
		m.selectedList = &list
		m.pendingFocus = false
		m.reminderPanel.SetTitle(list.Title)
		m.reminderPanel.SetReminders(nil)
		m.displayedListID = ""
		m.statusBar.SetLoading("Loading reminders...")
		return m.fetchSelectedReminders()
	}
	return nil
}
