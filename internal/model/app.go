package model

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nhinkley/clickban/internal/api"
	"github.com/nhinkley/clickban/internal/ui"
)

// ViewMode represents which top-level view is active.
type ViewMode int

const (
	ViewKanban  ViewMode = iota
	ViewMyTasks          // 1
	ViewDetail           // 2
)

// AppState holds shared data available to all views.
type AppState struct {
	Client      *api.Client
	TeamID      string
	SpaceID     string
	CurrentUser *api.User
	Members     []api.Member
	TaskTypes   []api.CustomItem
	Statuses    []api.Status
	Tasks       []api.Task
}

// DataLoadedMsg is sent when the initial data load completes.
type DataLoadedMsg struct {
	State AppState
	Err   error
}

// TaskUpdatedMsg is sent when a task has been updated.
type TaskUpdatedMsg struct {
	Task api.Task
}

// StatusMsg carries a transient status message for display.
type StatusMsg struct {
	Text string
}

// App is the root Bubble Tea model.
type App struct {
	state       AppState
	view        ViewMode
	kanban      Kanban
	myTasks     MyTasks
	detail      Detail
	detailFrom  ViewMode // which view we came from
	loading     bool
	statusText  string
	width       int
	height      int
	ready       bool
}

// NewApp creates a new App with the given client and IDs.
func NewApp(client *api.Client, teamID, spaceID string) App {
	return App{
		state: AppState{
			Client:  client,
			TeamID:  teamID,
			SpaceID: spaceID,
		},
		view:    ViewKanban,
		loading: true,
	}
}

// Init implements tea.Model — kick off initial data load.
func (a App) Init() tea.Cmd {
	return loadData(a.state.Client, a.state.TeamID, a.state.SpaceID)
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		if !a.loading {
			a.kanban = a.kanban.Resize(a.width, a.height)
			a.myTasks = a.myTasks.Resize(a.width, a.height)
			if a.view == ViewDetail {
				a.detail = a.detail.Resize(a.width, a.height)
			}
		}

	case DataLoadedMsg:
		if msg.Err != nil {
			a.statusText = "Error loading data: " + msg.Err.Error()
			a.loading = false
			return a, nil
		}
		a.state = msg.State
		a.loading = false
		a.kanban = NewKanban(a.state).Resize(a.width, a.height)
		a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)
		a.statusText = ""

	case StatusMsg:
		a.statusText = msg.Text

	case TaskUpdatedMsg:
		// Update the task in our state
		for i, t := range a.state.Tasks {
			if t.ID == msg.Task.ID {
				a.state.Tasks[i] = msg.Task
				break
			}
		}
		a.kanban = NewKanban(a.state).Resize(a.width, a.height)
		a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)

	case tea.KeyMsg:
		// Global keys when not in detail view
		if a.view != ViewDetail {
			switch msg.String() {
			case "q", "ctrl+c":
				return a, tea.Quit
			case "1":
				a.view = ViewKanban
				return a, nil
			case "2":
				a.view = ViewMyTasks
				return a, nil
			case "r":
				a.loading = true
				return a, loadData(a.state.Client, a.state.TeamID, a.state.SpaceID)
			case "enter":
				// Open detail view for selected task
				var task *api.Task
				if a.view == ViewKanban {
					task = a.kanban.SelectedTask()
				} else if a.view == ViewMyTasks {
					task = a.myTasks.SelectedTask()
				}
				if task != nil {
					a.detailFrom = a.view
					a.detail = NewDetail(*task, a.state).Resize(a.width, a.height)
					a.view = ViewDetail
				}
				return a, nil
			}
		} else {
			// In detail view, esc returns to previous view
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "q":
				// only quit if no overlay active
				if !a.detail.HasOverlay() {
					a.view = a.detailFrom
					return a, nil
				}
			}
		}
	}

	// Delegate updates to the active sub-model
	if !a.loading && a.ready {
		switch a.view {
		case ViewKanban:
			newKanban, cmd := a.kanban.Update(msg)
			a.kanban = newKanban
			// Check if kanban wants to open detail
			if sel := a.kanban.WantsDetail(); sel != nil {
				a.detailFrom = ViewKanban
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height)
				a.view = ViewDetail
				a.kanban.ClearWantsDetail()
			}
			return a, cmd

		case ViewMyTasks:
			newMyTasks, cmd := a.myTasks.Update(msg)
			a.myTasks = newMyTasks
			if sel := a.myTasks.WantsDetail(); sel != nil {
				a.detailFrom = ViewMyTasks
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height)
				a.view = ViewDetail
				a.myTasks.ClearWantsDetail()
			}
			return a, cmd

		case ViewDetail:
			newDetail, cmd := a.detail.Update(msg)
			a.detail = newDetail
			if a.detail.WantsBack() {
				// propagate any updates
				if updated := a.detail.UpdatedTask(); updated != nil {
					for i, t := range a.state.Tasks {
						if t.ID == updated.ID {
							a.state.Tasks[i] = *updated
							break
						}
					}
					a.kanban = NewKanban(a.state).Resize(a.width, a.height)
					a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)
				}
				a.view = a.detailFrom
				a.detail.ClearWantsBack()
			}
			return a, cmd
		}
	}

	return a, nil
}

// View implements tea.Model.
func (a App) View() string {
	if !a.ready {
		return "Initializing…\n"
	}
	if a.loading {
		return lipgloss.NewStyle().
			Foreground(ui.ColorBlue).
			Render("\n  Loading data from ClickUp…\n")
	}

	var content string
	switch a.view {
	case ViewKanban:
		content = a.kanban.View()
	case ViewMyTasks:
		content = a.myTasks.View()
	case ViewDetail:
		content = a.detail.View()
	}

	// Status bar at top
	header := renderHeader(a.view, a.statusText, a.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func renderHeader(view ViewMode, statusText string, width int) string {
	tabs := []string{"[1] Kanban", "[2] My Tasks"}
	var parts []string
	for i, tab := range tabs {
		mode := ViewMode(i)
		if mode == view {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ui.ColorBlue).
				Bold(true).
				Render(tab))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(ui.ColorFgDim).
				Render(tab))
		}
	}

	tabBar := strings.Join(parts, "  ")

	status := ""
	if statusText != "" {
		status = lipgloss.NewStyle().
			Foreground(ui.ColorYellow).
			Render("  " + statusText)
	}

	right := fmt.Sprintf("q:quit  r:refresh")
	rightStyle := lipgloss.NewStyle().
		Foreground(ui.ColorFgDim).
		Render(right)

	leftWidth := width - lipgloss.Width(rightStyle) - 2
	if leftWidth < 0 {
		leftWidth = 0
	}
	leftContent := lipgloss.NewStyle().Width(leftWidth).Render(tabBar + status)

	bar := lipgloss.NewStyle().
		Background(lipgloss.Color("#24283b")).
		Width(width).
		Padding(0, 1).
		Render(leftContent + rightStyle)

	return bar
}

// loadData fetches all required data from the API in one shot.
func loadData(client *api.Client, teamID, spaceID string) tea.Cmd {
	return func() tea.Msg {
		state := AppState{
			Client:  client,
			TeamID:  teamID,
			SpaceID: spaceID,
		}

		// Current user
		user, err := client.GetCurrentUser()
		if err != nil {
			return DataLoadedMsg{Err: fmt.Errorf("get current user: %w", err)}
		}
		state.CurrentUser = user

		// Workspace members
		members, err := client.GetWorkspaceMembers(teamID)
		if err != nil {
			return DataLoadedMsg{Err: fmt.Errorf("get members: %w", err)}
		}
		state.Members = members

		// Task types
		taskTypes, err := client.GetTaskTypes(teamID)
		if err != nil {
			// Non-fatal — continue without task types
			taskTypes = nil
		}
		state.TaskTypes = taskTypes

		// Space (for statuses)
		space, err := client.GetSpace(spaceID)
		if err != nil {
			return DataLoadedMsg{Err: fmt.Errorf("get space: %w", err)}
		}
		state.Statuses = space.Statuses

		// All lists in the space
		lists, err := client.GetAllLists(spaceID)
		if err != nil {
			return DataLoadedMsg{Err: fmt.Errorf("get lists: %w", err)}
		}

		// Gather statuses from lists too
		statusSeen := make(map[string]bool)
		for _, s := range state.Statuses {
			statusSeen[s.ID] = true
		}
		for _, list := range lists {
			for _, s := range list.Statuses {
				if !statusSeen[s.ID] {
					state.Statuses = append(state.Statuses, s)
					statusSeen[s.ID] = true
				}
			}
		}

		// Tasks for all lists
		var allTasks []api.Task
		for _, list := range lists {
			tasks, err := client.GetTasks(list.ID)
			if err != nil {
				// skip lists we can't read
				continue
			}
			allTasks = append(allTasks, tasks...)
		}

		// Resolve to leaf tasks
		state.Tasks = api.ResolveLeafTasks(allTasks, 5)

		return DataLoadedMsg{State: state}
	}
}
