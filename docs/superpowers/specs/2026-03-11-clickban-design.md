# clickban — Design Spec

## Overview

A Go TUI application using Bubble Tea + Lipgloss that integrates with the ClickUp API to present a lazygit-style kanban board and task manager for a specific ClickUp space.

## Authentication

- Personal API token read from `CLICKUP_API_TOKEN` environment variable
- Auto-detect current user identity via the ClickUp `/user` endpoint
- No OAuth flow required

## Target Space

This is a personal tool for a single workspace. Space and team IDs are configurable via environment variables with defaults for the primary workspace:

- `CLICKUP_TEAM_ID` (default: `9016771227`)
- `CLICKUP_SPACE_ID` (default: `90165823077`)
- Extracted from: `https://app.clickup.com/9016771227/v/s/90165823077`

## Leaf Task Resolution

Tasks displayed on the board are always the lowest-level subtask. The app recursively walks subtasks up to 5 levels deep. If a task has children, only the children are shown. If a task has no children, the task itself is shown. This applies to both the kanban and my-tasks views.

**Mechanism:** Use the `subtasks=true` query parameter on `GET /list/{list_id}/task` to include one level of subtasks. For deeper nesting, fetch subtasks of each task that has children via `GET /task/{task_id}?include_subtasks=true`, recursing until leaf tasks are found or depth limit (5) is reached.

## Views

### 1. Kanban View (default, key: `1`)

- Columns correspond to the statuses defined in the space
- Horizontally scrollable with `h`/`l` — active column stays visible
- Cards represent leaf tasks and display:
  - Title
  - Priority (color-coded left border)
  - Assignee(s)
  - Time tracked
- Right preview pane (~35% width) shows selected card details:
  - Title, priority/status/type badges, assignees, time tracked
  - Truncated description
- Move cards between columns with `m` key to update status via API

### 2. My Tasks View (key: `2`)

- Table listing all leaf tasks assigned to the current user
- Columns: task name, priority, status, time tracked
- Sorted by priority (Urgent > High > Normal > Low)
- Right preview pane (~35% width), same pattern as kanban view
- Navigate with `j`/`k`, open with `Enter`

### 3. Task Detail View (key: `Enter` on any card/row)

- Left panel (~65%):
  - Title with inline metadata badges (priority, status, task type)
  - Assignees and time tracked summary
  - Full description (scrollable)
- Right panel (~35%):
  - Scrollable comment thread (all users visible)
  - "Press `c` to add comment" prompt at bottom
- `Tab` switches focus between left and right panels

## Editable Fields

All edits from the task detail view. Each field has a dedicated keybind:

| Field       | Key | Control                                                              |
|-------------|-----|----------------------------------------------------------------------|
| Title       | `i` | Inline text edit                                                     |
| Description | `e` | Inline text area in TUI                                              |
| Description | `E` | Open in `$EDITOR` — writes to a temp file, opens editor, reads back on save, deletes temp file. If editor exits non-zero, discard changes. |
| Status      | `s` | Dropdown picker populated from space statuses                        |
| Priority    | `p` | Dropdown picker (Urgent, High, Normal, Low)                          |
| Task Type   | `y` | Dropdown picker populated via `GET /team/{team_id}/custom_item` (ClickUp custom task types endpoint) |
| Assignees   | `a` | Multi-select picker populated from workspace members                 |
| Time Entry  | `t` | Sub-menu with three modes (see below)                                |
| Comments    | `c` | Add new comment (from detail view)                                   |

### Comment Editing

From the comment thread (right panel, focused via `Tab`), navigate comments with `j`/`k`. On your own comments, press `e` to edit inline. Other users' comments are read-only.

### Time Entry Modes

Accessed via `t` from the detail view, presenting a sub-menu:

1. **Live timer** — start/stop a running timer
2. **Manual duration** — enter a duration string (e.g., "2h30m")
3. **Time range** — enter start and end times (e.g., "10:30am to now"). Assumes today's date unless a date is specified. Uses the system's local timezone. "now" resolves to current time.

## UI Style

- **Color palette:** Tokyo Night-inspired (dark background `#1a1a2e`, accents: blue `#7aa2f7`, red `#f7768e`, yellow `#e0af68`, green `#9ece6a`, purple `#bb9af7`)
- **Footer:** Context-sensitive keybind bar that updates based on current view/mode (lazygit pattern)
- **Navigation:** Vim-style — `h/j/k/l` for movement, `Esc` to go back, `Enter` to open/confirm
- **Borders and layout:** Lipgloss-rendered borders and panels
- **Priority indicators:** Color-coded left border on cards/rows (Urgent=red, High=yellow, Normal=green, Low=gray)

## Architecture

```
cmd/clickban/
  main.go                    — entrypoint, token loading, app initialization

internal/
  api/
    client.go                — HTTP client wrapper with auth header
    spaces.go                — space and status fetching
    tasks.go                 — task CRUD, subtask resolution
    comments.go              — comment thread fetching, create/edit
    time.go                  — time entry CRUD, timer start/stop
    users.go                 — current user, workspace members
  model/
    app.go                   — root Bubble Tea model, view switching
    kanban.go                — kanban board model
    mytasks.go               — my tasks table model
    detail.go                — task detail model
  ui/
    card.go                  — kanban card component
    table.go                 — table row component
    picker.go                — dropdown/multi-select picker
    footer.go                — context-sensitive keybind bar
    preview.go               — preview pane component
    editor.go                — inline text editor component
    timer.go                 — time entry input component
  config/
    config.go                — env var loading, space/team IDs
```

## Data Flow

1. **Startup:** Load token from env → validate token via `/user` → fetch space details (including folders and folderless lists) → fetch all tasks recursively per list → resolve leaf tasks → render kanban. If the token is missing or invalid, exit with a clear error message. If the space is inaccessible or network is unreachable, exit with a descriptive error.
2. **In-memory cache:** All fetched data cached locally. Press `r` to trigger a full refresh from the API.
3. **Mutations:** All field edits and status changes make ClickUp API calls. On success, update local state immediately. On failure, show error in a status bar and revert local state.

## ClickUp API Endpoints Used

| Endpoint                                    | Purpose                          |
|---------------------------------------------|----------------------------------|
| `GET /user`                                 | Get current user identity        |
| `GET /team/{team_id}/space`                 | Verify space access              |
| `GET /space/{space_id}`                     | Get space details and statuses   |
| `GET /space/{space_id}/folder`              | Get folders in space             |
| `GET /folder/{folder_id}/list`              | Get lists in folder              |
| `GET /space/{space_id}/list`                | Get folderless lists             |
| `GET /list/{list_id}/task`                  | Get tasks per list               |
| `GET /task/{task_id}`                       | Get task details                 |
| `GET /task/{task_id}?include_subtasks=true`  | Get task with subtasks           |
| `PUT /task/{task_id}`                       | Update task fields               |
| `GET /task/{task_id}/comment`               | Get comments                     |
| `POST /task/{task_id}/comment`              | Add comment                      |
| `PUT /comment/{comment_id}`                 | Edit own comment                 |
| `GET /task/{task_id}/time`                  | Get time entries                 |
| `POST /task/{task_id}/time`                 | Add time entry                   |
| `POST /team/{team_id}/time_entries/start`   | Start timer                      |
| `POST /team/{team_id}/time_entries/stop`    | Stop timer                       |
| `GET /team/{team_id}/member`               | Get workspace members            |
| `GET /team/{team_id}/custom_item`          | Get available task types         |

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/bubbles` — reusable components (textinput, viewport, list, table)
