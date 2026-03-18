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
	state        AppState
	cache        *cache.Cache
	items        []TodayItem
	todayActions map[string]string // taskID -> action
	cursor       int
	wantsDetail  *api.Task
	calculated   bool
	forcePicker  *ui.Picker
	width        int
	height       int
}

// NewToday creates a Today model, loading actions from SQLite.
func NewToday(state AppState, c *cache.Cache) Today {
	actions := make(map[string]string)
	if c != nil {
		today := time.Now().Format("2006-01-02")
		if a, err := c.GetTodayActions(today); err == nil {
			actions = a
		}
		_ = c.ClearExpiredTodayState(today)
	}
	t := Today{
		state:        state,
		cache:        c,
		todayActions: actions,
	}
	t.recalculate()
	return t
}

// NewTodayWithState creates a Today model preserving existing actions.
func NewTodayWithState(state AppState, c *cache.Cache, actions map[string]string) Today {
	t := Today{
		state:        state,
		cache:        c,
		todayActions: actions,
	}
	t.recalculate()
	return t
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
	return t.forcePicker != nil
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
	// Append completed (closed) tasks that were on the list
	for _, task := range t.state.Tasks {
		if !isClosedStatus(task.Status) {
			continue
		}
		if t.todayActions[task.ID] != "" {
			t.items = append(t.items, TodayItem{Task: task, Action: "completed"})
		}
	}
	t.calculated = true
}

func (t *Today) assignedTasks() []api.Task {
	var tasks []api.Task
	for _, task := range t.state.Tasks {
		if isClosedStatus(task.Status) {
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

		if usedMs+rem <= int64(todayCapacityMs) {
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
		return t.handleForceResult(msg)

	case tea.KeyMsg:
		if t.forcePicker != nil {
			p := *t.forcePicker
			m, cmd := p.Update(msg)
			np := m.(ui.Picker)
			t.forcePicker = &np
			return t, cmd
		}
		return t.updateNormal(msg)
	}

	// Delegate non-key messages to picker if active
	if t.forcePicker != nil {
		p := *t.forcePicker
		m, cmd := p.Update(msg)
		np := m.(ui.Picker)
		t.forcePicker = &np
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
	case "f":
		// Open force picker — list all assigned, non-closed tasks not already in the list
		items := t.forcePickerItems()
		if len(items) > 0 {
			p := ui.NewPicker("Force task into today", items, false)
			t.forcePicker = &p
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
	case "enter":
		if sel := t.SelectedTask(); sel != nil {
			t.wantsDetail = sel
		}
	}
	return t, nil
}

func (t Today) handleForceResult(res ui.PickerResult) (Today, tea.Cmd) {
	t.forcePicker = nil
	if res.Cancelled || len(res.Selected) == 0 {
		return t, nil
	}

	taskID := res.Selected[0].ID
	t.setAction(taskID, "forced")
	t.recalculate()
	return t, nil
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

func (t *Today) forcePickerItems() []ui.PickerItem {
	// Collect IDs already in the list (excluding ignored tasks which should be force-able)
	inList := make(map[string]bool)
	for _, item := range t.items {
		if item.Action != "ignored" {
			inList[item.Task.ID] = true
		}
	}

	var candidates []api.Task
	for _, task := range t.state.Tasks {
		if isClosedStatus(task.Status) {
			continue
		}
		if inList[task.ID] {
			continue
		}
		assigned := false
		for _, a := range task.Assignees {
			if t.state.CurrentUser != nil && a.ID == t.state.CurrentUser.ID {
				assigned = true
				break
			}
		}
		if !assigned {
			continue
		}
		candidates = append(candidates, task)
	}

	// Sort by priority then due date
	sort.SliceStable(candidates, func(i, j int) bool {
		iPri := priorityRank(candidates[i].Priority)
		jPri := priorityRank(candidates[j].Priority)
		if iPri != jPri {
			return iPri < jPri
		}
		iDate, iOk := parseDueDate(candidates[i].DueDate)
		jDate, jOk := parseDueDate(candidates[j].DueDate)
		if iOk && jOk {
			return iDate.Before(jDate)
		}
		return iOk && !jOk
	})

	var items []ui.PickerItem
	for _, task := range candidates {
		label := task.Name
		if task.TimeEstimate > 0 {
			label += " (" + ui.FormatDuration(task.TimeEstimate) + ")"
		}
		items = append(items, ui.PickerItem{ID: task.ID, Label: label})
	}
	return items
}

// View renders the Today view.
func (t Today) View() string {
	if len(t.items) == 0 {
		empty := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("\n  No tasks for today. Press 'c' to calculate.\n")
		footer := ui.RenderFooter(t.keyBindings(), t.width)
		return lipgloss.JoinVertical(lipgloss.Left, empty, footer)
	}

	previewW := t.previewWidth()
	tableW := t.width - previewW
	tableH := t.height - 3

	table := t.renderTable(tableW, tableH)

	var content string
	if previewW > 0 && t.SelectedTask() != nil {
		task := t.SelectedTask()
		preview := ui.RenderPreview(*task, previewW, tableH, t.listName(task.List.ID))
		content = lipgloss.JoinHorizontal(lipgloss.Top, table, preview)
	} else {
		content = table
	}

	footer := ui.RenderFooter(t.keyBindings(), t.width)
	result := lipgloss.JoinVertical(lipgloss.Left, content, footer)

	// Overlay picker
	if t.forcePicker != nil {
		overlayContent := t.forcePicker.View()
		overlayStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorBlue).
			Background(ui.ColorCardBg).
			Padding(1, 2).
			Width(50)
		overlayBox := overlayStyle.Render(overlayContent)

		ovH := lipgloss.Height(overlayBox)
		ovW := lipgloss.Width(overlayBox)
		topPad := (tableH - ovH) / 2
		leftPad := (t.width - ovW) / 2
		if topPad < 0 {
			topPad = 0
		}
		if leftPad < 0 {
			leftPad = 0
		}
		leftStr := strings.Repeat(" ", leftPad)
		lines := strings.Split(overlayBox, "\n")
		for i, line := range lines {
			lines[i] = leftStr + line
		}
		result = strings.Repeat("\n", topPad) + strings.Join(lines, "\n")
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
		doneForDay := item.Action == "done_for_day"

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
		statusCell := statusStyle.Render(task.Status.Status)

		// Remaining time
		rem := remainingTimeMs(task)
		timeStr := ""
		if rem > 0 {
			timeStr = formatDurationShort(rem)
		}
		timeStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Width(timeW).MaxWidth(timeW)
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
		{Key: "c", Label: "recalculate"},
		{Key: "f", Label: "force add"},
		{Key: "i", Label: "ignore"},
		{Key: "d", Label: "done for day"},
		{Key: "u", Label: "undo"},
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
