# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Completed smart list in the sidebar after Today and Scheduled: every completed reminder from every list, sectioned the way the Reminders app does — `Today`, `Yesterday`, `Previous 7 Days`, `Previous 30 Days`, then the remaining months of the current year (`Rest of <Month>` for the month the 30-day window cuts into) and each earlier year, newest first with the owning list, due date and completion time under every row; `Space`/`x` uncompletes a task and sends it back to its original list, re-creating that list first if it was deleted while the task sat here, and the sidebar badge carries the number of completed reminders
- Configurable keybindings via TOML (`nag config init` writes `~/.config/nag/config.toml` or `$XDG_CONFIG_HOME/nag/config.toml`); all scopes (global, list, filter, dialog, choice fields, selector, calendar, confirm, help, text input, ace) are independently remappable and `nag help` renders the configured bindings
- Org-mode style due-date entry (`today`, `tue`, `+2d`, `+7w`, `eow`, `eom`, `sep 15`, ISO weeks, `H:MM` times) with live autocomplete and a synchronized keyboard calendar while the due field is focused
- Fuzzy priority selection and repeat presets through a shared fuzzy selector popup
- Native Apple Reminders recurrence: round-trip display of existing rules plus editing of daily/weekly/monthly/yearly schedules (intervals, weekday and day-of-month patterns, end date/occurrence count) via a Custom repeat editor; hourly is not available through the public EventKit API
- `z` ace jump: label and jump to any visible sidebar list or visible reminder row of the selected list using configurable alphabet labels; the trigger is a toggle, trigger letters are never used as labels, and `[ace] timeout_seconds` auto-cancels the jump (`-1` disables, `0` cancels immediately)
- Separate Date and Time entry in the create/edit form: Org-style civil dates in the Date field, clock times (`6p`, `14:13`, `615p`) in the Time field with a live canonical preview and local zone; blank time with a date means midnight local
- Reminder form navigation: seven fields browsed with `Tab`/`j`, `Shift-Tab`/`k`, mnemonic jumps (`l` `t` `n` `d` `i` `p` `r`), `Enter` to edit/finish fields and `Ctrl-S` to save; all fields stay visible beside the active editor in a side-by-side pane layout that stacks on narrow terminals
- The Custom repeat editor steps its fields with `Tab`/`j` and `Shift-Tab`/`k` like the reminder form, and `Ctrl-F` toggles follow mode inside it: with follow on, confirming an inner chooser advances to the next editor field, including fields the choice just added; cancelling a chooser leaves the cursor where it was, and the editor's mode is mirrored back to the form
- Fuzzy list picker as the first field of the create/edit form: the reminder's list is chosen by typing, each option shows the list's colour, a save without a list is refused, and on edit picking a different list moves the reminder (an untouched list sends no patch)
- List colours: a 13-swatch row in the New/Edit List dialog (`Tab` to focus it, `Ctrl-←`/`Ctrl-→` to walk it), rendered as a dot in the sidebar and the list picker; a rename never overwrites a stored colour
- Mouse row selection: a click selects the row under the cursor, a double-click opens the matching reminder or list editor, and dragging a reminder onto a list moves it (the dragged row dims, the target list shows a `⤓ drop` marker, and a press without motion moves nothing)
- `nag --create [list]` opens just the reminder form, optionally with a list preselected; it exits once the reminder is saved, and `Esc` or `Ctrl-C` leaves it

### Fixed
- Calendar grid aligned every one of its 42 cells with the weekday columns; adjacent-month days render dimmed instead of leaving blank leading cells
- The create/edit form's vertical divider stays at the horizontal middle regardless of content length; long values truncate on the left pane and wrap in the right pane
- `Tab`/`Shift-Tab` no longer interrupt an open field editor or leak into its widget
- The Date field shows the resolved canonical date instead of the fuzzy term (`tom` commits and displays as `2026-10-05`), and while the date field is focused `Ctrl-PgUp`/`Ctrl-PgDn` move a month with day clamping (Oct 31 → Nov 30) and `Ctrl-D` clears the field
- Sync refreshes keep the reminder selection on the same reminder even when external deletions shift the rows above it
### Fixed
- Sidebar separator can no longer be selected by navigation, paging or wheel scrolling
- Filtered lists no longer flash empty while a background refresh re-applies items; an applied filter now survives refreshes
- Separator line between list sections spilled over onto a second line

### Changed
- Reminder form Notes is a multiline editor: `Ctrl-J` inserts a newline, `Enter` finishes the field, and `Ctrl-O` opens the value in `$EDITOR` (`VISUAL`/`EDITOR`/`vi`); committed notes keep their newlines
- The fuzzy priority/recurrence choosers render one line per option without checkbox markers; the cursor highlight indicates the selection, and multi-select toggles keep the `☑` marker
- Multi-select choosers mark every row: committed options keep `☑` and uncommitted ones render `☐`, so the popup no longer reads as single-select
- Server-side sync polls Reminders every 2 seconds (was 10); an unchanged poll no longer rewrites the reminder panel
- The reminder panel follows sidebar navigation: moving the list selection (`j`/`k`, arrows, paging, mouse wheel) loads that list's reminders without pressing `Enter`; `Enter` still moves focus to the reminders panel
- Due-date entry is date-only: combined date/time text and hour offsets (`+2h`) are rejected with a pointer to the Time field
- `Tab` no longer accepts a date completion; completion moved to `Ctrl-Y` (`keys.calendar.complete`) so Tab always navigates fields
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
