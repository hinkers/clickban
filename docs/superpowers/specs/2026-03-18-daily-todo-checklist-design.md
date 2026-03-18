# Daily Todo Checklist — Design Spec

## Overview

A new "Today" view becomes the default opening view in clickban. It shows a prioritized list of tasks for the day, auto-calculated to fit a 7-hour workday. Tasks are selected by due date first, then priority. Users can force-add, ignore, or mark tasks as "done for today." A global warning indicator surfaces tasks missing priority or time estimates.

## New View: Today (ViewToday, index 0)

### Layout

Task list on left, preview pane on right (matching kanban/my-tasks pattern). The preview pane shows full task details for the currently selected item.

### Task List Items

Each row displays:
- Priority color on left border (urgent=red, high=orange, normal=blue, low=grey)
- Task name
- Remaining time estimate (time_estimate minus logged time_spent)
- Due date (relative: "today", "tomorrow", "Fri", "Mar 25")
- Current ClickUp status

### Header

`📋 Today — {weekday} {month} {day}` with capacity indicator on right: `{filled}h / 7.0h`

When over capacity: `⚠ {filled}h / 7.0h — over capacity by {excess}h`

### Task States on Today List

| State | Display | Capacity |
|-------|---------|----------|
| Active | Normal display | Counts toward 7h |
| Forced | Shown with `[+]` indicator | Counts toward 7h |
| Done for today | Struck through, dimmed | Excluded |
| Completed | Struck through (task closed in ClickUp) | Excluded |

### Keybindings

| Key | Action |
|-----|--------|
| `c` | Calculate today's list (run algorithm) |
| `f` | Force-add a task (opens picker of assigned tasks not on list) |
| `i` | Ignore selected task for today (remove from list) |
| `d` | Done for today (mark as handled, keep in ClickUp) |
| `enter` | Open task detail view |
| `j/k` | Navigate up/down |

## Auto-Calculate Algorithm

Triggered by pressing `c` on the Today view. Steps:

1. Start with all tasks assigned to current user that are not in a closed/done status.
2. Remove tasks marked as "ignored" for today.
3. Include any tasks marked as "forced" unconditionally.
4. Sort remaining tasks:
   - **First tier:** Tasks due today or overdue (sorted by priority within this tier).
   - **Second tier:** All other tasks sorted by priority (urgent → high → normal → low), with due date as tiebreaker within the same priority level.
5. Calculate remaining time per task: `time_estimate - time_spent` (using ClickUp time entry data). If time_spent >= time_estimate, remaining is 0 (task still shows but doesn't consume capacity).
6. Fill the list up to 7 hours of remaining time.
7. Tasks with no time estimate are included but do not count toward the 7-hour capacity (the validation warning encourages fixing this).
8. If due-today/overdue tasks alone exceed 7 hours, include them all and show an over-capacity warning. Do not hide due tasks to fit the cap.
9. Underfill is acceptable — if only 4 hours of work qualifies, show 4 hours. Do not pad with filler tasks.

### Multi-Day Tasks

Tasks with large time estimates that span multiple days are handled naturally:
- Remaining time = `time_estimate - time_spent` (pulled from ClickUp time entries).
- If you log 3 hours today on a 20-hour task, tomorrow's calculation sees 17 hours remaining.
- "Done for today" removes it from today's view without affecting the ClickUp task status.

## Today State Persistence (SQLite)

Extend the existing SQLite cache database at `~/.cache/clickban/clickban.db`.

### Schema

```sql
CREATE TABLE IF NOT EXISTS today_state (
    task_id  TEXT PRIMARY KEY,
    action   TEXT NOT NULL,    -- 'forced', 'ignored', 'done_for_day'
    date     TEXT NOT NULL     -- YYYY-MM-DD format
);
```

### Behavior

- On app startup, delete all rows where `date != today's date` (auto-expire previous days).
- Force-add inserts a row with action `'forced'`.
- Ignore inserts a row with action `'ignored'`.
- Done-for-today inserts a row with action `'done_for_day'`.
- Completing a task (status change to closed/done) removes it from the list via normal status filtering.
- Recalculating (`c`) preserves forced/ignored/done_for_day state — it only recalculates the "active" portion.

## Validation Indicator

### Global Footer Warning

Visible from **all views** in the footer/status bar:

`⚠ N tasks need data`

Shown when any task assigned to the current user (with a non-closed status) is missing:
- Priority (nil), OR
- Time estimate (0 or unset)

When no tasks are missing data, the indicator is hidden.

### My Tasks Filter

On the My Tasks view, pressing `!` toggles a filter that shows **only** tasks missing priority or time estimate. This lets the user quickly find and fix incomplete tasks via the task detail view.

When the filter is active, show an indicator in the My Tasks header: `[!] Showing tasks needing data`

## New Fields on Existing Views

### Kanban Cards

Add to each card below existing content:
- Time estimate (e.g., `2.0h est`)
- Due date (e.g., `due Fri`)

Only shown when the field has a value. Cards remain compact when fields are empty.

### Task Detail View

Add two new fields to the detail info panel:
- **Time Estimate** — displayed as hours (e.g., `2.0h`), editable
- **Due Date** — displayed as date (e.g., `Mar 25, 2026`), editable

These fields use the existing `UpdateTaskRequest` which already supports `time_estimate` and `due_date`.

## View Numbering Changes

| Key | View | Notes |
|-----|------|-------|
| `0` or `t` | Today | New default on app open |
| `1` | Kanban | Was default, now second |
| `2` | My Tasks | Unchanged key |

The `ViewMode` enum shifts:
- `ViewToday = 0` (new)
- `ViewKanban = 1` (was 0)
- `ViewMyTasks = 2` (was 1)
- `ViewDetail = 3` (was 2)

## Data Requirements

### Already Available

- `Task.Priority` — priority struct with level name
- `Task.TimeEstimate` — int64 milliseconds
- `Task.TimeSpent` — int64 milliseconds (from task list endpoint)
- `Task.DueDate` — string timestamp
- `Task.Assignees` — user list
- `Task.Status` — current status
- `AppState.CurrentUser` — for filtering "my" tasks

### Time Tracking

The auto-calculate algorithm uses `Task.TimeSpent` which is returned by the ClickUp task list endpoint. No additional API calls needed for the basic calculation. The existing time tracking features (logging time via `t` in detail view) will update this value on next data refresh.

## Architecture

### New Files

- `internal/model/today.go` — Today view model (Bubble Tea model with Update/View)

### Modified Files

- `internal/model/app.go` — Add ViewToday, shift view indices, route to today model, add validation count to footer
- `internal/cache/cache.go` — Add today_state table methods (SetTodayAction, GetTodayActions, ClearExpiredTodayState)
- `internal/model/kanban.go` — Add time estimate and due date to card rendering
- `internal/model/detail.go` — Add time estimate and due date fields with editing
- `internal/model/mytasks.go` — Add `!` keybind for validation filter toggle
- `internal/ui/card.go` — Update card rendering to include new fields
- `internal/ui/footer.go` — Add validation warning indicator
