# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Configurable keybindings via TOML (`nag config init` writes `~/.config/nag/config.toml` or `$XDG_CONFIG_HOME/nag/config.toml`); all scopes (global, list, filter, dialog, choice fields, selector, calendar, confirm, help, text input, ace) are independently remappable and `nag help` renders the configured bindings
- Org-mode style due-date entry (`today`, `tue`, `+2d`, `+7w`, `eow`, `eom`, `sep 15`, ISO weeks, `H:MM` times) with live autocomplete and a synchronized keyboard calendar while the due field is focused
- Fuzzy priority selection and repeat presets through a shared fuzzy selector popup
- Native Apple Reminders recurrence: round-trip display of existing rules plus editing of daily/weekly/monthly/yearly schedules (intervals, weekday and day-of-month patterns, end date/occurrence count) via a Custom repeat editor; hourly is not available through the public EventKit API
- `z` ace jump: label and jump to any visible sidebar list or visible reminder row of the selected list using configurable alphabet labels

### Fixed
- Sidebar separator can no longer be selected by navigation, paging or wheel scrolling
- Filtered lists no longer flash empty while a background refresh re-applies items; an applied filter now survives refreshes
- Separator line between list sections spilled over onto a second line

### Changed
- Upgraded UI stack to Bubble Tea v2 (`charm.land/bubbletea/v2` v2.0.10), Bubbles v2 (v2.2.1), and Lip Gloss v2 (v2.0.6); requires Go 1.26+
- List styling now adapts to the terminal's light/dark background; app accent colors unchanged

## [0.3.2] - 2026-03-04

### Fixed
- Due-date placeholder text cut off in create dialog

## [0.3.0] - 2026-03-04

### Added
- Sorting toggle (`s`) with default, created, due date, and title modes

### Fixed
- Due dates displaying in UTC instead of local timezone

## [0.2.0] - 2026-03-04

### Added
- Context-sensitive `n`/`e`/`d` keys for both reminders and lists
- List CRUD operations (create, rename, delete)

## [0.1.0] - 2026-03-04

### Added
- Initial release
- Two-panel layout with lists sidebar and reminders
- Smart lists: Today (includes overdue) and Scheduled
- Vim-style navigation (`j`/`k`, `g`/`G`, `Ctrl-d`/`Ctrl-u`)
- Create reminders with title, due date, time, and priority
- Complete/uncomplete toggle with 2-second grace period
- Delete with confirmation
- Open in Apple Reminders
- Show/hide completed reminders
- Auto-refresh (10-second polling)
- Fuzzy filter/search
- Mouse support
- Help overlay (`?`)
