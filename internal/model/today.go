package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/cache"
	"github.com/hinkers/clickban/internal/ui"
)

const todayCapacityMs = 7 * 3600 * 1000 // 7 hours in milliseconds

// TodayItem wraps a task with its action state.
type TodayItem struct {
	Task   api.Task
	Action string // "forced", "ignored", "done_for_day", or ""
}

// Today is the today view model.
type Today struct {
	state          AppState
	cache          *cache.Cache
	items          []TodayItem
	todayActions   map[string]string // taskID -> action
	cursor         int
	wantsDetail    *api.Task
	calculated     bool
	planningMode   bool             // true when in planning mode
	plannedToday   bool             // true after planning has been done today
	planningPicker *ui.Picker       // multi-select picker for planning
	lastSessionIDs map[string]bool  // task IDs from last session (for pre-selection)
	width          int
	height         int
}

// NewToday creates a Today model, loading actions from SQLite.
func NewToday(state AppState, c *cache.Cache) Today {
	actions := make(map[string]string)
	plannedToday := false
	var lastSessionIDs map[string]bool

	if c != nil {
		today := time.Now().Format("2006-01-02")
		if a, err := c.GetTodayActions(today); err == nil {
			actions = a
		}

		if planned, err := c.IsTodayPlanned(today); err == nil {
			plannedToday = planned
		}

		// Get last session's actions before clearing (for outstanding tasks)
		if lastActions, _, err := c.GetLastSessionActions(today); err == nil && len(lastActions) > 0 {
			lastSessionIDs = make(map[string]bool)
			for taskID, action := range lastActions {
				if taskID == "_planned" {
					continue
				}
				if action == "forced" || action == "" {
					lastSessionIDs[taskID] = true
				}
			}
		}

		_ = c.ClearExpiredTodayState(today)
	}

	t := Today{
		state:          state,
		cache:          c,
		todayActions:   actions,
		plannedToday:   plannedToday,
		lastSessionIDs: lastSessionIDs,
	}

	if plannedToday || len(actions) > 0 {
		t.recalculate()
	}

	return t
}

// NewTodayWithState creates a Today model preserving existing actions and planning state.
func NewTodayWithState(state AppState, c *cache.Cache, actions map[string]string, lastSessionIDs map[string]bool, plannedToday bool) Today {
	t := Today{
		state:          state,
		cache:          c,
		todayActions:   actions,
		plannedToday:   plannedToday,
		lastSessionIDs: lastSessionIDs,
	}
	if plannedToday || len(actions) > 0 {
		t.recalculate()
	}
	return t
}

func (t *Today) openPlanningMode() {
	myTasks := t.assignedTasks()
	if len(myTasks) == 0 {
		return
	}

	var outstanding []api.Task
	var dueSoon []api.Task
	var other []api.Task
	now := time.Now()
	soonThreshold := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Add(3 * 24 * time.Hour)

	for _, task := range myTasks {
		if t.lastSessionIDs[task.ID] {
			outstanding = append(outstanding, task)
		} else if due, ok := parseDueDate(task.DueDate); ok && !due.After(soonThreshold) {
			dueSoon = append(dueSoon, task)
		} else {
			other = append(other, task)
		}
	}

	sortByPriorityDue := func(tasks []api.Task) {
		sort.SliceStable(tasks, func(i, j int) bool {
			iDue := isDueOrOverdue(tasks[i])
			jDue := isDueOrOverdue(tasks[j])
			if iDue != jDue {
				return iDue
			}
			iPri := priorityRank(tasks[i].Priority)
			jPri := priorityRank(tasks[j].Priority)
			if iPri != jPri {
				return iPri < jPri
			}
			iDate, iOk := parseDueDate(tasks[i].DueDate)
			jDate, jOk := parseDueDate(tasks[j].DueDate)
			if iOk && jOk {
				return iDate.Before(jDate)
			}
			return iOk && !jOk
		})
	}
	sortByPriorityDue(outstanding)
	sortByPriorityDue(dueSoon)
	sortByPriorityDue(other)

	var items []ui.PickerItem

	if len(outstanding) > 0 {
		items = append(items, ui.PickerItem{Label: "Outstanding from last session", Header: true})
		for _, task := range outstanding {
			items = append(items, t.planningPickerItem(task))
		}
	}

	if len(dueSoon) > 0 {
		items = append(items, ui.PickerItem{Label: "Due soon", Header: true})
		for _, task := range dueSoon {
			items = append(items, t.planningPickerItem(task))
		}
	}

	if len(other) > 0 {
		items = append(items, ui.PickerItem{Label: "Other assigned tasks", Header: true})
		for _, task := range other {
			items = append(items, t.planningPickerItem(task))
		}
	}

	if len(items) == 0 {
		return
	}

	p := ui.NewPicker("Plan Your Day", items, true)

	// Pre-select outstanding tasks
	for i, item := range items {
		if item.Header {
			continue
		}
		if t.lastSessionIDs[item.ID] {
			p.SetSelected(i, true)
		}
	}

	t.planningPicker = &p
	t.planningMode = true
}

func (t *Today) planningPickerItem(task api.Task) ui.PickerItem {
	label := task.Name

	if listName := t.listName(task.List.ID); listName != "" {
		label += " [" + listName + "]"
	}

	if task.TimeEstimate > 0 {
		rem := remainingTimeMs(task)
		if rem > 0 {
			label += " (" + ui.FormatDuration(rem) + " left)"
		}
	}

	if due, ok := parseDueDate(task.DueDate); ok {
		label += " — " + formatRelativeDate(due)
	}

	var color lipgloss.Color
	switch priorityRank(task.Priority) {
	case 1:
		color = ui.ColorRed
	case 2:
		color = ui.ColorYellow
	case 3:
		color = ui.ColorBlue
	case 4:
		color = ui.ColorFgDim
	}

	return ui.PickerItem{ID: task.ID, Label: label, Color: color}
}

// Resize sets the terminal dimensions.
func (t Today) Resize(w, h int) Today {
	t.width = w
	t.height = h
	return t
}

// SelectedTask returns the currently selected task or nil.
func (t *Today) SelectedTask() *api.Task {
	if t.cursor >= len(t.items) {
		return nil
	}
	task := t.items[t.cursor].Task
	return &task
}

// WantsDetail returns a task if the user pressed enter to open detail.
func (t *Today) WantsDetail() *api.Task {
	return t.wantsDetail
}

// ClearWantsDetail clears the pending detail request.
func (t *Today) ClearWantsDetail() {
	t.wantsDetail = nil
}

// HasOverlay returns true if a picker overlay is active.
func (t *Today) HasOverlay() bool {
	return t.planningPicker != nil
}

// TodayActions returns the current actions map.
func (t *Today) TodayActions() map[string]string {
	return t.todayActions
}

func (t *Today) listName(listID string) string {
	for _, l := range t.state.Lists {
		if l.ID == listID {
			return l.Name
		}
	}
	return ""
}

func (t *Today) recalculate() {
	myTasks := t.assignedTasks()
	result := calculateTodayList(myTasks, t.todayActions)
	t.items = make([]TodayItem, 0, len(result))
	for _, task := range result {
		// Tasks in "done" status go to the bottom, not inline
		if isClosedStatus(task.Status) {
			continue
		}
		t.items = append(t.items, TodayItem{
			Task:   task,
			Action: t.todayActions[task.ID],
		})
	}
	// Append done_for_day tasks (shown struck through, excluded from capacity)
	for _, task := range myTasks {
		if t.todayActions[task.ID] == "done_for_day" {
			t.items = append(t.items, TodayItem{Task: task, Action: "done_for_day"})
		}
	}
	// Append tasks in done/closed status at the bottom with strikethrough
	seen := make(map[string]bool)
	for _, task := range result {
		if isClosedStatus(task.Status) && !seen[task.ID] {
			seen[task.ID] = true
			t.items = append(t.items, TodayItem{Task: task, Action: "completed"})
		}
	}
	for _, task := range t.state.Tasks {
		if !isClosedStatus(task.Status) || seen[task.ID] {
			continue
		}
		if t.todayActions[task.ID] == "" || t.todayActions[task.ID] == "_planned" {
			continue
		}
		assigned := false
		for _, a := range task.Assignees {
			if t.state.CurrentUser != nil && a.ID == t.state.CurrentUser.ID {
				assigned = true
				break
			}
		}
		if assigned {
			t.items = append(t.items, TodayItem{Task: task, Action: "completed"})
		}
	}
	t.calculated = true
}

func (t *Today) assignedTasks() []api.Task {
	var tasks []api.Task
	for _, task := range t.state.Tasks {
		if strings.ToLower(task.Status.Type) == "closed" {
			continue
		}
		for _, a := range task.Assignees {
			if t.state.CurrentUser != nil && a.ID == t.state.CurrentUser.ID {
				tasks = append(tasks, task)
				break
			}
		}
	}
	return tasks
}

// calculateTodayList is the core algorithm for building the today list.
func calculateTodayList(tasks []api.Task, actions map[string]string) []api.Task {
	// Separate forced tasks and filter out ignored/done_for_day
	var forced []api.Task
	var candidates []api.Task

	for _, task := range tasks {
		action := actions[task.ID]
		switch action {
		case "ignored", "done_for_day":
			continue
		case "forced":
			forced = append(forced, task)
		default:
			candidates = append(candidates, task)
		}
	}

	// Sort candidates: due today/overdue first (by priority), then rest by priority (due date tiebreak)
	sort.SliceStable(candidates, func(i, j int) bool {
		iDue := isDueOrOverdue(candidates[i])
		jDue := isDueOrOverdue(candidates[j])

		if iDue != jDue {
			return iDue
		}

		iPri := priorityRank(candidates[i].Priority)
		jPri := priorityRank(candidates[j].Priority)
		if iPri != jPri {
			return iPri < jPri
		}

		// Tiebreak by due date (earlier first)
		iDate, iOk := parseDueDate(candidates[i].DueDate)
		jDate, jOk := parseDueDate(candidates[j].DueDate)
		if iOk && jOk {
			return iDate.Before(jDate)
		}
		if iOk != jOk {
			return iOk
		}
		return false
	})

	// Calculate forced capacity usage
	var usedMs int64
	for _, task := range forced {
		rem := remainingTimeMs(task)
		if rem > 0 {
			usedMs += rem
		}
	}

	// Fill to capacity
	var selected []api.Task
	for _, task := range candidates {
		rem := remainingTimeMs(task)
		dueToday := isDueOrOverdue(task)

		if rem == 0 {
			// No estimate: include but don't count toward capacity
			selected = append(selected, task)
			continue
		}

		if dueToday {
			// Due today: always include even if over capacity
			selected = append(selected, task)
			usedMs += rem
			continue
		}

		if usedMs < int64(todayCapacityMs) {
			selected = append(selected, task)
			usedMs += rem
		}
	}

	// Merge forced + selected and sort together
	all := make([]api.Task, 0, len(forced)+len(selected))
	all = append(all, forced...)
	all = append(all, selected...)
	sort.SliceStable(all, func(i, j int) bool {
		iDue := isDueOrOverdue(all[i])
		jDue := isDueOrOverdue(all[j])

		if iDue != jDue {
			return iDue
		}

		iPri := priorityRank(all[i].Priority)
		jPri := priorityRank(all[j].Priority)
		if iPri != jPri {
			return iPri < jPri
		}

		iDate, iOk := parseDueDate(all[i].DueDate)
		jDate, jOk := parseDueDate(all[j].DueDate)
		if iOk && jOk {
			return iDate.Before(jDate)
		}
		if iOk != jOk {
			return iOk
		}
		return false
	})

	return all
}

// Update handles messages for the Today view.
func (t Today) Update(msg tea.Msg) (Today, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PickerResult:
		if t.planningMode {
			return t.handlePlanningResult(msg)
		}
		return t, nil

	case tea.KeyMsg:
		if t.planningPicker != nil {
			if msg.String() == "a" {
				return t.handleAutoFill()
			}
			p := *t.planningPicker
			m, cmd := p.Update(msg)
			np := m.(ui.Picker)
			t.planningPicker = &np
			return t, cmd
		}

		return t.updateNormal(msg)
	}

	if t.planningPicker != nil {
		p := *t.planningPicker
		m, cmd := p.Update(msg)
		np := m.(ui.Picker)
		t.planningPicker = &np
		return t, cmd
	}
	return t, nil
}

func (t Today) updateNormal(msg tea.KeyMsg) (Today, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if t.cursor < len(t.items)-1 {
			t.cursor++
		}
	case "k", "up":
		if t.cursor > 0 {
			t.cursor--
		}
	case "c":
		t.recalculate()
		if t.cursor >= len(t.items) {
			t.cursor = max(0, len(t.items)-1)
		}
	case "i":
		if sel := t.SelectedTask(); sel != nil {
			t.setAction(sel.ID, "ignored")
			t.recalculate()
			if t.cursor >= len(t.items) {
				t.cursor = max(0, len(t.items)-1)
			}
		}
	case "d":
		if sel := t.SelectedTask(); sel != nil {
			t.setAction(sel.ID, "done_for_day")
			t.recalculate()
			if t.cursor >= len(t.items) {
				t.cursor = max(0, len(t.items)-1)
			}
		}
	case "u":
		// Undo action on selected task
		if sel := t.SelectedTask(); sel != nil {
			t.removeAction(sel.ID)
			t.recalculate()
			if t.cursor >= len(t.items) {
				t.cursor = max(0, len(t.items)-1)
			}
		}
	case "p":
		t.openPlanningMode()
	case "enter":
		if sel := t.SelectedTask(); sel != nil {
			t.wantsDetail = sel
		}
	}
	return t, nil
}

func (t Today) handlePlanningResult(res ui.PickerResult) (Today, tea.Cmd) {
	t.planningPicker = nil
	t.planningMode = false

	if res.Cancelled {
		return t, nil
	}

	for _, item := range res.Selected {
		if item.Header {
			continue
		}
		t.setAction(item.ID, "forced")
	}

	t.markPlanned()
	t.plannedToday = true
	t.recalculate()
	return t, nil
}

func (t Today) handleAutoFill() (Today, tea.Cmd) {
	if t.planningPicker == nil {
		return t, nil
	}

	p := *t.planningPicker
	items := p.Items()
	for i, item := range items {
		if item.Header {
			continue
		}
		if p.IsSelected(i) {
			t.setAction(item.ID, "forced")
		}
	}

	t.planningPicker = nil
	t.planningMode = false
	t.markPlanned()
	t.plannedToday = true
	t.recalculate()
	return t, nil
}

func (t *Today) markPlanned() {
	if t.cache != nil {
		today := time.Now().Format("2006-01-02")
		_ = t.cache.SetTodayAction("_planned", "done", today)
	}
}

func (t *Today) setAction(taskID, action string) {
	t.todayActions[taskID] = action
	if t.cache != nil {
		today := time.Now().Format("2006-01-02")
		_ = t.cache.SetTodayAction(taskID, action, today)
	}
}

func (t *Today) removeAction(taskID string) {
	delete(t.todayActions, taskID)
	if t.cache != nil {
		_ = t.cache.RemoveTodayAction(taskID)
	}
}

// View renders the Today view.
func (t Today) View() string {
	// Show planning prompt if not yet planned and no items
	if !t.plannedToday && len(t.items) == 0 && !t.planningMode {
		empty := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Padding(2, 4).Render(
			"No tasks planned for today.\n\nPress " +
				lipgloss.NewStyle().Foreground(ui.ColorBlue).Bold(true).Render("p") +
				lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(" to enter planning mode."))
		footer := ui.RenderFooter(t.emptyKeyBindings(), t.width)
		emptyH := lipgloss.Height(empty)
		footerH := lipgloss.Height(footer)
		spacerH := t.height - emptyH - footerH - 1
		if spacerH < 0 {
			spacerH = 0
		}
		spacer := strings.Repeat("\n", spacerH)
		return lipgloss.JoinVertical(lipgloss.Left, empty, spacer, footer)
	}

	if len(t.items) == 0 && !t.planningMode {
		empty := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("\n  No tasks for today. Press 'p' to plan or 'c' to auto-fill.\n")
		footer := ui.RenderFooter(t.keyBindings(), t.width)
		emptyH := lipgloss.Height(empty)
		footerH := lipgloss.Height(footer)
		spacerH := t.height - emptyH - footerH - 1
		if spacerH < 0 {
			spacerH = 0
		}
		spacer := strings.Repeat("\n", spacerH)
		return lipgloss.JoinVertical(lipgloss.Left, empty, spacer, footer)
	}

	previewW := t.previewWidth()
	tableW := t.width - previewW
	tableH := t.height - 3

	table := t.renderTable(tableW, tableH)

	var content string
	if previewW > 0 && t.SelectedTask() != nil {
		task := t.SelectedTask()
		preview := ui.RenderPreview(*task, previewW, tableH, t.listName(task.List.ID), t.state.RunningTaskID)
		content = lipgloss.JoinHorizontal(lipgloss.Top, table, preview)
	} else {
		content = table
	}

	footer := ui.RenderFooter(t.keyBindings(), t.width)
	result := lipgloss.JoinVertical(lipgloss.Left, content, footer)

	// Overlay: planning picker
	if t.planningPicker != nil {
		result = t.renderPlanningOverlay(result)
	}

	return result
}

func (t Today) renderTable(width, height int) string {
	var sb strings.Builder

	// Header
	now := time.Now()
	dateStr := now.Format("Mon Jan 2")

	// Calculate total capacity used (exclude done_for_day and completed)
	var totalMs int64
	for _, item := range t.items {
		if item.Action != "done_for_day" && item.Action != "completed" && !isClosedStatus(item.Task.Status) {
			rem := remainingTimeMs(item.Task)
			if rem > 0 {
				totalMs += rem
			}
		}
	}
	filledH := float64(totalMs) / 3600000.0
	headerStyle := lipgloss.NewStyle().Foreground(ui.ColorFgBright).Bold(true)
	titleStr := headerStyle.Render(fmt.Sprintf("📋 Today — %s", dateStr))

	var capStr string
	if totalMs > int64(todayCapacityMs) {
		excess := filledH - 7.0
		capStr = lipgloss.NewStyle().Foreground(ui.ColorRed).Render(
			fmt.Sprintf("⚠ %.1fh / 7.0h — over capacity by %.1fh", filledH, excess))
	} else {
		capStr = lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(
			fmt.Sprintf("%.1fh / 7.0h", filledH))
	}

	// Right-align capacity on the header line
	gap := width - lipgloss.Width(titleStr) - lipgloss.Width(capStr) - 6
	if gap < 1 {
		gap = 1
	}
	sb.WriteString(titleStr + strings.Repeat(" ", gap) + capStr)
	sb.WriteString("\n")

	// Column widths
	priW := 4
	forceW := 4
	statusW := 18
	timeW := 8
	dueW := 12
	nameW := width - priW - forceW - statusW - timeW - dueW - 10
	if nameW < 10 {
		nameW = 10
	}

	divider := lipgloss.NewStyle().Foreground(ui.ColorBorder).Render(strings.Repeat("─", width-4))
	sb.WriteString("  " + divider + "\n")

	// Rows
	maxRows := height - 5
	for i, item := range t.items {
		if i >= maxRows {
			break
		}

		selected := i == t.cursor
		task := item.Task
		doneForDay := item.Action == "done_for_day" || item.Action == "completed" || isClosedStatus(task.Status)

		// Priority color bar
		priRank := priorityRank(task.Priority)
		var priColor lipgloss.Color
		switch priRank {
		case 1:
			priColor = ui.ColorRed    // urgent=red
		case 2:
			priColor = ui.ColorYellow // high=orange (ColorYellow is #e0af68)
		case 3:
			priColor = ui.ColorBlue   // normal=blue
		case 4:
			priColor = ui.ColorFgDim  // low=grey
		default:
			priColor = ui.ColorFgDim
		}
		priBar := lipgloss.NewStyle().Foreground(priColor).Width(priW).Render("▎")

		// Cursor
		prefix := "  "
		if selected {
			prefix = "> "
		}

		// Force indicator
		forceInd := "    "
		if item.Action == "forced" {
			forceInd = lipgloss.NewStyle().Foreground(ui.ColorPurple).Render("[+] ")
		}

		// Task name
		name := task.Name
		nameStyle := lipgloss.NewStyle().Foreground(ui.ColorFg).Width(nameW).MaxWidth(nameW)
		if selected {
			nameStyle = nameStyle.Foreground(ui.ColorBlue).Bold(true)
		}
		if doneForDay {
			nameStyle = nameStyle.Strikethrough(true).Foreground(ui.ColorFgDim)
		}
		nameCell := nameStyle.Render(name)

		// Status
		statusColor := lipgloss.Color(task.Status.Color)
		if task.Status.Color == "" {
			statusColor = ui.ColorFgDim
		}
		statusStyle := lipgloss.NewStyle().Foreground(statusColor).Width(statusW).MaxWidth(statusW)
		if doneForDay {
			statusStyle = statusStyle.Strikethrough(true)
		}
		statusText := task.Status.Status
		if len(statusText) > statusW-1 {
			statusText = statusText[:statusW-2] + "…"
		}
		statusCell := statusStyle.Render(statusText)

		// Remaining time
		rem := remainingTimeMs(task)
		timeStr := ""
		if rem > 0 {
			timeStr = formatDurationShort(rem)
		} else if task.TimeEstimate > 0 && task.TimeSpent > task.TimeEstimate {
			timeStr = "-" + formatDurationShort(task.TimeSpent-task.TimeEstimate)
		}
		timeStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Width(timeW).MaxWidth(timeW)
		if task.TimeEstimate > 0 && task.TimeSpent > task.TimeEstimate {
			timeStyle = timeStyle.Foreground(ui.ColorRed)
		}
		if doneForDay {
			timeStyle = timeStyle.Strikethrough(true)
		}
		timeCell := timeStyle.Render(timeStr)

		// Due date
		dueStr := ""
		if due, ok := parseDueDate(task.DueDate); ok {
			dueStr = formatRelativeDate(due)
		}
		dueStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Width(dueW).MaxWidth(dueW)
		if isDueOrOverdue(task) {
			dueStyle = dueStyle.Foreground(ui.ColorRed)
		}
		if doneForDay {
			dueStyle = dueStyle.Strikethrough(true)
		}
		dueCell := dueStyle.Render(dueStr)

		sb.WriteString(fmt.Sprintf("%s%s%s%s  %s  %s  %s\n", prefix, priBar, forceInd, nameCell, statusCell, timeCell, dueCell))
	}

	return ui.BorderStyle.
		Width(width - 2).
		Height(height).
		Render(sb.String())
}

func (t Today) renderPlanningOverlay(background string) string {
	pickerContent := t.planningPicker.View()

	hint := lipgloss.NewStyle().Foreground(ui.ColorFgDim).MarginTop(1).Render(
		"space: toggle  a: auto-fill rest  enter: confirm  esc: cancel")
	fullContent := pickerContent + "\n" + hint

	overlayW := min(t.width-4, 100)
	overlayStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorBlue).
		Background(ui.ColorCardBg).
		Padding(1, 2).
		Width(overlayW)
	overlayBox := overlayStyle.Render(fullContent)

	ovH := lipgloss.Height(overlayBox)
	ovW := lipgloss.Width(overlayBox)
	topPad := max(0, (t.height-ovH)/2)
	leftPad := max(0, (t.width-ovW)/2)

	leftStr := strings.Repeat(" ", leftPad)
	lines := strings.Split(overlayBox, "\n")
	for i, line := range lines {
		lines[i] = leftStr + line
	}
	return strings.Repeat("\n", topPad) + strings.Join(lines, "\n")
}

func (t Today) previewWidth() int {
	if t.width < 100 {
		return 0
	}
	return t.width / 3
}

func (t Today) keyBindings() []ui.KeyBinding {
	return []ui.KeyBinding{
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "detail"},
		{Key: "p", Label: "plan"},
		{Key: "c", Label: "recalculate"},
		{Key: "i", Label: "ignore"},
		{Key: "d", Label: "done for day"},
		{Key: "u", Label: "undo"},
		{Key: "1/2/3", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
}

func (t Today) emptyKeyBindings() []ui.KeyBinding {
	return []ui.KeyBinding{
		{Key: "p", Label: "plan day"},
		{Key: "1/2/3", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
}

// formatDurationShort formats milliseconds as "Xh Ym".
func formatDurationShort(ms int64) string {
	hours := ms / 3600000
	minutes := (ms % 3600000) / 60000
	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "0m"
}
