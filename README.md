# nag

A LazyGit-style terminal UI for [Apple Reminders](https://support.apple.com/guide/reminders/welcome/mac). Browse lists, view reminders, create, complete, and delete — all from the terminal.

![CI](https://github.com/oronbz/nag/actions/workflows/ci.yaml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![macOS](https://img.shields.io/badge/macOS-only-000000?logo=apple&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)

![nag demo](demo.gif)

## Features

- **Two-panel layout** — lists sidebar + reminders
- **Smart lists** — Today (includes overdue) and Scheduled views
- **Vim-style navigation** — `j`/`k`, `g`/`G`, `Ctrl-d`/`Ctrl-u`
- **Create reminders** — title, due date with time, priority
- **Complete/uncomplete** — toggle with Space, 2s grace period to undo
- **Delete** — with confirmation prompt
- **Open in Reminders** — jump to the reminder in Apple Reminders
- **Sort** — cycle between default, created, due date, and title with `s`
- **Show/hide completed** — toggle visibility with `c`
- **Auto-refresh** — polls every 10 seconds for external changes
- **Filter/search** — fuzzy search across titles and notes
- **Mouse support** — click to focus panels, scroll to navigate

## Install

### Homebrew (recommended)

```bash
brew install oronbz/tap/nag
```

### Go install

Requires [Go 1.25+](https://go.dev/dl/) and **macOS** (uses native EventKit via cgo):

```bash
go install github.com/oronbz/nag@latest
```

### From source

```bash
git clone https://github.com/oronbz/nag.git
cd nag
make install
```

## Usage

```bash
nag
```

On first run, macOS will prompt for Reminders access. You can manage this in **System Settings > Privacy & Security > Reminders**.

## Key Bindings

All key bindings are configurable in `~/.config/nag/config.toml` (or
`$XDG_CONFIG_HOME/nag/config.toml`). Run `nag config init` to write the
defaults, and `nag help` to list the bindings currently in effect.

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k`, `↑` / `↓` | Navigate (never lands on a list separator) |
| `Enter` | Select list |
| `Tab` / `Shift-Tab` | Switch panel |
| `g` / `G` | Jump to top / bottom |
| `Ctrl-u` / `Ctrl-d` | Half page up / down |
| `/` | Filter / search |
| `z` | Ace jump to any visible list or reminder row |
| Left click | Focus panel |
| Mouse wheel | Scroll |

### Actions

| Key | Action |
|-----|--------|
| `Space` / `x` | Toggle reminder complete |
| `n` | New reminder / list |
| `e` | Edit reminder / list |
| `d` | Delete reminder / list |
| `o` | Open in Apple Reminders |
| `s` | Cycle sort order |
| `c` | Toggle show completed |
| `r` | Refresh |

### Reminder form

The create/edit form keeps all six fields visible beside the active editor.

| Key | Action |
|-----|--------|
| `Tab` / `j` | Next field (browsing only — disabled while a field editor is open) |
| `Shift-Tab` / `k` | Previous field (browsing only) |
| `t` `n` `d` `i` `p` `r` | Jump to Title / Notes / Date / Time / Priority / Recurrence and edit it |
| `Enter` | Edit selected field / finish the open field |
| `Ctrl-J` | New line in the Notes editor |
| `Ctrl-O` | Edit the Notes value in `$EDITOR` |
| `Ctrl-←` / `Ctrl-→` | Move the calendar highlight a day |
| `Ctrl-↑` / `Ctrl-↓` | Move the calendar highlight a week |
| `Ctrl-PgUp` / `Ctrl-PgDn` | Move the calendar highlight a month (clamped, e.g. Oct 31 → Nov 30) |
| `Ctrl-D` | Clear the date field |
| `Ctrl-S` | Save the form |
| `Esc` | Cancel the open field, then the form |

Single-field dialogs (create list, confirmations) still submit with `Enter`.

### Dialogs

| Key | Action |
|-----|--------|
| `Enter` | Submit dialog / open choices |
| `Ctrl-S` | Submit dialog |
| `Tab` / `j` | Next field |
| `Shift-Tab` / `k` | Previous field |
| `Esc` | Cancel dialog or inner chooser |

### General

| Key | Action |
|-----|--------|
| `?` | Toggle help overlay |
| `Esc` | Close dialog / overlay |
| `q` / `Ctrl-C` | Quit |

### Date and time entry

Date and Time are separate fields. The Date field takes Org-mode style civil
dates: `today`, `tom`, `tue`, `eow`, `eom`, `+2d`, `+7w`, `+2tue`,
`sep 15`, `2026-10-04`, … A keyboard calendar (Ctrl-h/J/K/L moves the date by
a day/week) and live autocomplete (Ctrl-Y completes, Ctrl-N/P cycle) appear
while the date field is focused.

The Time field takes clock times: `6p`, `6pm`, `14:13`, `1413`, `615p`,
`12am`, `12pm`, `9:05`, … A blank time with a date means 09:00 local.

### Recurrence

Daily, weekly, monthly, yearly and common presets via the fuzzy selector; the
`Custom…` editor covers intervals, weekday/day-of-month patterns and end
conditions, and round-trips natively with Apple Reminders.

## Layout

```
┌──────────┬─────────────────────┐
│  Lists   │     Reminders       │
│          │                     │
│ ◉ Today  │ ☐ Buy groceries !!! │
│ ▦ Sched. │   today  Get milk.. │
│ ───────  │ ☑ Call dentist      │
│ Personal │   yesterday         │
│ Work     │                     │
└──────────┴─────────────────────┘
 status bar
```

## Built With

- [go-eventkit](https://github.com/BRO3886/go-eventkit) — Native macOS EventKit bindings (cgo)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework (Elm Architecture)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling and layout
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components (list, viewport, textinput, spinner)

## License

MIT
