// Package keybind holds the single ordered registry of nag's key actions and
// compiles user configuration (scope -> action -> alias strings) into Bubble
// Tea key bindings.
package keybind

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
)

// Bindings maps scope name -> action name -> alias list, as decoded from the
// user's TOML config. A present-but-empty alias list disables the action.
type Bindings map[string]map[string][]string

// Map is the compiled form consumed by the UI: scope -> action -> binding.
type Map map[string]map[string]key.Binding

// DefaultAceAlphabet is the default letter set used for ace-jump labels.
const DefaultAceAlphabet = "asdfghjklqwertyuiopzxcvbnm"

// Action describes one registered key action.
type Action struct {
	Scope   string
	Name    string
	Default []string
	Help    string
}

var (
	registryOnce sync.Once
	registryActs []Action
)

// Registry returns every action in deterministic registration order. Scopes
// appear in the order listed below; help and config rendering rely on it.
func Registry() []Action {
	registryOnce.Do(func() {
		registryActs = []Action{
			// global
			{"global", "quit", []string{"q", "ctrl+c"}, "Quit"},
			{"global", "help", []string{"?"}, "Toggle help"},
			{"global", "next_panel", []string{"tab"}, "Focus next panel"},
			{"global", "previous_panel", []string{"shift+tab"}, "Focus previous panel"},
			{"global", "focus_left", []string{"h"}, "Focus lists panel"},
			{"global", "focus_right", []string{"l"}, "Focus reminders panel"},
			{"global", "toggle_complete", []string{"space", "x"}, "Toggle reminder complete"},
			{"global", "new", []string{"n"}, "New reminder or list"},
			{"global", "edit", []string{"enter", "e"}, "Edit reminder or list"},
			{"global", "delete", []string{"d"}, "Delete reminder or list"},
			{"global", "open_in_app", []string{"o"}, "Open in Reminders app"},
			{"global", "show_completed", []string{"c"}, "Toggle completed reminders"},
			{"global", "sort", []string{"s"}, "Cycle sort order"},
			{"global", "refresh", []string{"r"}, "Refresh"},
			{"global", "ace_jump", []string{"z"}, "Jump to visible row"},
			// list
			{"list", "select", []string{"enter"}, "Select list"},
			{"list", "up", []string{"up", "k"}, "Move up"},
			{"list", "down", []string{"down", "j"}, "Move down"},
			{"list", "first", []string{"home", "g"}, "Jump to first"},
			{"list", "last", []string{"end", "G"}, "Jump to last"},
			{"list", "previous_page", []string{"left", "pgup", "b", "u"}, "Previous page"},
			{"list", "next_page", []string{"right", "pgdown", "f"}, "Next page"},
			{"list", "half_page_up", []string{"ctrl+u"}, "Half page up"},
			{"list", "half_page_down", []string{"ctrl+d"}, "Half page down"},
			{"list", "filter", []string{"/"}, "Filter"},
			{"list", "clear_filter", []string{"esc"}, "Clear filter"},
			// filter
			{"filter", "accept", []string{"enter", "tab", "shift+tab", "ctrl+k", "up", "ctrl+j", "down"}, "Accept filter"},
			{"filter", "cancel", []string{"esc"}, "Cancel filter"},
			// dialog
			{"dialog", "submit", []string{"enter", "ctrl+s"}, "Submit dialog"},
			{"dialog", "cancel", []string{"esc"}, "Cancel dialog"},
			{"dialog", "next_field", []string{"tab", "j"}, "Next field"},
			{"dialog", "previous_field", []string{"shift+tab", "k"}, "Previous field"},
			{"dialog", "color_prev", []string{"left", "ctrl+left", "ctrl+h"}, "Previous colour"},
			{"dialog", "color_next", []string{"right", "ctrl+right", "ctrl+l"}, "Next colour"},
			// form
			{"form", "next_field", []string{"tab", "j"}, "Next field"},
			{"form", "previous_field", []string{"shift+tab", "k"}, "Previous field"},
			{"form", "edit", []string{"enter"}, "Edit selected field"},
			{"form", "save", []string{"ctrl+s"}, "Save form"},
			{"form", "cancel", []string{"esc"}, "Cancel form"},
			{"form", "jump_list", []string{"l"}, "Jump to list"},
			{"form", "jump_title", []string{"t"}, "Jump to title"},
			{"form", "jump_notes", []string{"n"}, "Jump to notes"},
			{"form", "jump_date", []string{"d"}, "Jump to date"},
			{"form", "jump_time", []string{"i"}, "Jump to time"},
			{"form", "jump_priority", []string{"p"}, "Jump to priority"},
			{"form", "jump_recurrence", []string{"r"}, "Jump to recurrence"},
			{"form", "follow_mode", []string{"ctrl+f"}, "Toggle follow mode"},
			// field
			{"field", "confirm", []string{"enter"}, "Finish field"},
			{"field", "cancel", []string{"esc"}, "Cancel field"},
			{"field", "notes_newline", []string{"ctrl+j"}, "New line in notes"},
			{"field", "external_editor", []string{"ctrl+o"}, "Edit notes in $EDITOR"},
			// choice_field
			{"choice_field", "open", []string{"enter", "space"}, "Open choices"},
			// selector
			{"selector", "down", []string{"ctrl+j", "ctrl+n", "down"}, "Next option"},
			{"selector", "up", []string{"ctrl+k", "ctrl+p", "up"}, "Previous option"},
			{"selector", "confirm", []string{"enter"}, "Confirm selection"},
			{"selector", "cancel", []string{"esc"}, "Cancel selection"},
			{"selector", "toggle", []string{"space"}, "Toggle option"},
			// calendar
			{"calendar", "left", []string{"ctrl+left", "ctrl+h"}, "Back one day"},
			{"calendar", "down", []string{"ctrl+down", "ctrl+j"}, "Forward one week"},
			{"calendar", "up", []string{"ctrl+up", "ctrl+k"}, "Back one week"},
			{"calendar", "right", []string{"ctrl+right", "ctrl+l"}, "Forward one day"},
			{"calendar", "complete", []string{"ctrl+y"}, "Accept completion"},
			{"calendar", "next_suggestion", []string{"down"}, "Next suggestion"},
			{"calendar", "previous_suggestion", []string{"up"}, "Previous suggestion"},
			{"calendar", "month_prev", []string{"ctrl+pgup", "ctrl+p"}, "Back one month"},
			{"calendar", "month_next", []string{"ctrl+pgdown", "ctrl+n"}, "Forward one month"},
			{"calendar", "reset", []string{"ctrl+d"}, "Clear the date field"},
			// confirm
			{"confirm", "yes", []string{"y", "Y", "enter"}, "Confirm"},
			{"confirm", "no", []string{"n", "N", "esc"}, "Cancel"},
			// help
			{"help", "page_down", vpKeys().PageDown.Keys(), "Page down"},
			{"help", "page_up", vpKeys().PageUp.Keys(), "Page up"},
			{"help", "half_page_up", vpKeys().HalfPageUp.Keys(), "Half page up"},
			{"help", "half_page_down", vpKeys().HalfPageDown.Keys(), "Half page down"},
			{"help", "up", vpKeys().Up.Keys(), "Scroll up"},
			{"help", "down", vpKeys().Down.Keys(), "Scroll down"},
			{"help", "left", vpKeys().Left.Keys(), "Scroll left"},
			{"help", "right", vpKeys().Right.Keys(), "Scroll right"},
			{"help", "close", []string{"?", "esc", "q", "ctrl+c"}, "Close help"},
			// textinput
			{"textinput", "character_forward", tiKeys().CharacterForward.Keys(), "Cursor forward"},
			{"textinput", "character_backward", tiKeys().CharacterBackward.Keys(), "Cursor backward"},
			{"textinput", "word_forward", tiKeys().WordForward.Keys(), "Word forward"},
			{"textinput", "word_backward", tiKeys().WordBackward.Keys(), "Word backward"},
			{"textinput", "delete_word_backward", tiKeys().DeleteWordBackward.Keys(), "Delete word backward"},
			{"textinput", "delete_word_forward", tiKeys().DeleteWordForward.Keys(), "Delete word forward"},
			{"textinput", "delete_after_cursor", tiKeys().DeleteAfterCursor.Keys(), "Delete after cursor"},
			{"textinput", "delete_before_cursor", tiKeys().DeleteBeforeCursor.Keys(), "Delete before cursor"},
			{"textinput", "delete_character_backward", tiKeys().DeleteCharacterBackward.Keys(), "Delete character backward"},
			{"textinput", "delete_character_forward", tiKeys().DeleteCharacterForward.Keys(), "Delete character forward"},
			{"textinput", "line_start", tiKeys().LineStart.Keys(), "Line start"},
			{"textinput", "line_end", tiKeys().LineEnd.Keys(), "Line end"},
			{"textinput", "paste", tiKeys().Paste.Keys(), "Paste"},
			{"textinput", "accept_suggestion", tiKeys().AcceptSuggestion.Keys(), "Accept suggestion"},
			{"textinput", "next_suggestion", tiKeys().NextSuggestion.Keys(), "Next suggestion"},
			{"textinput", "previous_suggestion", tiKeys().PrevSuggestion.Keys(), "Previous suggestion"},
			// ace
			{"ace", "cancel", []string{"esc"}, "Cancel ace jump"},
			{"ace", "backspace", []string{"backspace"}, "Remove last ace letter"},
		}
	})
	return registryActs
}

func vpKeys() viewport.KeyMap  { return viewport.DefaultKeyMap() }
func tiKeys() textinput.KeyMap { return textinput.DefaultKeyMap() }

// scopeTitles is the ordered list of scopes with human-readable help titles.
var scopeTitles = []struct {
	Name  string
	Title string
}{
	{"global", "Global"},
	{"list", "List navigation"},
	{"filter", "Filter input"},
	{"dialog", "Dialog"},
	{"form", "Reminder form"},
	{"field", "Field editor"},
	{"choice_field", "Choice fields"},
	{"selector", "Selector"},
	{"calendar", "Date calendar"},
	{"confirm", "Confirm"},
	{"help", "Help overlay"},
	{"textinput", "Text input"},
	{"ace", "Ace jump"},
}

// Scopes returns the ordered, de-duplicated scope names of the registry.
func Scopes() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopeTitles))
	for _, a := range Registry() {
		if !seen[a.Scope] {
			seen[a.Scope] = true
			out = append(out, a.Scope)
		}
	}
	return out
}

// ScopeTitle returns a human-readable title for a scope.
func ScopeTitle(scope string) string {
	for _, s := range scopeTitles {
		if s.Name == scope {
			return s.Title
		}
	}
	return scope
}

// Defaults returns a deep copy of the default bindings for every registered
// action. Callers may mutate the result.
func Defaults() Bindings {
	b := make(Bindings)
	for _, a := range Registry() {
		if b[a.Scope] == nil {
			b[a.Scope] = make(map[string][]string)
		}
		b[a.Scope][a.Name] = append([]string(nil), a.Default...)
	}
	return b
}

func newBinding(a Action, aliases []string) key.Binding {
	return key.NewBinding(
		key.WithKeys(aliases...),
		key.WithHelp(ShortKeys(aliases), a.Help),
	)
}

// Compile validates b against the registry and returns the compiled Map.
// Omitted scopes/actions keep their defaults; explicit empty alias lists
// disable the action. Unknown scopes/actions, empty alias strings and an
// alias bound to two different actions in the same scope are errors.
func Compile(b Bindings) (Map, error) {
	defaults := Defaults()
	out := make(Map, len(defaults))
	byName := map[string]map[string]Action{}
	for _, a := range Registry() {
		if byName[a.Scope] == nil {
			byName[a.Scope] = map[string]Action{}
		}
		byName[a.Scope][a.Name] = a
		m := make(map[string]key.Binding, len(defaults[a.Scope]))
		for name, aliases := range defaults[a.Scope] {
			m[name] = newBinding(byName[a.Scope][name], aliases)
		}
		out[a.Scope] = m
	}

	scopes := make([]string, 0, len(b))
	for scope := range b {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	for _, scope := range scopes {
		actions := b[scope]
		def, ok := byName[scope]
		if !ok {
			return nil, fmt.Errorf("unknown key scope %q", scope)
		}
		names := make([]string, 0, len(actions))
		for name := range actions {
			names = append(names, name)
		}
		sort.Strings(names)
		owner := map[string]string{}
		for _, name := range names {
			a, ok := def[name]
			if !ok {
				return nil, fmt.Errorf("unknown action %q in key scope %q", name, scope)
			}
			for _, alias := range actions[name] {
				if alias == "" {
					return nil, fmt.Errorf("empty key binding for %s.%s", scope, name)
				}
				if prev, dup := owner[alias]; dup && prev != name {
					return nil, fmt.Errorf("key %q is bound to both %s.%s and %s.%s", alias, scope, prev, scope, name)
				}
				owner[alias] = name
			}
			out[scope][name] = newBinding(a, actions[name])
		}
	}
	return out, nil
}

// Bind returns the compiled binding for scope.action, or a binding that never
// matches when the action is not present in the map.
func (m Map) Bind(scope, action string) key.Binding {
	if sm, ok := m[scope]; ok {
		if b, ok := sm[action]; ok {
			return b
		}
	}
	return key.Binding{}
}

// Aliases returns the configured alias strings for scope.action.
func (m Map) Aliases(scope, action string) []string {
	if sm, ok := m[scope]; ok {
		if b, ok := sm[action]; ok {
			return b.Keys()
		}
	}
	return nil
}

// ShortKeys renders compact hint labels for a set of aliases, joined by "/".
func ShortKeys(aliases []string) string {
	parts := make([]string, 0, len(aliases))
	for _, a := range aliases {
		parts = append(parts, ShortKey(a))
	}
	return strings.Join(parts, "/")
}

// ShortKey renders one compact hint label for a key alias.
func ShortKey(alias string) string {
	switch alias {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	case "enter":
		return "⏎"
	case "space":
		return "spc"
	case "backspace":
		return "bksp"
	case "pgup":
		return "pgup"
	case "pgdown":
		return "pgdn"
	}
	s := alias
	s = strings.ReplaceAll(s, "ctrl+", "C-")
	s = strings.ReplaceAll(s, "alt+", "M-")
	s = strings.ReplaceAll(s, "shift+", "S-")
	s = strings.ReplaceAll(s, "+left", "+←")
	s = strings.ReplaceAll(s, "+right", "+→")
	s = strings.ReplaceAll(s, "+up", "+↑")
	s = strings.ReplaceAll(s, "+down", "+↓")
	s = strings.ReplaceAll(s, "+pgdown", "+pgdn")
	return s
}

// EffectiveAceAlphabet validates the raw alphabet's invariant (nonempty,
// ASCII lowercase, no duplicates) and then removes every single-letter alias
// of the configured ace trigger (global.ace_jump), ace.cancel and
// ace.backspace, retaining alphabet order. Modifier aliases such as
// "ctrl+z" do not remove the bare letter. At least two usable letters must
// remain; the result is the label alphabet the UI generates labels from.
func EffectiveAceAlphabet(alphabet string, keys Map) (string, error) {
	if strings.TrimSpace(alphabet) == "" {
		return "", fmt.Errorf("ace alphabet must not be empty")
	}
	seen := map[rune]bool{}
	for _, r := range alphabet {
		if r < 'a' || r > 'z' {
			return "", fmt.Errorf("ace alphabet must contain only ASCII lowercase letters, found %q", string(r))
		}
		if seen[r] {
			return "", fmt.Errorf("ace alphabet contains duplicate letter %q", string(r))
		}
		seen[r] = true
	}
	reserved := map[rune]bool{}
	addReserved := func(scope, action string) {
		for _, alias := range keys.Aliases(scope, action) {
			runes := []rune(alias)
			if len(runes) == 1 {
				reserved[runes[0]] = true
			}
		}
	}
	addReserved("global", "ace_jump")
	addReserved("ace", "cancel")
	addReserved("ace", "backspace")
	usable := make([]rune, 0, len(seen))
	for _, r := range alphabet {
		if !reserved[r] {
			usable = append(usable, r)
		}
	}
	if len(usable) < 2 {
		return "", fmt.Errorf("ace alphabet must retain at least two usable letters after reserved keys, got %q", string(usable))
	}
	return string(usable), nil
}
