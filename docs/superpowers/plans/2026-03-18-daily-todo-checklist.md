# Daily Todo Checklist Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Today" view as the default opening view, with auto-calculated daily task list, force/ignore/done-for-today actions, validation warnings for incomplete task data, and time estimate/due date fields on existing views.

**Architecture:** New `today.go` model follows the same Bubble Tea pattern as `kanban.go` and `mytasks.go`. Today state (forced/ignored/done_for_day) persists in the existing SQLite cache database via a new `today_state` table. The auto-calculate algorithm runs on-demand via keybind, filling a 7-hour day by due date priority then task priority. Shared helpers (`isClosedStatus`, `priorityRank`, `formatTimestamp`) are extracted to avoid duplication.

**Tech Stack:** Go, Bubble Tea, Lipgloss, SQLite (modernc.org/sqlite)

**Spec:** `docs/superpowers/specs/2026-03-18-daily-todo-checklist-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/model/helpers.go` | Create | Shared helpers extracted from kanban/mytasks: `isClosedStatus()`, `priorityRank()`, `priorityDisplay()`, `parseDueDate()`, `formatRelativeDate()` |
| `internal/model/today.go` | Create | Today view model — list rendering, calculate algorithm, force/ignore/done keybinds |
| `internal/model/today_test.go` | Create | Tests for the calculate algorithm and today state logic |
| `internal/cache/cache.go` | Modify | Add `today_state` table creation in `Open()`, add `SetTodayAction()`, `GetTodayActions()`, `ClearExpiredTodayState()` methods |
| `internal/cache/cache_test.go` | Create | Tests for today_state persistence methods |
| `internal/model/app.go` | Modify | Add `ViewToday`, shift enum, update header tabs, route to today model, pass cache to today, add validation count |
| `internal/ui/card.go` | Modify | Add time estimate and due date lines to card rendering |
| `internal/ui/card_test.go` | Modify | Add tests for new card fields |
| `internal/model/detail.go` | Modify | Add time estimate and due date display + editing keybinds |
| `internal/model/mytasks.go` | Modify | Add `!` keybind for validation filter toggle |
| `internal/ui/footer.go` | Modify | Add `RenderValidationWarning()` function |
| `internal/model/kanban.go` | Modify | Remove duplicated `isClosedStatus()`, import from helpers |

---

### Task 1: Extract shared helpers

**Files:**
- Create: `internal/model/helpers.go`
- Modify: `internal/model/kanban.go` (remove `isClosedStatus` at lines 592-596)
- Modify: `internal/model/mytasks.go` (remove `priorityRank` at lines 247-263, `priorityDisplay` at lines 265-280)

- [ ] **Step 1: Create helpers.go with extracted functions**

```go
// internal/model/helpers.go
package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// isClosedStatus returns true if the status type indicates a closed/done task.
func isClosedStatus(s api.Status) bool {
	t := strings.ToLower(s.Type)
	return t == "closed" || t == "done"
}

// priorityRank returns a numeric rank for sorting (lower = higher priority).
// 1=urgent, 2=high, 3=normal, 4=low, 99=none.
func priorityRank(t api.Task) int {
	if t.Priority == nil {
		return 99
	}
	switch strings.ToLower(t.Priority.Priority) {
	case "urgent":
		return 1
	case "high":
		return 2
	case "normal":
		return 3
	case "low":
		return 4
	default:
		return 99
	}
}

// priorityDisplay returns (label, color) for rendering a task's priority.
func priorityDisplay(t api.Task) (string, string) {
	if t.Priority == nil {
		return "  None", ui.ColorFgDim
	}
	switch strings.ToLower(t.Priority.Priority) {
	case "urgent":
		return "!! Urgent", ui.ColorRed
	case "high":
		return "!  High", ui.ColorYellow
	case "normal":
		return "   Normal", ui.ColorGreen
	case "low":
		return "   Low", ui.ColorFgDim
	default:
		return "   " + t.Priority.Priority, ui.ColorFgDim
	}
}

// parseDueDate parses a ClickUp millisecond timestamp string into a time.Time.
// Returns zero time and false if the string is empty or unparseable.
func parseDueDate(ts string) (time.Time, bool) {
	if ts == "" {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms), true
}

// isDueOrOverdue returns true if the task is due today or before today.
func isDueOrOverdue(t api.Task) bool {
	due, ok := parseDueDate(t.DueDate)
	if !ok {
		return false
	}
	now := time.Now()
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return !due.After(endOfToday)
}

// formatRelativeDate formats a due date relative to today.
// Returns "today", "tomorrow", weekday name (if within 7 days), or "Jan 2" format.
func formatRelativeDate(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	days := int(dueDay.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return fmt.Sprintf("%d days ago", -days)
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days < 7:
		return t.Format("Mon")
	default:
		return t.Format("Jan 2")
	}
}

// remainingTimeMs returns the remaining time estimate in milliseconds.
// Returns 0 if time_spent >= time_estimate or if time_estimate is 0.
func remainingTimeMs(t api.Task) int64 {
	if t.TimeEstimate <= 0 {
		return 0
	}
	remaining := t.TimeEstimate - t.TimeSpent
	if remaining < 0 {
		return 0
	}
	return remaining
}

// taskNeedsData returns true if a task assigned to the user is missing priority or time estimate.
func taskNeedsData(t api.Task) bool {
	return t.Priority == nil || t.TimeEstimate <= 0
}
```

- [ ] **Step 2: Update kanban.go to use shared helpers**

In `internal/model/kanban.go`, delete the `isClosedStatus` function (lines 592-596). All call sites already reference it as just `isClosedStatus(...)` which will resolve to the package-level function in `helpers.go`.

- [ ] **Step 3: Update mytasks.go to use shared helpers**

In `internal/model/mytasks.go`, delete the `priorityRank` function (lines 247-263) and `priorityDisplay` function (lines 265-280). Call sites in the same package will resolve to `helpers.go`.

- [ ] **Step 4: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: All pass — no behavior change, just extraction.

- [ ] **Step 5: Commit**

```bash
git add internal/model/helpers.go internal/model/kanban.go internal/model/mytasks.go
git commit -m "refactor: extract shared helpers from kanban and mytasks"
```

---

### Task 2: Add today_state persistence to cache

**Files:**
- Modify: `internal/cache/cache.go`
- Create: `internal/cache/cache_test.go`

- [ ] **Step 1: Write failing tests for today_state methods**

```go
// internal/cache/cache_test.go
package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Create the cache dir so Open() works
	os.MkdirAll(filepath.Join(dir, ".cache", "clickban"), 0o755)

	c, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSetAndGetTodayActions(t *testing.T) {
	c := testCache(t)
	today := time.Now().Format("2006-01-02")

	if err := c.SetTodayAction("task1", "forced", today); err != nil {
		t.Fatalf("SetTodayAction: %v", err)
	}
	if err := c.SetTodayAction("task2", "ignored", today); err != nil {
		t.Fatalf("SetTodayAction: %v", err)
	}

	actions, err := c.GetTodayActions(today)
	if err != nil {
		t.Fatalf("GetTodayActions: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions["task1"] != "forced" {
		t.Errorf("task1: expected forced, got %s", actions["task1"])
	}
	if actions["task2"] != "ignored" {
		t.Errorf("task2: expected ignored, got %s", actions["task2"])
	}
}

func TestClearExpiredTodayState(t *testing.T) {
	c := testCache(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	c.SetTodayAction("old_task", "forced", yesterday)
	c.SetTodayAction("new_task", "forced", today)

	if err := c.ClearExpiredTodayState(today); err != nil {
		t.Fatalf("ClearExpiredTodayState: %v", err)
	}

	actions, _ := c.GetTodayActions(today)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action after clear, got %d", len(actions))
	}
	if _, ok := actions["new_task"]; !ok {
		t.Error("new_task should survive clear")
	}
}

func TestRemoveTodayAction(t *testing.T) {
	c := testCache(t)
	today := time.Now().Format("2006-01-02")

	c.SetTodayAction("task1", "forced", today)
	if err := c.RemoveTodayAction("task1"); err != nil {
		t.Fatalf("RemoveTodayAction: %v", err)
	}

	actions, _ := c.GetTodayActions(today)
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions after remove, got %d", len(actions))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cache/ -v`
Expected: FAIL — methods don't exist yet.

- [ ] **Step 3: Add today_state table creation to Open()**

In `internal/cache/cache.go`, after the existing `CREATE TABLE IF NOT EXISTS cache` statement (around line 44), add:

```go
_, err = db.Exec(`CREATE TABLE IF NOT EXISTS today_state (
	task_id TEXT PRIMARY KEY,
	action  TEXT NOT NULL,
	date    TEXT NOT NULL
)`)
if err != nil {
	db.Close()
	return nil, fmt.Errorf("create today_state table: %w", err)
}
```

- [ ] **Step 4: Add SetTodayAction method**

Add to `internal/cache/cache.go`:

```go
// SetTodayAction inserts or replaces a today action for a task.
func (c *Cache) SetTodayAction(taskID, action, date string) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO today_state (task_id, action, date) VALUES (?, ?, ?)`,
		taskID, action, date,
	)
	return err
}
```

- [ ] **Step 5: Add GetTodayActions method**

Add to `internal/cache/cache.go`:

```go
// GetTodayActions returns a map of task_id -> action for the given date.
func (c *Cache) GetTodayActions(date string) (map[string]string, error) {
	rows, err := c.db.Query(`SELECT task_id, action FROM today_state WHERE date = ?`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := make(map[string]string)
	for rows.Next() {
		var taskID, action string
		if err := rows.Scan(&taskID, &action); err != nil {
			return nil, err
		}
		actions[taskID] = action
	}
	return actions, rows.Err()
}
```

- [ ] **Step 6: Add ClearExpiredTodayState and RemoveTodayAction methods**

Add to `internal/cache/cache.go`:

```go
// ClearExpiredTodayState deletes all today_state rows not matching the given date.
func (c *Cache) ClearExpiredTodayState(today string) error {
	_, err := c.db.Exec(`DELETE FROM today_state WHERE date != ?`, today)
	return err
}

// RemoveTodayAction deletes a specific task's today action.
func (c *Cache) RemoveTodayAction(taskID string) error {
	_, err := c.db.Exec(`DELETE FROM today_state WHERE task_id = ?`, taskID)
	return err
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/cache/ -v`
Expected: All PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cache/cache.go internal/cache/cache_test.go
git commit -m "feat: add today_state persistence to SQLite cache"
```

---

### Task 3: Add time estimate and due date to kanban cards

**Files:**
- Modify: `internal/ui/card.go` (lines 23-115)
- Modify: `internal/ui/card_test.go`

- [ ] **Step 1: Write failing test for new card fields**

Add to `internal/ui/card_test.go`:

```go
func TestRenderCard_WithTimeEstimateAndDueDate(t *testing.T) {
	task := api.Task{
		Name:         "Test Task",
		TimeEstimate: 7200000, // 2 hours
		DueDate:      fmt.Sprintf("%d", time.Now().UnixMilli()), // due today
	}
	result := RenderCard(task, 30, false)
	if !strings.Contains(result, "2h0m est") {
		t.Errorf("expected time estimate in card, got:\n%s", result)
	}
	if !strings.Contains(result, "due") {
		t.Errorf("expected due date in card, got:\n%s", result)
	}
}

func TestRenderCard_WithoutTimeEstimateOrDueDate(t *testing.T) {
	task := api.Task{Name: "No Data Task"}
	result := RenderCard(task, 30, false)
	if strings.Contains(result, "est") {
		t.Errorf("should not show estimate when absent")
	}
	if strings.Contains(result, "due") {
		t.Errorf("should not show due date when absent")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestRenderCard_With -v`
Expected: FAIL.

- [ ] **Step 3: Add time estimate and due date to RenderCard**

In `internal/ui/card.go`, in the `RenderCard` function, after the time spent line (around line 95 where `⏱` is rendered), add:

```go
// Time estimate
if task.TimeEstimate > 0 {
	estStr := FormatDuration(task.TimeEstimate) + " est"
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFgDim)).Render("⏳ "+estStr))
}

// Due date — uses model.parseDueDate and model.formatRelativeDate helpers.
// Since card.go is in the ui package, we need to add equivalent helpers here
// or accept the package boundary. Add these two small functions to card.go:
if task.DueDate != "" {
	if ms, err := strconv.ParseInt(task.DueDate, 10, 64); err == nil {
		due := time.UnixMilli(ms)
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
		days := int(dueDay.Sub(today).Hours() / 24)
		var label string
		switch {
		case days < 0:
			label = fmt.Sprintf("%d days ago", -days)
		case days == 0:
			label = "today"
		case days == 1:
			label = "tomorrow"
		case days < 7:
			label = due.Format("Mon")
		default:
			label = due.Format("Jan 2")
		}
		dueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFgDim))
		if days <= 0 {
			dueStyle = dueStyle.Foreground(lipgloss.Color(ColorRed))
		}
		lines = append(lines, dueStyle.Render("📅 due "+label))
	}
}
```

Add `"fmt"`, `"strconv"`, and `"time"` to the imports.

Note: The date formatting logic is duplicated between `ui/card.go` and `model/helpers.go` due to package boundaries (`ui` cannot import `model`). This is acceptable since the logic is small and the packages should not have circular dependencies.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/ -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/card.go internal/ui/card_test.go
git commit -m "feat: add time estimate and due date to kanban cards"
```

---

### Task 4: Add time estimate and due date editing to detail view

**Files:**
- Modify: `internal/model/detail.go`

- [ ] **Step 1: Add OverlayTimeEstimate and OverlayDueDate to the overlay enum**

In `internal/model/detail.go`, add to the `DetailOverlay` enum (after `OverlayTimer`):

```go
OverlayTimeEstimate
OverlayDueDate
```

- [ ] **Step 2: Add keybindings for time estimate and due date editing**

In `updateMain()` (around line 260 where existing keybinds are), add cases:

```go
case "T":
	// Edit time estimate
	d.overlay = OverlayTimeEstimate
	d.editor = ui.NewEditor("Time Estimate", "")
	return d, d.editor.Init()
case "D":
	// Edit due date
	d.overlay = OverlayDueDate
	d.editor = ui.NewEditor("Due Date (YYYY-MM-DD)", "")
	return d, d.editor.Init()
```

- [ ] **Step 3: Handle editor results for time estimate and due date**

In `handleEditorResult()`, add cases:

```go
case OverlayTimeEstimate:
	// Parse duration string like "2h30m", "3h", "45m"
	ms, err := ui.ParseDuration(result.Value)
	if err != nil {
		d.statusMsg = "Invalid duration: " + result.Value
		return d, nil
	}
	d.task.TimeEstimate = ms
	updated := d.task
	d.updatedTask = &updated
	taskID := d.task.ID
	return d, func() tea.Msg {
		client := d.state.Client
		err := client.UpdateTask(taskID, &api.UpdateTaskRequest{TimeEstimate: &ms})
		if err != nil {
			return StatusMsg{Text: "time estimate update failed: " + err.Error()}
		}
		return StatusMsg{Text: "Time estimate updated"}
	}

case OverlayDueDate:
	// Parse date string like "2026-03-25"
	t, err := time.Parse("2006-01-02", strings.TrimSpace(result.Value))
	if err != nil {
		d.statusMsg = "Invalid date (use YYYY-MM-DD): " + result.Value
		return d, nil
	}
	ms := t.UnixMilli()
	d.task.DueDate = fmt.Sprintf("%d", ms)
	updated := d.task
	d.updatedTask = &updated
	taskID := d.task.ID
	return d, func() tea.Msg {
		client := d.state.Client
		err := client.UpdateTask(taskID, &api.UpdateTaskRequest{DueDate: &ms})
		if err != nil {
			return StatusMsg{Text: "due date update failed: " + err.Error()}
		}
		return StatusMsg{Text: "Due date updated"}
	}
```

- [ ] **Step 4: Add time estimate and due date to renderMain()**

In `renderMain()`, after the time spent line (around line 685), add:

```go
// Time estimate
if d.task.TimeEstimate > 0 {
	sb.WriteString(labelStyle.Render("Estimate: "))
	sb.WriteString(valueStyle.Render(ui.FormatDuration(d.task.TimeEstimate)))
	sb.WriteString("\n")
}

// Due date
if d.task.DueDate != "" {
	if ms, err := strconv.ParseInt(d.task.DueDate, 10, 64); err == nil {
		due := time.UnixMilli(ms)
		sb.WriteString(labelStyle.Render("Due: "))
		sb.WriteString(valueStyle.Render(due.Format("Jan 2, 2006")))
		sb.WriteString("\n")
	}
}
```

- [ ] **Step 5: Add keybindings to footer**

In the footer keybindings section of `View()` (around line 560), add `T` and `D`:

```go
{Key: "T", Label: "estimate"},
{Key: "D", Label: "due date"},
```

- [ ] **Step 6: Add delegateToOverlay routing for new overlays**

In `delegateToOverlay()`, ensure `OverlayTimeEstimate` and `OverlayDueDate` route to `d.editor.Update(msg)`.

- [ ] **Step 7: Add renderWithOverlay routing for new overlays**

In `renderWithOverlay()`, ensure `OverlayTimeEstimate` and `OverlayDueDate` render `d.editor.View()`.

- [ ] **Step 8: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass.

- [ ] **Step 9: Commit**

```bash
git add internal/model/detail.go
git commit -m "feat: add time estimate and due date editing to detail view"
```

---

### Task 5: Add validation warning to footer

**Files:**
- Modify: `internal/ui/footer.go`

- [ ] **Step 1: Add RenderValidationWarning function**

Add to `internal/ui/footer.go`:

```go
// RenderValidationWarning renders "⚠ N tasks need data" if count > 0.
// Returns empty string if count is 0.
func RenderValidationWarning(count int) string {
	if count <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorYellow)).
		Render(fmt.Sprintf("⚠ %d tasks need data", count))
}
```

Add `"fmt"` to imports.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: Compiles.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/footer.go
git commit -m "feat: add validation warning renderer to footer"
```

---

### Task 6: Add validation filter to My Tasks

**Files:**
- Modify: `internal/model/mytasks.go`

- [ ] **Step 1: Add needsDataFilter field and toggle**

Add `needsDataFilter bool` to the `MyTasks` struct.

In `Update()`, add a keybind handler for `!`:

```go
case "!":
	m.needsDataFilter = !m.needsDataFilter
	m.tasks = m.filterTasks()
	if m.cursor >= len(m.tasks) {
		m.cursor = max(0, len(m.tasks)-1)
	}
	return m, nil
```

- [ ] **Step 2: Refactor filterAndSortMyTasks to support the filter**

Rename `filterAndSortMyTasks` to a method `filterTasks()` on `MyTasks` that checks `m.needsDataFilter`:

```go
func (m *MyTasks) filterTasks() []api.Task {
	var tasks []api.Task
	for _, t := range m.state.Tasks {
		isAssigned := false
		for _, a := range t.Assignees {
			if m.state.CurrentUser != nil && a.ID == m.state.CurrentUser.ID {
				isAssigned = true
				break
			}
		}
		if !isAssigned {
			continue
		}
		if m.needsDataFilter && !taskNeedsData(t) {
			continue
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool {
		return priorityRank(tasks[i]) < priorityRank(tasks[j])
	})
	return tasks
}
```

Update `NewMyTasks` to call `m.filterTasks()` after setting state.

- [ ] **Step 3: Add filter indicator to header**

In `renderTable()`, if `m.needsDataFilter` is true, append `" [!] Showing tasks needing data"` to the header row in dim yellow.

- [ ] **Step 4: Add `!` to keybindings**

In `keyBindings()`, add:

```go
{Key: "!", Label: "needs data"},
```

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add internal/model/mytasks.go
git commit -m "feat: add validation filter toggle to My Tasks view"
```

---

### Task 7: Update app.go — view numbering and validation count

**Files:**
- Modify: `internal/model/app.go`

- [ ] **Step 1: Shift ViewMode enum**

Change the enum at the top of `app.go`:

```go
const (
	ViewToday   ViewMode = iota // 0
	ViewKanban                   // 1
	ViewMyTasks                  // 2
	ViewDetail                   // 3
)
```

- [ ] **Step 2: Add today field to App struct**

Add to the `App` struct:

```go
today    Today
cache    *cache.Cache
```

Import `"github.com/hinkers/clickban/internal/cache"`.

- [ ] **Step 3: Update NewApp to set default view to ViewToday**

Change: `view: ViewKanban` → `view: ViewToday`

- [ ] **Step 4: Update Init() to open cache and pass it**

The cache is already opened in `Init()`. Store it on the App struct so the Today model can use it. After `c, err := cache.Open()`, add `a.cache = c`.

- [ ] **Step 5: Update DataLoadedMsg handler to create Today model**

After the existing kanban/myTasks creation in the `DataLoadedMsg` handler, add:

```go
a.today = NewToday(a.state, a.cache).Resize(a.width, a.height)
```

- [ ] **Step 6: Update key handlers for view switching**

Replace the existing "1"/"2" handlers:

```go
case "0", "t":
	if a.view != ViewDetail {
		a.view = ViewToday
	}
case "1":
	if a.view != ViewDetail {
		a.view = ViewKanban
	}
case "2":
	if a.view != ViewDetail {
		a.view = ViewMyTasks
	}
```

- [ ] **Step 7: Route Update messages to today model**

In the `Update()` method, add routing for `ViewToday` alongside the existing kanban/myTasks routing:

```go
case ViewToday:
	a.today, cmd = a.today.Update(msg)
	if task := a.today.WantsDetail(); task != nil {
		a.detail = NewDetail(task, &a.state)
		a.view = ViewDetail
		a.today.ClearWantsDetail()
		cmds = append(cmds, a.detail.Init())
	}
```

- [ ] **Step 8: Route View to today model**

In `View()`, add the `ViewToday` case:

```go
case ViewToday:
	body = a.today.View()
```

- [ ] **Step 9: Update renderHeader tabs**

Update `renderHeader()` to include the Today tab. Change the tab labels array to:

```go
tabs := []struct {
	key  string
	view ViewMode
}{
	{"0", ViewToday},
	{"1", ViewKanban},
	{"2", ViewMyTasks},
}
```

Render each tab as `[key] Name` with the active view highlighted.

- [ ] **Step 10: Add validation count to header/footer**

Add a helper to count tasks needing data:

```go
func (a *App) validationCount() int {
	count := 0
	for _, t := range a.state.Tasks {
		if a.state.CurrentUser == nil {
			break
		}
		isAssigned := false
		for _, u := range t.Assignees {
			if u.ID == a.state.CurrentUser.ID {
				isAssigned = true
				break
			}
		}
		if isAssigned && !isClosedStatus(t.Status) && taskNeedsData(t) {
			count++
		}
	}
	return count
}
```

Call `ui.RenderValidationWarning(a.validationCount())` in the header or footer render, appending it to the right side of the status bar.

- [ ] **Step 11: Update propagateDetailUpdates to rebuild today**

In `propagateDetailUpdates()`, after rebuilding kanban and myTasks, add:

```go
a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height)
```

- [ ] **Step 12: Update DataLoadedMsg (FromCache) handler to rebuild today**

In the `DataLoadedMsg` handler where `FromCache == true` triggers a background refresh, ensure the today model is also rebuilt after refresh with preserved state.

- [ ] **Step 13: Build**

Run: `go build ./...`
Expected: Will fail because `Today` model doesn't exist yet. That's expected — this task can be committed alongside Task 8.

---

### Task 8: Create the Today view model

**Files:**
- Create: `internal/model/today.go`
- Create: `internal/model/today_test.go`

- [ ] **Step 1: Write tests for the calculate algorithm**

```go
// internal/model/today_test.go
package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/hinkers/clickban/internal/api"
)

func makeTask(id, name string, priority int, estimateHrs float64, dueDaysFromNow int) api.Task {
	t := api.Task{
		ID:           id,
		Name:         name,
		TimeEstimate: int64(estimateHrs * 3600000),
	}
	if priority > 0 {
		priNames := map[int]string{1: "urgent", 2: "high", 3: "normal", 4: "low"}
		t.Priority = &api.Priority{Priority: priNames[priority]}
	}
	if dueDaysFromNow >= 0 {
		due := time.Now().AddDate(0, 0, dueDaysFromNow)
		t.DueDate = fmt.Sprintf("%d", due.UnixMilli())
	}
	t.Status = api.Status{Status: "open", Type: "open"}
	return t
}

func TestCalculate_PriorityOrder(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Low", 4, 1, -1),
		makeTask("2", "Urgent", 1, 2, 30),
		makeTask("3", "High", 2, 1.5, 30),
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) < 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result))
	}
	// Due/overdue first, then by priority
	if result[0].ID != "1" {
		t.Errorf("expected overdue task first, got %s", result[0].ID)
	}
	if result[1].ID != "2" {
		t.Errorf("expected urgent second, got %s", result[1].ID)
	}
}

func TestCalculate_CapAt7Hours(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Big", 1, 4, 30),
		makeTask("2", "Medium", 2, 3, 30),
		makeTask("3", "Small", 3, 2, 30),
	}
	result := calculateTodayList(tasks, map[string]string{})
	// 4 + 3 = 7, should include first two but not third
	if len(result) != 2 {
		t.Fatalf("expected 2 tasks (7h cap), got %d", len(result))
	}
}

func TestCalculate_DueTodayAlwaysIncluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Due today big", 4, 10, 0), // 10 hours, due today
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) != 1 {
		t.Fatalf("due-today task must always be included")
	}
}

func TestCalculate_IgnoredExcluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Task A", 1, 2, 30),
		makeTask("2", "Task B", 2, 2, 30),
	}
	actions := map[string]string{"1": "ignored"}
	result := calculateTodayList(tasks, actions)
	if len(result) != 1 || result[0].ID != "2" {
		t.Errorf("ignored task should be excluded")
	}
}

func TestCalculate_ForcedAlwaysIncluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Forced low", 4, 1, 30),
		makeTask("2", "Urgent big", 1, 7, 30),
	}
	actions := map[string]string{"1": "forced"}
	result := calculateTodayList(tasks, actions)
	// Forced task should be in the list regardless
	found := false
	for _, t := range result {
		if t.ID == "1" {
			found = true
		}
	}
	if !found {
		t.Error("forced task must be included")
	}
}

func TestCalculate_NoEstimateIncludedButNotCounted(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Has est", 1, 7, 30),
		makeTask("2", "No est", 2, 0, 30), // 0 estimate
	}
	result := calculateTodayList(tasks, map[string]string{})
	// Task with 7h fills capacity, but no-estimate task should still be included
	if len(result) != 2 {
		t.Fatalf("no-estimate task should be included, got %d tasks", len(result))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run TestCalculate -v`
Expected: FAIL — `calculateTodayList` doesn't exist.

- [ ] **Step 3: Create today.go with the calculate algorithm**

```go
// internal/model/today.go
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/cache"
	"github.com/hinkers/clickban/internal/ui"
)

const dailyCapacityMs = 7 * 3600 * 1000 // 7 hours in milliseconds

// TodayItem wraps a task with its today-specific state.
type TodayItem struct {
	Task   api.Task
	Action string // "", "forced", "ignored", "done_for_day"
}

// Today is the Bubble Tea model for the daily todo view.
type Today struct {
	state        AppState
	cache        *cache.Cache
	items        []TodayItem
	todayActions map[string]string // task_id -> action from SQLite
	cursor       int
	wantsDetail  *api.Task
	calculated   bool // whether calculate has been run
	forcePicker  *ui.Picker
	width        int
	height       int
}

// NewToday creates a new Today model.
func NewToday(state AppState, c *cache.Cache) Today {
	t := Today{
		state:        state,
		cache:        c,
		todayActions: make(map[string]string),
	}
	t.loadActions()
	return t
}

// NewTodayWithState creates a Today model preserving existing actions.
func NewTodayWithState(state AppState, c *cache.Cache, actions map[string]string) Today {
	t := Today{
		state:        state,
		cache:        c,
		todayActions: actions,
		calculated:   true,
	}
	if t.todayActions == nil {
		t.todayActions = make(map[string]string)
	}
	t.items = buildTodayItems(calculateTodayList(t.myOpenTasks(), t.todayActions), t.todayActions)
	return t
}

func (t *Today) loadActions() {
	if t.cache == nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	t.cache.ClearExpiredTodayState(today)
	actions, err := t.cache.GetTodayActions(today)
	if err != nil {
		return
	}
	t.todayActions = actions
}

// Resize sets the terminal dimensions.
func (t Today) Resize(w, h int) Today {
	t.width = w
	t.height = h
	return t
}

// SelectedTask returns the currently highlighted task.
func (t *Today) SelectedTask() *api.Task {
	active := t.activeItems()
	if len(active) == 0 {
		return nil
	}
	idx := t.cursor
	if idx >= len(active) {
		idx = len(active) - 1
	}
	task := active[idx].Task
	return &task
}

// WantsDetail returns a task if the user pressed enter.
func (t *Today) WantsDetail() *api.Task {
	return t.wantsDetail
}

// ClearWantsDetail resets the detail navigation flag.
func (t *Today) ClearWantsDetail() {
	t.wantsDetail = nil
}

// HasOverlay returns true if a picker is open.
func (t *Today) HasOverlay() bool {
	return t.forcePicker != nil
}

// activeItems returns items that are not ignored (visible in the list).
func (t *Today) activeItems() []TodayItem {
	var active []TodayItem
	for _, item := range t.items {
		if item.Action != "ignored" {
			active = append(active, item)
		}
	}
	return active
}

// myOpenTasks returns tasks assigned to the current user that aren't closed.
func (t *Today) myOpenTasks() []api.Task {
	var tasks []api.Task
	for _, task := range t.state.Tasks {
		if isClosedStatus(task.Status) {
			continue
		}
		isAssigned := false
		if t.state.CurrentUser != nil {
			for _, a := range task.Assignees {
				if a.ID == t.state.CurrentUser.ID {
					isAssigned = true
					break
				}
			}
		}
		if isAssigned {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// calculateTodayList runs the auto-calculate algorithm.
// Returns tasks in priority order, respecting due dates, capacity, and actions.
func calculateTodayList(tasks []api.Task, actions map[string]string) []api.Task {
	var forced []api.Task
	var candidates []api.Task

	for _, t := range tasks {
		action := actions[t.ID]
		if action == "ignored" || action == "done_for_day" {
			continue
		}
		if action == "forced" {
			forced = append(forced, t)
			continue
		}
		candidates = append(candidates, t)
	}

	// Sort candidates: due today/overdue first (by priority), then rest by priority (due date tiebreak)
	sort.SliceStable(candidates, func(i, j int) bool {
		iDue := isDueOrOverdue(candidates[i])
		jDue := isDueOrOverdue(candidates[j])
		if iDue != jDue {
			return iDue // due/overdue first
		}
		iPri := priorityRank(candidates[i])
		jPri := priorityRank(candidates[j])
		if iPri != jPri {
			return iPri < jPri
		}
		// Due date tiebreaker (earlier due date first)
		iDate, iOk := parseDueDate(candidates[i].DueDate)
		jDate, jOk := parseDueDate(candidates[j].DueDate)
		if iOk && jOk {
			return iDate.Before(jDate)
		}
		return iOk && !jOk // tasks with due dates before those without
	})

	// Fill up to 7 hours
	var result []api.Task
	var usedMs int64

	// Forced tasks always included
	for _, t := range forced {
		result = append(result, t)
		usedMs += remainingTimeMs(t)
	}

	for _, t := range candidates {
		remaining := remainingTimeMs(t)

		if isDueOrOverdue(t) {
			// Always include due/overdue tasks
			result = append(result, t)
			usedMs += remaining
			continue
		}

		if remaining == 0 {
			// No estimate — include but don't count toward capacity
			result = append(result, t)
			continue
		}

		if usedMs+remaining <= dailyCapacityMs {
			result = append(result, t)
			usedMs += remaining
		}
	}

	return result
}

// buildTodayItems wraps tasks with their action state.
func buildTodayItems(tasks []api.Task, actions map[string]string) []TodayItem {
	var items []TodayItem
	for _, t := range tasks {
		items = append(items, TodayItem{
			Task:   t,
			Action: actions[t.ID],
		})
	}
	return items
}

// Update handles messages for the Today view.
func (t Today) Update(msg tea.Msg) (Today, tea.Cmd) {
	// Handle force picker overlay
	if t.forcePicker != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			p, cmd := t.forcePicker.Update(msg)
			picker := p.(ui.Picker)
			t.forcePicker = &picker
			return t, cmd
		case ui.PickerResult:
			t.forcePicker = nil
			if !msg.Cancelled && len(msg.Selected) > 0 {
				taskID := msg.Selected[0].ID
				t.todayActions[taskID] = "forced"
				if t.cache != nil {
					today := time.Now().Format("2006-01-02")
					t.cache.SetTodayAction(taskID, "forced", today)
				}
				// Recalculate with new forced task
				t.items = buildTodayItems(calculateTodayList(t.myOpenTasks(), t.todayActions), t.todayActions)
			}
			return t, nil
		}
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return t.updateKeys(msg)
	}
	return t, nil
}

func (t Today) updateKeys(msg tea.KeyMsg) (Today, tea.Cmd) {
	active := t.activeItems()

	switch msg.String() {
	case "j", "down":
		if t.cursor < len(active)-1 {
			t.cursor++
		}
	case "k", "up":
		if t.cursor > 0 {
			t.cursor--
		}
	case "c":
		// Calculate today's list
		t.items = buildTodayItems(calculateTodayList(t.myOpenTasks(), t.todayActions), t.todayActions)
		t.calculated = true
		t.cursor = 0
	case "f":
		// Force-add picker
		t.forcePicker = t.buildForcePicker()
		if t.forcePicker != nil {
			return t, t.forcePicker.Init()
		}
	case "i":
		// Ignore for today
		if task := t.selectedActiveTask(); task != nil {
			t.todayActions[task.ID] = "ignored"
			if t.cache != nil {
				today := time.Now().Format("2006-01-02")
				t.cache.SetTodayAction(task.ID, "ignored", today)
			}
			t.items = buildTodayItems(calculateTodayList(t.myOpenTasks(), t.todayActions), t.todayActions)
			if t.cursor >= len(t.activeItems()) {
				t.cursor = max(0, len(t.activeItems())-1)
			}
		}
	case "d":
		// Done for today
		if task := t.selectedActiveTask(); task != nil {
			t.todayActions[task.ID] = "done_for_day"
			if t.cache != nil {
				today := time.Now().Format("2006-01-02")
				t.cache.SetTodayAction(task.ID, "done_for_day", today)
			}
			// Update item action in place
			for i := range t.items {
				if t.items[i].Task.ID == task.ID {
					t.items[i].Action = "done_for_day"
				}
			}
		}
	case "enter":
		if task := t.selectedActiveTask(); task != nil {
			t.wantsDetail = task
		}
	}
	return t, nil
}

func (t *Today) selectedActiveTask() *api.Task {
	active := t.activeItems()
	if len(active) == 0 || t.cursor >= len(active) {
		return nil
	}
	task := active[t.cursor].Task
	return &task
}

func (t *Today) buildForcePicker() *ui.Picker {
	// Show tasks assigned to me that aren't already on today's list
	onList := make(map[string]bool)
	for _, item := range t.items {
		onList[item.Task.ID] = true
	}

	var pickerItems []ui.PickerItem
	myTasks := t.myOpenTasks()

	// Sort by priority then due date
	sort.SliceStable(myTasks, func(i, j int) bool {
		iPri := priorityRank(myTasks[i])
		jPri := priorityRank(myTasks[j])
		if iPri != jPri {
			return iPri < jPri
		}
		iDate, iOk := parseDueDate(myTasks[i].DueDate)
		jDate, jOk := parseDueDate(myTasks[j].DueDate)
		if iOk && jOk {
			return iDate.Before(jDate)
		}
		return iOk && !jOk
	})

	for _, task := range myTasks {
		if onList[task.ID] && t.todayActions[task.ID] != "ignored" {
			continue
		}
		label := task.Name
		if task.TimeEstimate > 0 {
			label += " (" + ui.FormatDuration(task.TimeEstimate) + ")"
		}
		pickerItems = append(pickerItems, ui.PickerItem{ID: task.ID, Label: label})
	}

	if len(pickerItems) == 0 {
		return nil
	}

	p := ui.NewPicker("Force Add Task", pickerItems, false)
	return &p
}

// View renders the Today view.
func (t Today) View() string {
	if !t.calculated && len(t.items) == 0 {
		return t.renderEmpty()
	}
	return t.renderList()
}

func (t Today) renderEmpty() string {
	bw := t.boardWidth()
	pw := t.previewWidth()

	content := lipgloss.NewStyle().
		Width(bw).
		Height(t.height - 2).
		Padding(2, 4).
		Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFgDim)).Render(
				"No tasks calculated yet.\n\nPress [c] to calculate today's task list."),
		)

	footer := ui.RenderFooter(t.keyBindings(), t.width)

	if pw > 0 {
		empty := lipgloss.NewStyle().Width(pw).Render("")
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, empty)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (t Today) renderList() string {
	bw := t.boardWidth()
	pw := t.previewWidth()
	bodyH := t.height - 2 // minus header/footer

	// Header
	now := time.Now()
	dateStr := now.Format("Mon Jan 2")
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ui.ColorBlue)).
		Bold(true).
		Render(fmt.Sprintf("📋 Today — %s", dateStr))

	// Capacity
	capStr := t.capacityString()
	headerLine := header + strings.Repeat(" ", max(0, bw-lipgloss.Width(header)-lipgloss.Width(capStr)-2)) + capStr

	// Task rows
	active := t.activeItems()
	var rows []string
	maxRows := bodyH - 3 // header + spacing
	for i, item := range active {
		if i >= maxRows {
			break
		}
		rows = append(rows, t.renderRow(item, i == t.cursor, bw-4))
	}

	listContent := lipgloss.JoinVertical(lipgloss.Left,
		headerLine,
		"",
		strings.Join(rows, "\n"),
	)

	listPanel := lipgloss.NewStyle().
		Width(bw).
		Height(bodyH).
		Padding(0, 1).
		Render(listContent)

	// Preview pane
	var body string
	if pw > 0 {
		task := t.SelectedTask()
		var preview string
		if task != nil {
			listName := ""
			for _, l := range t.state.Lists {
				if l.ID == task.ListID {
					listName = l.Name
					break
				}
			}
			preview = ui.RenderPreview(*task, pw, bodyH, listName)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, listPanel, preview)
	} else {
		body = listPanel
	}

	footer := ui.RenderFooter(t.keyBindings(), t.width)

	result := lipgloss.JoinVertical(lipgloss.Left, body, footer)

	// Overlay
	if t.forcePicker != nil {
		result = t.renderOverlay(result)
	}

	return result
}

func (t Today) renderRow(item TodayItem, selected bool, width int) string {
	task := item.Task

	// Priority color bar
	priColor := ui.ColorFgDim
	if task.Priority != nil {
		switch strings.ToLower(task.Priority.Priority) {
		case "urgent":
			priColor = ui.ColorRed
		case "high":
			priColor = ui.ColorYellow
		case "normal":
			priColor = ui.ColorGreen
		}
	}

	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(priColor)).Render("▌")

	// Cursor
	cursor := "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue)).Render("> ")
	}

	// Task name
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFg))
	if item.Action == "done_for_day" || isClosedStatus(task.Status) {
		nameStyle = nameStyle.Strikethrough(true).Foreground(lipgloss.Color(ui.ColorFgDim))
	}
	if selected {
		nameStyle = nameStyle.Bold(true)
	}

	// Forced indicator
	forceInd := ""
	if item.Action == "forced" {
		forceInd = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPurple)).Render("[+] ")
	}

	name := nameStyle.Render(task.Name)

	// Right-side info
	var info []string
	// Status
	info = append(info, lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFgDim)).Render(task.Status.Status))
	if task.TimeEstimate > 0 {
		remaining := remainingTimeMs(task)
		info = append(info, lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFgDim)).Render(ui.FormatDuration(remaining)))
	}
	if due, ok := parseDueDate(task.DueDate); ok {
		rel := formatRelativeDate(due)
		dueColor := ui.ColorFgDim
		if isDueOrOverdue(task) {
			dueColor = ui.ColorRed
		}
		info = append(info, lipgloss.NewStyle().Foreground(lipgloss.Color(dueColor)).Render("due "+rel))
	}

	infoStr := strings.Join(info, "  ")
	nameWidth := width - lipgloss.Width(cursor) - lipgloss.Width(bar) - lipgloss.Width(forceInd) - lipgloss.Width(infoStr) - 2
	if nameWidth < 10 {
		nameWidth = 10
	}
	truncName := lipgloss.NewStyle().Width(nameWidth).MaxWidth(nameWidth).Render(name)

	return cursor + bar + " " + forceInd + truncName + " " + infoStr
}

func (t Today) capacityString() string {
	var filledMs int64
	for _, item := range t.activeItems() {
		if item.Action == "done_for_day" || isClosedStatus(item.Task.Status) {
			continue
		}
		filledMs += remainingTimeMs(item.Task)
	}

	filledH := float64(filledMs) / 3600000.0
	totalH := float64(dailyCapacityMs) / 3600000.0

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFgDim))
	if filledMs > dailyCapacityMs {
		excess := filledH - totalH
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorRed)).
			Render(fmt.Sprintf("⚠ %.1f/%.1fh — over by %.1fh", filledH, totalH, excess))
	}
	return style.Render(fmt.Sprintf("%.1f/%.1fh", filledH, totalH))
}

func (t Today) renderOverlay(base string) string {
	if t.forcePicker == nil {
		return base
	}

	overlay := t.forcePicker.View()
	overlayBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorBorderAct)).
		Background(lipgloss.Color(ui.ColorCardBg)).
		Padding(1, 2).
		Render(overlay)

	// Center overlay
	boxW := lipgloss.Width(overlayBox)
	padLeft := max(0, (t.width-boxW)/2)
	padTop := max(0, (t.height-lipgloss.Height(overlayBox))/2)

	lines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlayBox, "\n")
	indent := strings.Repeat(" ", padLeft)

	for i, ol := range overlayLines {
		row := padTop + i
		if row < len(lines) {
			lines[row] = indent + ol
		}
	}
	return strings.Join(lines, "\n")
}

func (t Today) boardWidth() int {
	pw := t.previewWidth()
	return t.width - pw
}

func (t Today) previewWidth() int {
	if t.width >= 100 {
		return t.width / 3
	}
	return 0
}

func (t Today) keyBindings() []ui.KeyBinding {
	return []ui.KeyBinding{
		{Key: "c", Label: "calculate"},
		{Key: "f", Label: "force add"},
		{Key: "i", Label: "ignore"},
		{Key: "d", Label: "done today"},
		{Key: "enter", Label: "open"},
		{Key: "j/k", Label: "navigate"},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -run TestCalculate -v`
Expected: All PASS.

- [ ] **Step 5: Build everything together with Task 7 changes**

Run: `go build ./...`
Expected: Compiles with the app.go changes from Task 7.

- [ ] **Step 6: Run full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 7: Commit Tasks 7 and 8 together**

```bash
git add internal/model/app.go internal/model/today.go internal/model/today_test.go internal/ui/footer.go
git commit -m "feat: add Today view as default with auto-calculate algorithm"
```

---

### Task 9: Integration testing and polish

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS.

- [ ] **Step 2: Build and verify binary runs**

Run: `go build -o clickban ./cmd/clickban && echo "Build OK"`
Expected: "Build OK"

- [ ] **Step 3: Verify view switching works**

Manual check: run the binary, verify:
- App opens to Today view (empty, shows "press [c] to calculate")
- Press `1` → Kanban view
- Press `2` → My Tasks view
- Press `0` or `t` → back to Today view
- Press `!` on My Tasks → toggles validation filter

- [ ] **Step 4: Verify kanban cards show new fields**

Manual check: navigate to Kanban view, verify cards show time estimate and due date when present.

- [ ] **Step 5: Verify detail view shows new fields**

Manual check: open a task detail, verify time estimate and due date are shown. Press `T` to edit estimate, `D` to edit due date.

- [ ] **Step 6: Verify validation warning in footer**

Manual check: if any assigned tasks are missing priority or time estimate, the footer should show "⚠ N tasks need data".

- [ ] **Step 7: Final commit if any polish needed**

```bash
git add -A
git commit -m "polish: integration fixes for daily todo checklist"
```

---

## Summary of Changes

| Task | What | Files |
|------|------|-------|
| 1 | Extract shared helpers | helpers.go, kanban.go, mytasks.go |
| 2 | SQLite today_state persistence | cache.go, cache_test.go |
| 3 | Time estimate + due date on kanban cards | card.go, card_test.go |
| 4 | Time estimate + due date in detail view | detail.go |
| 5 | Validation warning renderer | footer.go |
| 6 | Validation filter on My Tasks | mytasks.go |
| 7 | View numbering + app routing | app.go |
| 8 | Today view model + algorithm | today.go, today_test.go |
| 9 | Integration testing + polish | all |
