package model

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// KanbanColumn groups tasks under a single status.
type KanbanColumn struct {
	Status api.Status
	Tasks  []api.Task
}

// Kanban is the kanban board model.
type Kanban struct {
	state       AppState
	columns     []KanbanColumn
	allColumns  []KanbanColumn // includes closed columns
	colIndex    int            // active column
	rowIndex    int            // active card within column
	colOffset   int            // horizontal scroll offset
	showClosed  bool
	movePicker  *ui.Picker // status picker for moving cards
	wantsDetail *api.Task
	width       int
	height      int
}

// NewKanban creates a Kanban from the given app state.
func NewKanban(state AppState) Kanban {
	k := Kanban{state: state}
	k.allColumns = buildColumns(state)
	k.columns = filterColumns(k.allColumns, k.showClosed)
	return k
}

// NewKanbanWithClosed creates a Kanban preserving the showClosed state.
func NewKanbanWithClosed(state AppState, showClosed bool) Kanban {
	k := NewKanban(state)
	k.showClosed = showClosed
	k.columns = filterColumns(k.allColumns, k.showClosed)
	return k
}

// Resize sets the terminal dimensions for rendering.
func (k Kanban) Resize(w, h int) Kanban {
	k.width = w
	k.height = h
	return k
}

// SelectedTask returns a pointer to the currently selected task, or nil.
func (k *Kanban) SelectedTask() *api.Task {
	if k.colIndex >= len(k.columns) {
		return nil
	}
	col := k.columns[k.colIndex]
	if k.rowIndex >= len(col.Tasks) {
		return nil
	}
	t := col.Tasks[k.rowIndex]
	return &t
}

// WantsDetail returns a task if the user pressed enter to open detail, or nil.
func (k *Kanban) WantsDetail() *api.Task {
	return k.wantsDetail
}

// statusItemsForTask returns picker items using the task's list statuses.
func (k *Kanban) statusItemsForTask(task api.Task) []ui.PickerItem {
	for _, list := range k.state.Lists {
		if list.ID == task.ListID {
			var items []ui.PickerItem
			for _, s := range list.Statuses {
				items = append(items, ui.PickerItem{ID: s.ID, Label: s.Status})
			}
			return items
		}
	}
	var items []ui.PickerItem
	for _, s := range k.state.Statuses {
		items = append(items, ui.PickerItem{ID: s.ID, Label: s.Status})
	}
	return items
}

// HasOverlay returns true if a picker overlay is active.
func (k *Kanban) HasOverlay() bool {
	return k.movePicker != nil
}

// ClearWantsDetail clears the pending detail request.
func (k *Kanban) ClearWantsDetail() {
	k.wantsDetail = nil
}

// Update implements tea.Model.
func (k Kanban) Update(msg tea.Msg) (Kanban, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PickerResult:
		return k.handleMoveResult(msg)
	case tea.KeyMsg:
		if k.movePicker != nil {
			p := *k.movePicker
			m, cmd := p.Update(msg)
			pp := m.(ui.Picker)
			k.movePicker = &pp
			return k, cmd
		}
		return k.updateNormal(msg)
	}
	if k.movePicker != nil {
		p := *k.movePicker
		m, cmd := p.Update(msg)
		pp := m.(ui.Picker)
		k.movePicker = &pp
		return k, cmd
	}
	return k, nil
}

func (k Kanban) updateNormal(msg tea.KeyMsg) (Kanban, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		if k.colIndex > 0 {
			k.colIndex--
			k.rowIndex = 0
			k.ensureVisible()
		}
	case "l", "right":
		if k.colIndex < len(k.columns)-1 {
			k.colIndex++
			k.rowIndex = 0
			k.ensureVisible()
		}
	case "j", "down":
		if k.colIndex < len(k.columns) {
			col := k.columns[k.colIndex]
			if k.rowIndex < len(col.Tasks)-1 {
				k.rowIndex++
			}
		}
	case "k", "up":
		if k.rowIndex > 0 {
			k.rowIndex--
		}
	case "m":
		if task := k.SelectedTask(); task != nil {
			items := k.statusItemsForTask(*task)
			p := ui.NewPicker("Move to Status", items, false)
			k.movePicker = &p
		}
	case "x":
		k.showClosed = !k.showClosed
		k.columns = filterColumns(k.allColumns, k.showClosed)
		if k.colIndex >= len(k.columns) {
			k.colIndex = len(k.columns) - 1
		}
		k.rowIndex = 0
		k.ensureVisible()
	case "enter":
		if t := k.SelectedTask(); t != nil {
			k.wantsDetail = t
		}
	}
	return k, nil
}

func (k Kanban) handleMoveResult(res ui.PickerResult) (Kanban, tea.Cmd) {
	k.movePicker = nil
	if res.Cancelled || len(res.Selected) == 0 {
		return k, nil
	}

	task := k.SelectedTask()
	if task == nil {
		return k, nil
	}

	newStatusID := res.Selected[0].ID
	newStatusName := res.Selected[0].Label

	// Find the target column
	targetCol := -1
	for i, col := range k.columns {
		if strings.EqualFold(col.Status.Status, newStatusName) {
			targetCol = i
			break
		}
	}

	// Optimistic local update
	srcCol := k.colIndex
	var srcTasks []api.Task
	for _, t := range k.columns[srcCol].Tasks {
		if t.ID != task.ID {
			srcTasks = append(srcTasks, t)
		}
	}
	k.columns[srcCol].Tasks = srcTasks
	if k.rowIndex >= len(srcTasks) && k.rowIndex > 0 {
		k.rowIndex = len(srcTasks) - 1
	}

	updatedTask := *task
	updatedTask.Status = api.Status{ID: newStatusID, Status: newStatusName}
	if targetCol >= 0 {
		k.columns[targetCol].Tasks = append(k.columns[targetCol].Tasks, updatedTask)
	}

	// API call — ClickUp expects lowercase status names
	client := k.state.Client
	taskID := task.ID
	statusLower := strings.ToLower(newStatusName)
	return k, func() tea.Msg {
		if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Status: &statusLower}); err != nil {
			return StatusMsg{Text: fmt.Sprintf("move failed: %v", err)}
		}
		return StatusMsg{Text: fmt.Sprintf("Moved to %s", newStatusName)}
	}
}

func (k *Kanban) ensureVisible() {
	// Keep colOffset so that colIndex is within visible range
	visibleCols := k.visibleColumnCount()
	if k.colIndex < k.colOffset {
		k.colOffset = k.colIndex
	}
	if k.colIndex >= k.colOffset+visibleCols {
		k.colOffset = k.colIndex - visibleCols + 1
	}
}

func (k *Kanban) visibleColumnCount() int {
	colWidth := k.columnWidth()
	if colWidth <= 0 {
		return 1
	}
	// Leave 20 cols for preview pane if width allows
	boardWidth := k.boardWidth()
	return boardWidth / colWidth
}

func (k *Kanban) boardWidth() int {
	previewW := k.previewWidth()
	return k.width - previewW
}

func (k *Kanban) previewWidth() int {
	if k.width < 100 {
		return 0
	}
	return k.width / 3
}

func (k *Kanban) columnWidth() int {
	if k.width < 40 {
		return k.width
	}
	// Try to fit 3-4 columns in the board area, min 24
	boardW := k.boardWidth()
	w := boardW / 3
	if w < 24 {
		w = 24
	}
	if w > 40 {
		w = 40
	}
	return w
}

// View implements tea.Model.
func (k Kanban) View() string {
	if len(k.columns) == 0 {
		return lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("\n  No tasks found.\n")
	}

	colW := k.columnWidth()
	previewW := k.previewWidth()
	boardH := k.height - 3 // subtract header + footer rows

	// Render visible columns
	visCount := k.visibleColumnCount()
	if visCount < 1 {
		visCount = 1
	}

	var colViews []string
	for i := k.colOffset; i < k.colOffset+visCount && i < len(k.columns); i++ {
		col := k.columns[i]
		colViews = append(colViews, k.renderColumn(col, i, colW, boardH))
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)

	// Render preview pane if wide enough
	if previewW > 0 {
		task := k.SelectedTask()
		if task != nil {
			preview := ui.RenderPreview(*task, previewW, boardH)
			board = lipgloss.JoinHorizontal(lipgloss.Top, board, preview)
		}
	}

	// Footer
	bindings := k.keyBindings()
	footer := ui.RenderFooter(bindings, k.width)

	result := lipgloss.JoinVertical(lipgloss.Left, board, footer)

	// Overlay picker for move
	if k.movePicker != nil {
		overlayContent := k.movePicker.View()
		overlayStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorBlue).
			Background(ui.ColorCardBg).
			Padding(1, 2).
			Width(40)
		overlayBox := overlayStyle.Render(overlayContent)

		ovH := lipgloss.Height(overlayBox)
		ovW := lipgloss.Width(overlayBox)
		topPad := (boardH - ovH) / 2
		leftPad := (k.width - ovW) / 2
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

func (k Kanban) renderColumn(col KanbanColumn, colIdx, width, height int) string {
	active := colIdx == k.colIndex

	headerStyle := lipgloss.NewStyle().
		Foreground(ui.ColorFg).
		Bold(active).
		Width(width - 2)
	if active {
		headerStyle = headerStyle.Foreground(ui.ColorBlue)
	}

	colHeader := headerStyle.Render(col.Status.Status + fmt.Sprintf(" (%d)", len(col.Tasks)))

	var cards []string
	cards = append(cards, colHeader)

	maxCards := height - 2
	for i, task := range col.Tasks {
		if i >= maxCards {
			break
		}
		selected := active && i == k.rowIndex
		card := ui.RenderCard(task, width, selected)
		cards = append(cards, card)
	}

	content := strings.Join(cards, "\n")

	borderStyle := ui.BorderStyle
	if active {
		borderStyle = ui.ActiveBorderStyle
	}

	return borderStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (k Kanban) keyBindings() []ui.KeyBinding {
	return []ui.KeyBinding{
		{Key: "h/l", Label: "switch column"},
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "detail"},
		{Key: "m", Label: "move card"},
		{Key: "x", Label: "toggle done"},
		{Key: "1/2", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
}

// isClosedStatus returns true if the status type indicates completion.
func isClosedStatus(s api.Status) bool {
	t := strings.ToLower(s.Type)
	return t == "closed" || t == "done"
}

// filterColumns returns columns filtered by showClosed.
func filterColumns(all []KanbanColumn, showClosed bool) []KanbanColumn {
	if showClosed {
		return all
	}
	var filtered []KanbanColumn
	for _, col := range all {
		if !isClosedStatus(col.Status) {
			filtered = append(filtered, col)
		}
	}
	return filtered
}

// buildColumns creates sorted columns from the tasks in the state,
// merging statuses that share the same name (case-insensitive).
func buildColumns(state AppState) []KanbanColumn {
	// Merge statuses by lowercase name. Use the first occurrence for display.
	type mergedCol struct {
		status api.Status
		tasks  []api.Task
		order  int
	}

	colByName := make(map[string]*mergedCol)
	nameOrder := 0

	// Register all known statuses first (preserves ordering)
	for _, s := range state.Statuses {
		key := strings.ToLower(s.Status)
		if _, ok := colByName[key]; !ok {
			colByName[key] = &mergedCol{status: s, order: nameOrder}
			nameOrder++
		}
	}

	// Map status IDs to their merged column name
	idToName := make(map[string]string)
	for _, s := range state.Statuses {
		idToName[s.ID] = strings.ToLower(s.Status)
	}

	// Place tasks into merged columns
	for _, task := range state.Tasks {
		key, ok := idToName[task.Status.ID]
		if !ok {
			key = strings.ToLower(task.Status.Status)
		}
		mc, ok := colByName[key]
		if !ok {
			mc = &mergedCol{status: task.Status, order: nameOrder}
			colByName[key] = mc
			nameOrder++
		}
		mc.tasks = append(mc.tasks, task)
	}

	// Sort columns by their original order
	var cols []KanbanColumn
	for _, mc := range colByName {
		cols = append(cols, KanbanColumn{Status: mc.status, Tasks: mc.tasks})
	}
	sort.Slice(cols, func(i, j int) bool {
		ki := strings.ToLower(cols[i].Status.Status)
		kj := strings.ToLower(cols[j].Status.Status)
		return colByName[ki].order < colByName[kj].order
	})

	return cols
}
