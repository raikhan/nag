package dialog

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"

	"github.com/oronbz/nag/internal/keybind"
)

// textInputActions maps snake_case registry action names to their
// corresponding Bubbles textinput keymap fields.
var textInputActions = []struct {
	action  string
	binding func(k *textinput.KeyMap) *key.Binding
}{
	{"character_forward", func(k *textinput.KeyMap) *key.Binding { return &k.CharacterForward }},
	{"character_backward", func(k *textinput.KeyMap) *key.Binding { return &k.CharacterBackward }},
	{"word_forward", func(k *textinput.KeyMap) *key.Binding { return &k.WordForward }},
	{"word_backward", func(k *textinput.KeyMap) *key.Binding { return &k.WordBackward }},
	{"delete_word_backward", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteWordBackward }},
	{"delete_word_forward", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteWordForward }},
	{"delete_after_cursor", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteAfterCursor }},
	{"delete_before_cursor", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteBeforeCursor }},
	{"delete_character_backward", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteCharacterBackward }},
	{"delete_character_forward", func(k *textinput.KeyMap) *key.Binding { return &k.DeleteCharacterForward }},
	{"line_start", func(k *textinput.KeyMap) *key.Binding { return &k.LineStart }},
	{"line_end", func(k *textinput.KeyMap) *key.Binding { return &k.LineEnd }},
	{"paste", func(k *textinput.KeyMap) *key.Binding { return &k.Paste }},
	{"accept_suggestion", func(k *textinput.KeyMap) *key.Binding { return &k.AcceptSuggestion }},
	{"next_suggestion", func(k *textinput.KeyMap) *key.Binding { return &k.NextSuggestion }},
	{"previous_suggestion", func(k *textinput.KeyMap) *key.Binding { return &k.PrevSuggestion }},
}

// projectTextInputKeys rebinds a Bubbles textinput to the configured
// keys.textinput actions, dropping any alias claimed by reserved scopes so
// those keys can be intercepted before the editor sees them.
func projectTextInputKeys(m *textinput.Model, keys keybind.Map, reservedScopes ...string) {
	reservedAliases := map[string]bool{}
	for _, scope := range reservedScopes {
		for _, action := range scopeActions(scope) {
			for _, alias := range keys.Aliases(scope, action) {
				reservedAliases[alias] = true
			}
		}
	}

	km := textinput.KeyMap{}
	for _, a := range textInputActions {
		aliases := keys.Aliases("textinput", a.action)
		filtered := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			if !reservedAliases[alias] {
				filtered = append(filtered, alias)
			}
		}
		if len(filtered) == 0 {
			filtered = []string{"\x00disabled"} // never matched by real keys
		}
		*a.binding(&km) = key.NewBinding(key.WithKeys(filtered...))
	}
	m.KeyMap = km
}

// scopeActions lists the registry action names of the given scope that the
// dialog packages consume.
func scopeActions(scope string) []string {
	switch scope {
	case "calendar":
		return []string{"left", "down", "up", "right", "complete", "next_suggestion", "previous_suggestion"}
	case "selector":
		return []string{"down", "up", "confirm", "cancel", "toggle"}
	case "dialog":
		return []string{"submit", "cancel", "next_field", "previous_field"}
	case "field":
		return []string{"confirm", "cancel", "next_field", "previous_field"}
	case "choice_field":
		return []string{"open"}
	case "confirm":
		return []string{"yes", "no"}
	default:
		return nil
	}
}
