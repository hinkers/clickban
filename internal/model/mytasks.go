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

// MyTasks is the my-tasks table view model.
type MyTasks struct {
	state       AppState
	tasks       []api.Task
	cursor      int
	wantsDetail *api.Task
	width       int
	height      int
}

func (m *MyTasks) listName(listID string) string {
	for _, l := range m.state.Lists {
		if l.ID == listID {
			return l.Name
		}
	}
	return ""
}

// NewMyTasks creates a MyTasks model from the app state.
func NewMyTasks(state AppState) MyTasks {
	m := MyTasks{state: state}
	m.tasks = filterAndSortMyTasks(state)
	return m
}

// Resize sets terminal dimensions.
func (m MyTasks) Resize(w, h int) MyTasks {
	m.width = w
	m.height = h
	return m
}

// SelectedTask returns the currently selected task or nil.
func (m *MyTasks) SelectedTask() *api.Task {
	if m.cursor >= len(m.tasks) {
		return nil
	}
	t := m.tasks[m.cursor]
	return &t
}

// WantsDetail returns a task if the user pressed enter to open detail.
func (m *MyTasks) WantsDetail() *api.Task {
	return m.wantsDetail
}

// ClearWantsDetail clears the pending detail request.
func (m *MyTasks) ClearWantsDetail() {
	m.wantsDetail = nil
}

// Update implements tea.Model.
func (m MyTasks) Update(msg tea.Msg) (MyTasks, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if t := m.SelectedTask(); t != nil {
				m.wantsDetail = t
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m MyTasks) View() string {
	if len(m.tasks) == 0 {
		empty := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("\n  No tasks assigned to you.\n")
		footer := ui.RenderFooter(m.keyBindings(), m.width)
		return lipgloss.JoinVertical(lipgloss.Left, empty, footer)
	}

	previewW := m.previewWidth()
	tableW := m.width - previewW
	tableH := m.height - 3

	table := m.renderTable(tableW, tableH)

	var content string
	if previewW > 0 && m.SelectedTask() != nil {
		task := m.SelectedTask()
		preview := ui.RenderPreview(*task, previewW, tableH, m.listName(task.ListID))
		content = lipgloss.JoinHorizontal(lipgloss.Top, table, preview)
	} else {
		content = table
	}

	footer := ui.RenderFooter(m.keyBindings(), m.width)
	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m MyTasks) renderTable(width, height int) string {
	var sb strings.Builder

	// Column widths
	priW := 10
	statusW := 16
	timeW := 10
	listW := 18
	nameW := width - priW - statusW - timeW - listW - 10
	if nameW < 10 {
		nameW = 10
	}

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(ui.ColorFgBright).Bold(true)
	hPri := headerStyle.Width(priW).Render("Priority")
	hName := headerStyle.Width(nameW).Render("Task")
	hList := headerStyle.Width(listW).Render("List")
	hStatus := headerStyle.Width(statusW).Render("Status")
	hTime := headerStyle.Width(timeW).Render("Time")
	sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n", hPri, hName, hList, hStatus, hTime))

	divider := lipgloss.NewStyle().Foreground(ui.ColorBorder).Render(strings.Repeat("─", width-2))
	sb.WriteString("  " + divider + "\n")

	// Rows
	maxRows := height - 4
	for i, task := range m.tasks {
		if i >= maxRows {
			break
		}

		selected := i == m.cursor

		// Priority display
		priLabel, priColor := priorityDisplay(task.Priority)
		priStyle := lipgloss.NewStyle().Foreground(priColor).Width(priW)
		if selected {
			priStyle = priStyle.Bold(true)
		}
		priCell := priStyle.Render(priLabel)

		// Task name
		name := task.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		nameStyle := lipgloss.NewStyle().Foreground(ui.ColorFg).Width(nameW)
		if selected {
			nameStyle = nameStyle.Foreground(ui.ColorBlue).Bold(true)
		}
		nameCell := nameStyle.Render(name)

		// Status
		statusColor := lipgloss.Color(task.Status.Color)
		if task.Status.Color == "" {
			statusColor = ui.ColorFgDim
		}
		statusStyle := lipgloss.NewStyle().Foreground(statusColor).Width(statusW)
		statusCell := statusStyle.Render(task.Status.Status)

		// List name
		ln := m.listName(task.ListID)
		if len(ln) > listW {
			ln = ln[:listW-1] + "…"
		}
		listStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Width(listW)
		listCell := listStyle.Render(ln)

		// Time tracked
		timeStr := ""
		if task.TimeSpent > 0 {
			timeStr = ui.FormatDuration(task.TimeSpent)
		}
		timeStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Width(timeW)
		timeCell := timeStyle.Render(timeStr)

		prefix := "  "
		if selected {
			prefix = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%s  %s  %s  %s  %s\n", prefix, priCell, nameCell, listCell, statusCell, timeCell))
	}

	return ui.BorderStyle.
		Width(width - 2).
		Height(height).
		Render(sb.String())
}

func (m MyTasks) previewWidth() int {
	if m.width < 100 {
		return 0
	}
	return m.width / 3
}

func (m MyTasks) keyBindings() []ui.KeyBinding {
	return []ui.KeyBinding{
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "detail"},
		{Key: "1/2", Label: "switch view"},
		{Key: "r", Label: "refresh"},
		{Key: "q", Label: "quit"},
	}
}

func filterAndSortMyTasks(state AppState) []api.Task {
	if state.CurrentUser == nil {
		return nil
	}
	myID := state.CurrentUser.ID

	var mine []api.Task
	for _, task := range state.Tasks {
		for _, u := range task.Assignees {
			if u.ID == myID {
				mine = append(mine, task)
				break
			}
		}
	}

	sort.Slice(mine, func(i, j int) bool {
		ri := priorityRank(mine[i].Priority)
		rj := priorityRank(mine[j].Priority)
		return ri < rj
	})

	return mine
}

