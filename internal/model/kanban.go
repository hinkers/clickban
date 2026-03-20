package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// SortMode controls how tasks within columns are sorted.
type SortMode int

const (
	SortDefault       SortMode = iota
	SortCreatedDesc            // newest first
	SortCreatedAsc             // oldest first
	SortUpdatedDesc            // recently updated first
	SortUpdatedAsc             // least recently updated first
	SortAlphaAsc               // A-Z
	SortAlphaDesc              // Z-A
	sortModeCount              // sentinel for cycling
)

func (s SortMode) String() string {
	switch s {
	case SortCreatedDesc:
		return "Created ↓"
	case SortCreatedAsc:
		return "Created ↑"
	case SortUpdatedDesc:
		return "Updated ↓"
	case SortUpdatedAsc:
		return "Updated ↑"
	case SortAlphaAsc:
		return "Name A-Z"
	case SortAlphaDesc:
		return "Name Z-A"
	default:
		return "Default"
	}
}

var sortPickerItems = []ui.PickerItem{
	{ID: "0", Label: "Default"},
	{ID: "1", Label: "Created (newest first)"},
	{ID: "2", Label: "Created (oldest first)"},
	{ID: "3", Label: "Updated (newest first)"},
	{ID: "4", Label: "Updated (oldest first)"},
	{ID: "5", Label: "Name A-Z"},
	{ID: "6", Label: "Name Z-A"},
}

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
	rowOffset   int            // vertical scroll offset within active column
	showClosed  bool
	sortMode    SortMode
	sortPicker  *ui.Picker
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

// NewKanbanWithOptions creates a Kanban preserving display options.
func NewKanbanWithOptions(state AppState, showClosed bool, sortMode SortMode) Kanban {
	k := NewKanban(state)
	k.showClosed = showClosed
	k.sortMode = sortMode
	k.columns = filterColumns(k.allColumns, k.showClosed)
	if sortMode != SortDefault {
		k.sortColumns()
	}
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

// FocusTask moves the cursor to the column and row containing the given task ID.
func (k *Kanban) FocusTask(taskID string) {
	for ci, col := range k.columns {
		for ri, t := range col.Tasks {
			if t.ID == taskID {
				k.colIndex = ci
				k.rowIndex = ri
				k.ensureVisible()
				return
			}
		}
	}
}

// WantsDetail returns a task if the user pressed enter to open detail, or nil.
func (k *Kanban) WantsDetail() *api.Task {
	return k.wantsDetail
}

// statusItemsForTask returns picker items using the task's list statuses.
func (k *Kanban) statusItemsForTask(task api.Task) []ui.PickerItem {
	for _, list := range k.state.Lists {
		if list.ID == task.List.ID {
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
	return k.movePicker != nil || k.sortPicker != nil
}

func (k *Kanban) listName(listID string) string {
	for _, l := range k.state.Lists {
		if l.ID == listID {
			return l.Name
		}
	}
	return ""
}

// ClearWantsDetail clears the pending detail request.
func (k *Kanban) ClearWantsDetail() {
	k.wantsDetail = nil
}

// activePicker returns whichever picker is currently open, or nil.
func (k *Kanban) activePicker() **ui.Picker {
	if k.movePicker != nil {
		return &k.movePicker
	}
	if k.sortPicker != nil {
		return &k.sortPicker
	}
	return nil
}

// Update implements tea.Model.
func (k Kanban) Update(msg tea.Msg) (Kanban, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PickerResult:
		if k.sortPicker != nil {
			return k.handleSortResult(msg)
		}
		return k.handleMoveResult(msg)
	case tea.KeyMsg:
		if pp := k.activePicker(); pp != nil {
			p := **pp
			m, cmd := p.Update(msg)
			np := m.(ui.Picker)
			*pp = &np
			return k, cmd
		}
		return k.updateNormal(msg)
	}
	if pp := k.activePicker(); pp != nil {
		p := **pp
		m, cmd := p.Update(msg)
		np := m.(ui.Picker)
		*pp = &np
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
			k.rowOffset = 0
			k.ensureVisible()
		}
	case "l", "right":
		if k.colIndex < len(k.columns)-1 {
			k.colIndex++
			k.rowIndex = 0
			k.rowOffset = 0
			k.ensureVisible()
		}
	case "j", "down":
		if k.colIndex < len(k.columns) {
			col := k.columns[k.colIndex]
			if k.rowIndex < len(col.Tasks)-1 {
				k.rowIndex++
				visCount := k.visibleCardCount(k.rowOffset)
				if visCount == 0 {
					visCount = 1
				}
				if k.rowIndex >= k.rowOffset+visCount {
					k.rowOffset = k.rowIndex - visCount + 1
				}
			}
		}
	case "k", "up":
		if k.rowIndex > 0 {
			k.rowIndex--
			if k.rowIndex < k.rowOffset {
				k.rowOffset = k.rowIndex
			}
		}
	case "o":
		p := ui.NewPicker("Sort By", sortPickerItems, false)
		k.sortPicker = &p
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

func (k Kanban) handleSortResult(res ui.PickerResult) (Kanban, tea.Cmd) {
	k.sortPicker = nil
	if res.Cancelled || len(res.Selected) == 0 {
		return k, nil
	}
	id, _ := strconv.Atoi(res.Selected[0].ID)
	k.sortMode = SortMode(id)
	k.sortColumns()
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

// visibleCardCount returns how many cards fit starting from the given offset
// in the active column.
func (k Kanban) visibleCardCount(offset int) int {
	if k.colIndex >= len(k.columns) {
		return 0
	}
	col := k.columns[k.colIndex]
	colW := k.columnWidth()
	maxLines := k.height - 3 - 2 // boardH minus border
	usedLines := 1               // header
	if offset > 0 {
		usedLines++ // scroll-up indicator
	}
	count := 0
	for i := offset; i < len(col.Tasks); i++ {
		card := ui.RenderCard(col.Tasks[i], colW, false)
		cardLines := lipgloss.Height(card)
		if usedLines+cardLines+1 > maxLines {
			break
		}
		usedLines += cardLines
		count++
	}
	return count
}

func (k *Kanban) sortColumns() {
	for i := range k.columns {
		k.sortColumnTasks(&k.columns[i])
	}
	for i := range k.allColumns {
		k.sortColumnTasks(&k.allColumns[i])
	}
}

func (k *Kanban) sortColumnTasks(col *KanbanColumn) {
	switch k.sortMode {
	case SortCreatedDesc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return col.Tasks[a].DateCreated > col.Tasks[b].DateCreated
		})
	case SortCreatedAsc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return col.Tasks[a].DateCreated < col.Tasks[b].DateCreated
		})
	case SortUpdatedDesc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return col.Tasks[a].DateUpdated > col.Tasks[b].DateUpdated
		})
	case SortUpdatedAsc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return col.Tasks[a].DateUpdated < col.Tasks[b].DateUpdated
		})
	case SortAlphaAsc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return strings.ToLower(col.Tasks[a].Name) < strings.ToLower(col.Tasks[b].Name)
		})
	case SortAlphaDesc:
		sort.Slice(col.Tasks, func(a, b int) bool {
			return strings.ToLower(col.Tasks[a].Name) > strings.ToLower(col.Tasks[b].Name)
		})
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
			preview := ui.RenderPreview(*task, previewW, boardH, k.listName(task.List.ID), k.state.RunningTaskID)
			board = lipgloss.JoinHorizontal(lipgloss.Top, board, preview)
		}
	}

	// Footer
	bindings := k.keyBindings()
	footer := ui.RenderFooter(bindings, k.width)

	result := lipgloss.JoinVertical(lipgloss.Left, board, footer)

	// Overlay picker
	var activePicker *ui.Picker
	if k.movePicker != nil {
		activePicker = k.movePicker
	} else if k.sortPicker != nil {
		activePicker = k.sortPicker
	}
	if activePicker != nil {
		overlayContent := activePicker.View()
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
	usedLines := 1 // header
	maxLines := height - 2 // leave room for border

	offset := 0
	if active {
		offset = k.rowOffset
	}
	if offset > 0 {
		scrollUp := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(fmt.Sprintf("  ↑ %d more", offset))
		cards = append(cards, scrollUp)
		usedLines++
	}
	lastRendered := offset
	for i := offset; i < len(col.Tasks); i++ {
		selected := active && i == k.rowIndex
		card := ui.RenderCard(col.Tasks[i], width, selected)
		cardLines := lipgloss.Height(card)
		if usedLines+cardLines+1 > maxLines { // +1 for potential scroll indicator
			break
		}
		cards = append(cards, card)
		usedLines += cardLines
		lastRendered = i + 1
	}
	remaining := len(col.Tasks) - lastRendered
	if remaining > 0 {
		scrollDown := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(fmt.Sprintf("  ↓ %d more", remaining))
		cards = append(cards, scrollDown)
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
		{Key: "o", Label: "sort:" + k.sortMode.String()},
		{Key: "x", Label: "toggle done"},
		{Key: "1/2/3", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
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
