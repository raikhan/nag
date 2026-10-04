package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oronbz/nag/internal/keybind"
)

func tempXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	tempXDG(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AceAlphabet != strings.ReplaceAll(keybind.DefaultAceAlphabet, "z", "") {
		t.Fatalf("alphabet = %q", cfg.AceAlphabet)
	}
	if cfg.AceTimeoutSeconds != -1 {
		t.Fatalf("timeout = %d", cfg.AceTimeoutSeconds)
	}
	if got := cfg.Keys["global"]["quit"]; len(got) != 2 {
		t.Fatalf("quit aliases = %v", got)
	}
}

func TestLoadPartialOverrideKeepsDefaults(t *testing.T) {
	dir := tempXDG(t)
	path := filepath.Join(dir, "nag", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[keys.global]\nquit = [\"ctrl+q\"]\n\n[keys.list]\ndown = [\"ctrl+n\"]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Keys["global"]["quit"]; len(got) != 1 || got[0] != "ctrl+q" {
		t.Fatalf("quit = %v", got)
	}
	if got := cfg.Keys["global"]["new"]; len(got) != 1 || got[0] != "n" {
		t.Fatalf("new = %v", got)
	}
	if got := cfg.Keys["list"]["down"]; len(got) != 1 || got[0] != "ctrl+n" {
		t.Fatalf("list.down = %v", got)
	}
}

func TestLoadEmptyArrayDisables(t *testing.T) {
	dir := tempXDG(t)
	path := filepath.Join(dir, "nag", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[keys.global]\nace_jump = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	m, err := keybind.Compile(cfg.Keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Aliases("global", "ace_jump")) != 0 {
		t.Fatal("ace_jump should be disabled")
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed", "[keys.global\nquit = [\"q\"]"},
		{"unknown scope", "[keys.nope]\nquit = [\"q\"]"},
		{"unknown action", "[keys.global]\nquitx = [\"q\"]"},
		{"empty alias", "[keys.global]\nquit = [\"\"]"},
		{"non-array", "[keys.global]\nquit = \"q\""},
		{"duplicate in scope", "[keys.list]\nup = [\"w\"]\n\ndown = [\"w\"]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempXDG(t)
			path := filepath.Join(dir, "nag", "config.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(); err == nil {
				t.Fatal("expected error")
			} else if !strings.HasPrefix(err.Error(), path+":") {
				t.Fatalf("error not path-qualified: %v", err)
			}
		})
	}
}

func TestLoadAceTimeout(t *testing.T) {
	write := func(t *testing.T, body string) {
		t.Helper()
		dir := tempXDG(t)
		path := filepath.Join(dir, "nag", "config.toml")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("timeout-only table keeps alphabet default", func(t *testing.T) {
		write(t, "[ace]\ntimeout_seconds = 2\n")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AceTimeoutSeconds != 2 {
			t.Fatalf("timeout = %d", cfg.AceTimeoutSeconds)
		}
		if cfg.AceAlphabet != strings.ReplaceAll(keybind.DefaultAceAlphabet, "z", "") {
			t.Fatalf("alphabet = %q", cfg.AceAlphabet)
		}
	})

	for _, ok := range []int64{-1, 0, 1, 30, 9223372036} {
		t.Run(fmt.Sprintf("accepts %d", ok), func(t *testing.T) {
			write(t, fmt.Sprintf("[ace]\ntimeout_seconds = %d\n", ok))
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AceTimeoutSeconds != ok {
				t.Fatalf("timeout = %d, want %d", cfg.AceTimeoutSeconds, ok)
			}
		})
	}

	for _, bad := range []string{
		"-2",
		"9223372037",
		"1.5",
		"\"2\"",
	} {
		t.Run("rejects "+bad, func(t *testing.T) {
			write(t, "[ace]\ntimeout_seconds = "+bad+"\n")
			if _, err := Load(); err == nil {
				t.Fatal("expected timeout error")
			}
		})
	}
}

func TestLoadAlphabetValidation(t *testing.T) {
	dir := tempXDG(t)
	path := filepath.Join(dir, "nag", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[ace]\nalphabet = \"aab\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected alphabet error")
	}
}

func TestRelativeXDGRejected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative/path")
	if _, err := Path(); err == nil {
		t.Fatal("expected error for relative XDG_CONFIG_HOME")
	}
}

func TestInitCreatesExclusive(t *testing.T) {
	dir := tempXDG(t)
	path, created, err := Init()
	if err != nil || !created {
		t.Fatalf("Init: created=%v err=%v", created, err)
	}
	if path != filepath.Join(dir, "nag", "config.toml") {
		t.Fatalf("path = %q", path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v", fi.Mode())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v", di.Mode())
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	path2, created2, err := Init()
	if err != nil || created2 || path2 != path {
		t.Fatalf("second Init: created=%v err=%v path=%q", created2, err, path2)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("existing config bytes were modified")
	}
	// The written file must itself parse as valid config.
	if _, err := Load(); err != nil {
		t.Fatalf("generated config does not load: %v", err)
	}
}

func TestInitXDGHomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	path, created, err := Init()
	if err != nil || !created {
		t.Fatalf("Init: created=%v err=%v", created, err)
	}
	want := filepath.Join(os.Getenv("HOME"), ".config", "nag", "config.toml")
	if path != want {
		t.Fatalf("path = %q want %q", path, want)
	}
}
