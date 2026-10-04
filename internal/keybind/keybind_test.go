package keybind

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func keyPress(k rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: k}
}

func TestDefaultsCoverRegistry(t *testing.T) {
	d := Defaults()
	for _, a := range Registry() {
		if _, ok := d[a.Scope]; !ok {
			t.Fatalf("scope %q missing from defaults", a.Scope)
		}
		if _, ok := d[a.Scope][a.Name]; !ok {
			t.Fatalf("action %s.%s missing from defaults", a.Scope, a.Name)
		}
	}
}

func TestCompileDefaultsOmittedKept(t *testing.T) {
	m, err := Compile(Bindings{})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Aliases("global", "quit"); len(got) != 2 || got[0] != "q" || got[1] != "ctrl+c" {
		t.Fatalf("default quit aliases = %v", got)
	}
}

func TestCompileOverridesAndDisable(t *testing.T) {
	m, err := Compile(Bindings{
		"global": {"quit": {"ctrl+q"}, "help": {}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Aliases("global", "quit"); len(got) != 1 || got[0] != "ctrl+q" {
		t.Fatalf("quit aliases = %v", got)
	}
	if got := m.Aliases("global", "help"); len(got) != 0 {
		t.Fatalf("disabled help still has aliases %v", got)
	}
	if key.Matches(keyPress('?'), m.Bind("global", "help")) {
		t.Fatal("disabled action matched a key")
	}
	if !key.Matches(keyPress('z'), m.Bind("global", "ace_jump")) {
		t.Fatal("default ace_jump should match z")
	}
}

// TestEditMatchesEnterAndE pins the default edit aliases: Enter is the
// primary key and e remains an alias for both.
func TestEditMatchesEnterAndE(t *testing.T) {
	m, err := Compile(Defaults())
	if err != nil {
		t.Fatal(err)
	}
	b := m.Bind("global", "edit")
	if !key.Matches(tea.KeyPressMsg{Code: tea.KeyEnter}, b) {
		t.Fatal("global.edit should match Enter")
	}
	if !key.Matches(tea.KeyPressMsg{Code: 'e', Text: "e"}, b) {
		t.Fatal("global.edit should match e")
	}
	if !key.Matches(tea.KeyPressMsg{Code: tea.KeyEnter}, m.Bind("list", "select")) {
		t.Fatal("list.select should match Enter")
	}
}

// TestHLFocusesPanelsNotPages pins the split: h/l move between panels, and
// paging keeps its own keys.
func TestHLFocusesPanelsNotPages(t *testing.T) {
	m, err := Compile(Defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !key.Matches(tea.KeyPressMsg{Code: 'h', Text: "h"}, m.Bind("global", "focus_left")) {
		t.Fatal("global.focus_left should match h")
	}
	if !key.Matches(tea.KeyPressMsg{Code: 'l', Text: "l"}, m.Bind("global", "focus_right")) {
		t.Fatal("global.focus_right should match l")
	}
	prev := m.Aliases("list", "previous_page")
	next := m.Aliases("list", "next_page")
	for _, alias := range []string{"h", "l"} {
		for _, got := range append(append([]string{}, prev...), next...) {
			if got == alias {
				t.Fatalf("page key %q must not also page: it moves panels", alias)
			}
		}
	}
	if !contains(prev, "pgup") || !contains(next, "pgdown") {
		t.Fatalf("paging lost its arrow/pgup keys: prev=%v next=%v", prev, next)
	}
	if !contains(prev, "left") || !contains(next, "right") {
		t.Fatalf("paging lost the arrow keys: prev=%v next=%v", prev, next)
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func TestCompileRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		b    Bindings
	}{
		{"unknown scope", Bindings{"nope": {"quit": {"q"}}}},
		{"unknown action", Bindings{"global": {"quitx": {"q"}}}},
		{"empty alias", Bindings{"global": {"quit": {"", "q"}}}},
		{"same-scope duplicate", Bindings{"list": {"up": {"w"}, "down": {"w"}}}},
	}
	for _, tc := range cases {
		if _, err := Compile(tc.b); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestCompileKeepsCase(t *testing.T) {
	m, err := Compile(Bindings{})
	if err != nil {
		t.Fatal(err)
	}
	if !key.Matches(keyPress('G'), m.Bind("list", "last")) {
		t.Fatal("G should match list.last")
	}
	if key.Matches(keyPress('g'), m.Bind("list", "last")) {
		t.Fatal("G must not match g")
	}
}

func TestEffectiveAceAlphabet(t *testing.T) {
	keys, err := Compile(Defaults())
	if err != nil {
		t.Fatal(err)
	}
	// The default trigger z is reserved; the rest of the alphabet survives in
	// original order.
	eff, err := EffectiveAceAlphabet(DefaultAceAlphabet, keys)
	if err != nil {
		t.Fatalf("default alphabet rejected: %v", err)
	}
	want := strings.ReplaceAll(DefaultAceAlphabet, "z", "")
	if eff != want {
		t.Fatalf("effective = %q, want %q", eff, want)
	}

	for _, bad := range []struct {
		alpha string
		keys  Map
	}{
		{"", keys},
		{"aab", keys},
		{"abc1", keys},
		{"a", keys},
		// Trigger removal leaves fewer than two usable letters.
		{"abc", mustCompileBindings(t, Bindings{
			"global": {"ace_jump": {"a"}},
			"ace":    {"cancel": {"b"}, "backspace": {"c"}},
		})},
	} {
		if _, err := EffectiveAceAlphabet(bad.alpha, bad.keys); err == nil {
			t.Errorf("alphabet %q expected error", bad.alpha)
		}
	}

	// Multiple trigger aliases each remove their bare letter; modifier
	// aliases such as ctrl+g do not remove bare g.
	b := Bindings{
		"global": {"ace_jump": {"a", "z", "ctrl+g"}},
	}
	keys2, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	eff2, err := EffectiveAceAlphabet("azgcq", keys2)
	if err != nil {
		t.Fatalf("multi-alias alphabet rejected: %v", err)
	}
	if eff2 != "gcq" {
		t.Fatalf("effective = %q, want %q", eff2, "gcq")
	}

	// Disabled trigger removes nothing.
	b["global"] = map[string][]string{"ace_jump": {}}
	keys3, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	eff3, err := EffectiveAceAlphabet("azgcq", keys3)
	if err != nil {
		t.Fatalf("disabled-trigger alphabet rejected: %v", err)
	}
	if eff3 != "azgcq" {
		t.Fatalf("effective = %q, want %q", eff3, "azgcq")
	}
}

func mustCompileBindings(t *testing.T, b Bindings) Map {
	t.Helper()
	m, err := Compile(b)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return m
}
