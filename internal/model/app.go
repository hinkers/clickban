package model

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/cache"
	"github.com/hinkers/clickban/internal/ui"
)

// ViewMode represents which top-level view is active.
type ViewMode int

const (
	ViewToday   ViewMode = iota // 0
	ViewKanban                  // 1
	ViewMyTasks                 // 2
	ViewDetail                  // 3
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
	Lists       []api.List
	Tasks       []api.Task
}

// CachedState holds the serializable parts of AppState for caching.
type CachedState struct {
	CurrentUser *api.User        `json:"current_user"`
	Members     []api.Member     `json:"members"`
	TaskTypes   []api.CustomItem `json:"task_types"`
	Statuses    []api.Status     `json:"statuses"`
	Lists       []api.List       `json:"lists"`
	Tasks       []api.Task       `json:"tasks"`
}

// DataLoadedMsg is sent when the initial data load completes.
type DataLoadedMsg struct {
	State    AppState
	Err      error
	FromCache bool
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
	today       Today
	kanban      Kanban
	myTasks     MyTasks
	detail      Detail
	detailFrom  ViewMode // which view we came from
	cache       *cache.Cache
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
		view:    ViewToday,
		loading: true,
	}
}

// Init implements tea.Model — try cache first, then load from API.
func (a App) Init() tea.Cmd {
	return func() tea.Msg {
		c, err := cache.Open()
		if err == nil {
			defer c.Close()
			var cached CachedState
			if ok, _ := c.Get("state:"+a.state.SpaceID, &cached); ok {
				state := AppState{
					Client:      a.state.Client,
					TeamID:      a.state.TeamID,
					SpaceID:     a.state.SpaceID,
					CurrentUser: cached.CurrentUser,
					Members:     cached.Members,
					TaskTypes:   cached.TaskTypes,
					Statuses:    cached.Statuses,
					Lists:       cached.Lists,
					Tasks:       cached.Tasks,
				}
				return DataLoadedMsg{State: state, FromCache: true}
			}
		}
		// No cache — load from API directly
		return loadDataSync(a.state.Client, a.state.TeamID, a.state.SpaceID)
	}
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		if !a.loading {
			a.today = a.today.Resize(a.width, a.height)
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
		if a.cache == nil {
			if c, err := cache.Open(); err == nil {
				a.cache = c
			}
		}
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height)
		a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)
		if a.today.calculated {
			a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height)
		} else {
			a.today = NewToday(a.state, a.cache).Resize(a.width, a.height)
		}
		a.statusText = ""
		if msg.FromCache {
			a.statusText = "Loaded from cache, refreshing…"
			return a, loadData(a.state.Client, a.state.TeamID, a.state.SpaceID)
		}

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
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height)
		a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)
		a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height)

	case tea.KeyMsg:
		// Global keys when not in detail view
		if a.view != ViewDetail {
			switch msg.String() {
			case "q", "ctrl+c":
				return a, tea.Quit
			case "1":
				a.view = ViewToday
				return a, nil
			case "2":
				a.view = ViewKanban
				return a, nil
			case "3":
				a.view = ViewMyTasks
				return a, nil
			case "r":
				a.loading = true
				return a, loadData(a.state.Client, a.state.TeamID, a.state.SpaceID)
			case "enter":
				// Don't intercept enter when an overlay is active
				if a.view == ViewKanban && a.kanban.HasOverlay() {
					break
				}
				if a.view == ViewToday && a.today.HasOverlay() {
					break
				}
				// Open detail view for selected task
				var task *api.Task
				if a.view == ViewToday {
					task = a.today.SelectedTask()
				} else if a.view == ViewKanban {
					task = a.kanban.SelectedTask()
				} else if a.view == ViewMyTasks {
					task = a.myTasks.SelectedTask()
				}
				if task != nil {
					a.detailFrom = a.view
					a.detail = NewDetail(*task, a.state).Resize(a.width, a.height)
					a.view = ViewDetail
					return a, a.detail.Init()
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
					a.propagateDetailUpdates()
					a.view = a.detailFrom
					return a, nil
				}
			}
		}
	}

	// Delegate updates to the active sub-model
	if !a.loading && a.ready {
		switch a.view {
		case ViewToday:
			newToday, cmd := a.today.Update(msg)
			a.today = newToday
			if sel := a.today.WantsDetail(); sel != nil {
				a.detailFrom = ViewToday
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height)
				a.view = ViewDetail
				a.today.ClearWantsDetail()
				return a, a.detail.Init()
			}
			return a, cmd

		case ViewKanban:
			newKanban, cmd := a.kanban.Update(msg)
			a.kanban = newKanban
			// Check if kanban wants to open detail
			if sel := a.kanban.WantsDetail(); sel != nil {
				a.detailFrom = ViewKanban
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height)
				a.view = ViewDetail
				a.kanban.ClearWantsDetail()
				return a, a.detail.Init()
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
				return a, a.detail.Init()
			}
			return a, cmd

		case ViewDetail:
			newDetail, cmd := a.detail.Update(msg)
			a.detail = newDetail
			if a.detail.WantsBack() {
				a.propagateDetailUpdates()
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
	case ViewToday:
		content = a.today.View()
	case ViewKanban:
		content = a.kanban.View()
	case ViewMyTasks:
		content = a.myTasks.View()
	case ViewDetail:
		content = a.detail.View()
	}

	// Status bar at top
	header := renderHeader(a.view, a.statusText, a.validationCount(), a.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

func (a *App) propagateDetailUpdates() {
	if updated := a.detail.UpdatedTask(); updated != nil {
		task := *updated
		for i, t := range a.state.Tasks {
			if t.ID == task.ID {
				a.state.Tasks[i] = task
				break
			}
		}
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height)
		a.myTasks = NewMyTasks(a.state).Resize(a.width, a.height)
		a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height)
	}
}

// validationCount returns the number of assigned, non-closed tasks that need data.
func (a *App) validationCount() int {
	count := 0
	for _, t := range a.state.Tasks {
		if isClosedStatus(t.Status) {
			continue
		}
		assigned := false
		for _, u := range t.Assignees {
			if a.state.CurrentUser != nil && u.ID == a.state.CurrentUser.ID {
				assigned = true
				break
			}
		}
		if assigned && taskNeedsData(t) {
			count++
		}
	}
	return count
}

func renderHeader(view ViewMode, statusText string, validationCount int, width int) string {
	tabs := []string{"[1] Today", "[2] Kanban", "[3] My Tasks"}
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

	validationWarning := ui.RenderValidationWarning(validationCount)
	if validationWarning != "" {
		status += "  " + validationWarning
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

// loadData fetches all required data from the API and caches it.
func loadData(client *api.Client, teamID, spaceID string) tea.Cmd {
	return func() tea.Msg {
		return loadDataSync(client, teamID, spaceID)
	}
}

func loadDataSync(client *api.Client, teamID, spaceID string) DataLoadedMsg {
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

	state.Lists = lists

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

	// Save to cache
	if c, err := cache.Open(); err == nil {
		cached := CachedState{
			CurrentUser: state.CurrentUser,
			Members:     state.Members,
			TaskTypes:   state.TaskTypes,
			Statuses:    state.Statuses,
			Lists:       state.Lists,
			Tasks:       state.Tasks,
		}
		c.Set("state:"+spaceID, cached)
		c.Close()
	}

	return DataLoadedMsg{State: state}
}
