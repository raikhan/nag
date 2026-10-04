# Step 1 — diagnosing the `Remind me` sync report

Date: 2026-10-04 · macOS 24.6.0 (darwin/arm64) · nag on AWST (+0800) ·
`github.com/BRO3886/go-eventkit v0.2.0`

> **Superseded conclusion.** The first version of this document ended with
> "nag writes the alarm; the reported defect does not reproduce". That was
> wrong: it proved nag's write survives EventKit without checking which
> store the Reminders app reads. The section "The two alert stores" below is
> the actual root cause, and the `Remind me` row has since been removed from
> nag. The probe results under "What was ruled out" are still accurate and are
> kept, because they are what narrowed the search.

## The report under test

A save from nag updated title, notes, date, time, priority and recurrence, but the
Reminders app's `early reminder` stayed `None`, across an app restart.

The planned decision tree had three branches — bridge drops it, app-display
limitation, or nag's own conversion is wrong. The first two probes ruled out the
first and third. Neither question was the right one to ask.

## The two alert stores

A reminder has **two independent places** where an alert can live:

| Store | Written by | Read by | Shown in the app as |
|---|---|---|---|
| `alarms` (EventKit `EKAlarm`) | nag, via go-eventkit | nag, EventKit | the `At a Time` checkbox; a relative offset is invisible there |
| `dueDateDeltaAlerts` (private `REMDueDateDeltaAlert`) | the Reminders app | the Reminders app only | `early reminder` |

EventKit has no method or property for the second store. Everything that touches
`dueDateDeltaAlert` lives in the private `ReminderKit.framework`: a runtime
sweep of ReminderKit, EventKit and CalendarDaemon for any selector or property
containing `early` found nothing outside that framework, and everything matching
`delta` is ReminderKit-only (`REMReminderDueDateDeltaAlertContext`,
`REMDueDateDeltaInterval`, `REMDueDateDeltaAlert`, `REMSaveRequest`).

Reading both stores for the two reminders from the report:

```
"test early reminder"                    (set in nag: 1 day before)
  EKAlarm:            rel=-86400 abs=nil    ← nag's write is stored
  dueDateDeltaAlerts: ()                    ← the app shows "None"

"test early reminder from the app"        (set in app: 2 days before)
  EKAlarm:            abs=2026-10-31 12:00 AWST  ← the app's "At a Time", at the due time
  dueDateDeltaAlerts: ()                       ← empty; the app now shows "None"
```

A throwaway reminder taken through every step an edit can perform confirms the
two stores never interact:

```
1. EventKit create                EK alarms=0   deltas=()
2. ReminderKit adds a -2d delta   EK alarms=0   deltas=(-2d)   ← EventKit cannot see it
3. EventKit title-only save       EK alarms=0   deltas=(-2d)
4. EventKit due-date change       EK alarms=0   deltas=(-2d)
5. EventKit alarm replace (-1h)   EK alarms=1   deltas=(-2d)
```

`REMDueDateDeltaInterval initWithUnit:count:` uses `0 = minutes, 1 = hours,
2 = days, 3 = weeks, 4 = months`, which matches the app's own chooser list.
Writing the value from an unsigned binary does work, via
`REMSaveRequest` → `updateReminder:` → `dueDateDeltaAlertContext` →
`addDueDateDeltaAlertWithDueDateDelta:`.

## What was ruled out

These probes are kept because they are what pointed at the store split. Each one
answered a question about the `EKAlarm` side, which turned out to be the side
that does not matter to the app's `early reminder` field.

**go-eventkit round-trips a relative alarm intact**, through both a direct
`CreateReminder` and nag's own `internal/reminders` layer: the alarm reads back
with the same `RelativeOffset` and a nil `AbsoluteDate`, before and after an
`UpdateReminder`. No fork of the dependency was ever warranted.

Driving the real binary through a pty — `j j tab j j e`, `a`, down × 9, enter,
`ctrl-s` — reported `Reminder updated` and left `rel=-24h0m0s` in the store. nag
does write what it was asked to write.

## Consequences for the app UI

Two separate display problems fell out of the same cause:

- nag's relative `-24h` alarm lands in `EKAlarm`, which the app's `early
  reminder` row does not read, so the app shows `None`.
- the app's own `At a Time` alarm is an absolute `EKAlarm` sitting exactly on the
  due instant. nag loaded it as `At due time`, a row that had already been
  removed from the chooser, so it reappeared whenever an app-created timed
  reminder was opened.

## What shipped

The `Remind me` row is removed from nag: the form field, its chooser, the
preset table, the free-form lead-time editor and its parser
(`timeentry.ParseLead`), the `Alarm` type and its EventKit conversions in
`internal/reminders`, and the `jump_alarm` binding. nag no longer reads or
writes alarms in either store, so it can no longer desynchronise them. The
README documents the limitation instead of the feature.

Writing the app's store through private ReminderKit was available and would
have made the feature work; it was rejected as an undocumented dependency on
Apple's private framework.

## Reproducing

The probes lived in `/tmp/ekdump` and were deleted once the diagnosis was
recorded. The store dump needs an EventKit `EKReminder` read for `alarms` and a
`REMStore` fetch with `REMReminderFetchOptions
fetchOptionsIncludingDueDateDeltaAlerts` for `dueDateDeltaAlerts`. Both require
Full Reminders access, which an unsigned binary only has once TCC has granted it
to the terminal.

Every probe wrote real reminders into the live Reminders store. Each one,
including the throwaway delta-alert reminder, was deleted afterwards; the store
was left holding only the pre-existing `Reminders` and `To do` lists.