package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/oronbz/nag/internal/config"
	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/reminders"
	"github.com/oronbz/nag/internal/ui"
)

var version = "dev"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	args := os.Args[1:]
	createOnly := false
	createList := ""
	if len(args) > 0 {
		switch args[0] {
		case "config":
			if len(args) != 2 || args[1] != "init" {
				fmt.Fprintln(os.Stderr, "Usage: nag config init")
				os.Exit(1)
			}
			path, created, err := config.Init()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if created {
				fmt.Printf("Created config: %s\n", path)
			} else {
				fmt.Printf("Config already exists: %s\n", path)
			}
			return
		case "help", "--help", "-h":
			keys, _ := loadKeysOrExit()
			printHelp(keys)
			return
		case "version", "--version", "-v":
			fmt.Println("nag " + getVersion())
			return
		case "--create":
			// Falls through to normal startup with only the form shown.
			createOnly = true
			switch len(args) {
			case 1:
			case 2:
				createList = args[1]
			default:
				fmt.Fprintln(os.Stderr, cliUsage())
				os.Exit(1)
			}
		default:
			fmt.Fprintln(os.Stderr, cliUsage())
			os.Exit(1)
		}
	}

	keys, cfg := loadKeysOrExit()
	client, err := reminders.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to access Reminders.")
		fmt.Fprintln(os.Stderr, "Make sure you've granted Reminders access in System Settings > Privacy & Security > Reminders.")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var model ui.Model
	if createOnly {
		model = ui.NewCreateModel(client, keys, createList)
	} else {
		model = ui.NewModel(client, keys, cfg.AceAlphabet, cfg.AceTimeoutSeconds)
	}
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadKeysOrExit loads the user config and compiles key bindings. Any failure
// exits before Reminders/EventKit access is attempted.
func loadKeysOrExit() (keybind.Map, config.Config) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	keys, err := keybind.Compile(cfg.Keys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return keys, cfg
}

func cliUsage() string {
	return `nag — A terminal UI for Apple Reminders

  nag                  Launch the TUI
  nag --create [list]  Open the reminder form only (optionally preselecting a list)
  nag config init      Write the default config file
  nag help             Show this help message
  nag version          Show version

Note:
  On first run, macOS will prompt for Reminders access.
  You can manage this in System Settings > Privacy & Security > Reminders.
`
}

func printHelp(keys keybind.Map) {
	fmt.Print(cliUsage())
	fmt.Print(`
Keybindings (inside TUI):

`)
	for _, scope := range keybind.Scopes() {
		var lines []string
		for _, a := range keybind.Registry() {
			if a.Scope != scope {
				continue
			}
			aliases := keys.Aliases(scope, a.Name)
			if len(aliases) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %-22s%s", keybind.ShortKeys(aliases)+":", a.Help))
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Println(keybind.ScopeTitle(scope) + ":")
		fmt.Println(strings.Join(lines, "\n"))
		fmt.Println()
	}
	fmt.Print(`Note:
  On first run, macOS will prompt for Reminders access.
  You can manage this in System Settings > Privacy & Security > Reminders.
`)
}
