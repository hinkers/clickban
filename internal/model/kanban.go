package model

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nhinkley/clickban/internal/api"
	"github.com/nhinkley/clickban/internal/ui"
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
	colIndex    int // active column
	rowIndex    int // active card within column
	colOffset   int // horizontal scroll offset
	moveMode    bool
	wantsDetail *api.Task
	width       int
	height      int
}

// NewKanban creates a Kanban from the given app state.
func NewKanban(state AppState) Kanban {
	k := Kanban{state: state}
	k.columns = buildColumns(state)
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

// ClearWantsDetail clears the pending detail request.
func (k *Kanban) ClearWantsDetail() {
	k.wantsDetail = nil
}

// Update implements tea.Model.
func (k Kanban) Update(msg tea.Msg) (Kanban, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k.moveMode {
			return k.updateMoveMode(msg)
		}
		return k.updateNormal(msg)
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
		// enter move-card mode
		if k.SelectedTask() != nil {
			k.moveMode = true
		}
	case "enter":
		if t := k.SelectedTask(); t != nil {
			k.wantsDetail = t
		}
	}
	return k, nil
}

func (k Kanban) updateMoveMode(msg tea.KeyMsg) (Kanban, tea.Cmd) {
	switch msg.String() {
	case "esc":
		k.moveMode = false
	case "h", "left":
		if k.colIndex > 0 {
			return k.moveCard(k.colIndex - 1)
		}
	case "l", "right":
		if k.colIndex < len(k.columns)-1 {
			return k.moveCard(k.colIndex + 1)
		}
	default:
		// 1-9 to jump to column by number
		if len(msg.String()) == 1 && msg.String() >= "1" && msg.String() <= "9" {
			target := int(msg.String()[0]-'1')
			if target < len(k.columns) {
				return k.moveCard(target)
			}
		}
	}
	return k, nil
}

func (k Kanban) moveCard(targetCol int) (Kanban, tea.Cmd) {
	task := k.SelectedTask()
	if task == nil {
		k.moveMode = false
		return k, nil
	}

	newStatus := k.columns[targetCol].Status

	// Optimistic local update: remove from current column, add to target
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
	updatedTask.Status = newStatus
	k.columns[targetCol].Tasks = append(k.columns[targetCol].Tasks, updatedTask)
	k.moveMode = false

	// API call
	statusStr := newStatus.Status
	updateReq := &api.UpdateTaskRequest{
		Status: &statusStr,
	}
	client := k.state.Client
	taskID := task.ID

	return k, func() tea.Msg {
		if err := client.UpdateTask(taskID, updateReq); err != nil {
			return StatusMsg{Text: fmt.Sprintf("move failed: %v", err)}
		}
		return StatusMsg{Text: fmt.Sprintf("Moved to %s", newStatus.Status)}
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

	return lipgloss.JoinVertical(lipgloss.Left, board, footer)
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

	// Mark move target columns when in move mode
	borderStyle := ui.BorderStyle
	if active {
		borderStyle = ui.ActiveBorderStyle
	}
	if k.moveMode && !active {
		borderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorYellow)
	}

	return borderStyle.
		Width(width).
		Height(height).
		Render(content)
}

func (k Kanban) keyBindings() []ui.KeyBinding {
	if k.moveMode {
		return []ui.KeyBinding{
			{Key: "h/l", Label: "move left/right"},
			{Key: "1-9", Label: "move to column"},
			{Key: "esc", Label: "cancel"},
		}
	}
	return []ui.KeyBinding{
		{Key: "h/l", Label: "switch column"},
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "detail"},
		{Key: "m", Label: "move card"},
		{Key: "1/2", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
}

// buildColumns creates sorted columns from the tasks in the state.
func buildColumns(state AppState) []KanbanColumn {
	// Build a map of status.ID -> column
	colMap := make(map[string]*KanbanColumn)
	statusOrder := make(map[string]int)

	for i, s := range state.Statuses {
		statusOrder[s.ID] = i
		if _, ok := colMap[s.ID]; !ok {
			sc := s
			colMap[s.ID] = &KanbanColumn{Status: sc}
		}
	}

	for _, task := range state.Tasks {
		id := task.Status.ID
		if _, ok := colMap[id]; !ok {
			// status not in space statuses — add it
			colMap[id] = &KanbanColumn{Status: task.Status}
		}
		col := colMap[id]
		col.Tasks = append(col.Tasks, task)
	}

	// Sort columns by status order index
	var cols []KanbanColumn
	for _, col := range colMap {
		cols = append(cols, *col)
	}
	sort.Slice(cols, func(i, j int) bool {
		oi, oiOk := statusOrder[cols[i].Status.ID]
		oj, ojOk := statusOrder[cols[j].Status.ID]
		if oiOk && ojOk {
			return oi < oj
		}
		if oiOk {
			return true
		}
		if ojOk {
			return false
		}
		return cols[i].Status.OrderIndex < cols[j].Status.OrderIndex
	})

	return cols
}
