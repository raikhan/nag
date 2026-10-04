package keybind

import (
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

func TestValidateAceAlphabet(t *testing.T) {
	if err := ValidateAceAlphabet(DefaultAceAlphabet, []string{"esc"}, []string{"backspace"}); err != nil {
		t.Fatalf("default alphabet rejected: %v", err)
	}
	for _, bad := range []struct {
		alpha string
		cnl   []string
		bksp  []string
	}{
		{"", nil, nil},
		{"aab", nil, nil},
		{"abc1", nil, nil},
		{"a", nil, nil},
		{"abc", []string{"a"}, nil},
	} {
		if err := ValidateAceAlphabet(bad.alpha, bad.cnl, bad.bksp); err == nil {
			t.Errorf("alphabet %q expected error", bad.alpha)
		}
	}
}
