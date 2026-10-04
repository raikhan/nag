// Package config loads nag's TOML configuration file: key binding overrides
// and the ace-jump alphabet.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/oronbz/nag/internal/keybind"
)

// Config is the decoded nag configuration.
type Config struct {
	Keys              keybind.Bindings
	AceAlphabet       string
	AceTimeoutSeconds int64
}

// Path resolves the configuration file location: $XDG_CONFIG_HOME/nag/config.toml
// when XDG_CONFIG_HOME is set to an absolute path, otherwise
// $HOME/.config/nag/config.toml.
func Path() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		if !filepath.IsAbs(x) {
			return "", fmt.Errorf("XDG_CONFIG_HOME must be an absolute path, got %q", x)
		}
		return filepath.Join(x, "nag", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "nag", "config.toml"), nil
}

// aceTable uses pointers so a partial [ace] table (for example only
// timeout_seconds) overlays onto the defaults instead of discarding them.
type aceTable struct {
	Alphabet       *string `toml:"alphabet"`
	TimeoutSeconds *int64  `toml:"timeout_seconds"`
}

type rawConfig struct {
	Keys map[string]map[string][]string `toml:"keys"`
	Ace  *aceTable                      `toml:"ace"`
}

// Load reads the config file, overlaying any explicit values onto the default
// registry so partial tables cannot discard defaults. A missing file yields
// pure defaults that still go through the same binding/ace validation.
// Malformed TOML, unreadable files, invalid bindings, an invalid ace alphabet
// or an invalid ace timeout produce a path-qualified error. The returned
// AceAlphabet is the effective label alphabet computed against the user's
// fully compiled bindings, not the raw configured alphabet.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	cfg := defaultsConfig()
	if err == nil {
		var raw rawConfig
		dec := toml.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&raw); err != nil {
			return Config{}, fmt.Errorf("%s: %w", path, err)
		}
		if raw.Keys != nil {
			for scope, actions := range raw.Keys {
				if cfg.Keys[scope] == nil {
					// Unknown scopes are rejected by Compile below; keep the raw
					// entry so the error reports the exact scope name.
					cfg.Keys[scope] = make(map[string][]string, len(actions))
				}
				for action, aliases := range actions {
					cfg.Keys[scope][action] = aliases
				}
			}
		}
		if raw.Ace != nil {
			if raw.Ace.Alphabet != nil {
				cfg.AceAlphabet = *raw.Ace.Alphabet
			}
			if raw.Ace.TimeoutSeconds != nil {
				cfg.AceTimeoutSeconds = *raw.Ace.TimeoutSeconds
			}
		}
	}

	keys, err := keybind.Compile(cfg.Keys)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	effective, err := keybind.EffectiveAceAlphabet(cfg.AceAlphabet, keys)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	cfg.AceAlphabet = effective
	if !validAceTimeout(cfg.AceTimeoutSeconds) {
		return Config{}, fmt.Errorf("%s: invalid ace timeout_seconds %d (use -1 to disable, or 0..9223372036)", path, cfg.AceTimeoutSeconds)
	}
	return cfg, nil
}

// maxAceTimeoutSeconds is the largest whole-second duration representable by
// time.Duration without overflow.
const maxAceTimeoutSeconds = int64(math.MaxInt64 / int64(time.Second))

func validAceTimeout(seconds int64) bool {
	return seconds == -1 || (seconds >= 0 && seconds <= maxAceTimeoutSeconds)
}

func defaultsConfig() Config {
	return Config{
		Keys:              keybind.Defaults(),
		AceAlphabet:       keybind.DefaultAceAlphabet,
		AceTimeoutSeconds: -1,
	}
}

// Init creates the config file with defaults using exclusive creation, so an
// existing file is never overwritten. It reports the path and whether the
// file was newly created; an existing file is a successful no-op.
func Init() (path string, created bool, err error) {
	path, err = Path()
	if err != nil {
		return "", false, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, false, fmt.Errorf("%s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return path, false, nil
		}
		return path, false, fmt.Errorf("%s: %w", path, err)
	}
	if err := writeDefaults(f); err != nil {
		f.Close()
		os.Remove(path)
		return path, false, fmt.Errorf("%s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return path, false, fmt.Errorf("%s: %w", path, err)
	}
	return path, true, nil
}

func writeDefaults(w io.Writer) error {
	bb := bufio.NewWriter(w)
	fmt.Fprintln(bb, "# nag configuration file.")
	fmt.Fprintln(bb, "#")
	fmt.Fprintln(bb, "# Key bindings use Bubble Tea key-string notation, e.g.")
	fmt.Fprintln(bb, "# \"enter\", \"esc\", \"space\", \"up\", \"ctrl+s\", \"shift+tab\".")
	fmt.Fprintln(bb, "# Set an action's list to [] to disable it; unlisted actions keep their")
	fmt.Fprintln(bb, "# defaults. Run `nag help` for the full, currently configured list.")
	fmt.Fprintln(bb, "#")
	fmt.Fprintln(bb, "# alphabet: letters used as ace-jump row labels (activated by the")
	fmt.Fprintln(bb, "# configured global.ace_jump trigger; letters used by the trigger,")
	fmt.Fprintln(bb, "# ace.cancel or ace.backspace are never used as labels).")
	fmt.Fprintln(bb, "# timeout_seconds: auto-cancel the ace jump after this many seconds;")
	fmt.Fprintln(bb, "# -1 disables the timeout, 0 cancels immediately.")
	fmt.Fprintln(bb, "")
	fmt.Fprintln(bb, "[ace]")
	fmt.Fprintf(bb, "alphabet = %q\n", keybind.DefaultAceAlphabet)
	fmt.Fprintln(bb, "timeout_seconds = -1")
	for _, scope := range keybind.Scopes() {
		fmt.Fprintln(bb)
		fmt.Fprintf(bb, "[keys.%s]\n", scope)
		for _, a := range keybind.Registry() {
			if a.Scope != scope {
				continue
			}
			fmt.Fprintf(bb, "%s = [%s]\n", a.Name, quotedList(a.Default))
		}
	}
	return bb.Flush()
}

func quotedList(aliases []string) string {
	out := make([]byte, 0, len(aliases)*12)
	for i, a := range aliases {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, '"')
		out = append(out, a...)
		out = append(out, '"')
	}
	return string(out)
}
