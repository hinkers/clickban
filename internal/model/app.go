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
	Lists          []api.List
	Tasks          []api.Task
	RunningTaskID  string // task ID with a running timer (from ClickUp API)
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

// TaskCreatedMsg is sent when a new task has been created via the API.
type TaskCreatedMsg struct {
	Task *api.Task
	Err  error
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
	// Task creation state
	createListPicker *ui.Picker
	createEditor     *ui.Editor
	createListID     string // selected list for new task
	// Weekly summary overlay
	showWeekly bool
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
		viewH := a.height - 1 // reserve 1 line for header bar
		if !a.loading {
			a.today = a.today.Resize(a.width, viewH)
			a.kanban = a.kanban.Resize(a.width, viewH)
			a.myTasks = a.myTasks.Resize(a.width, viewH)
			if a.view == ViewDetail {
				a.detail = a.detail.Resize(a.width, viewH)
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
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height-1)
		a.myTasks = NewMyTasksWithFilter(a.state, a.myTasks.needsDataFilter, a.myTasks.showClosed).Resize(a.width, a.height-1)
		if a.today.calculated {
			a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height-1)
		} else {
			a.today = NewToday(a.state, a.cache).Resize(a.width, a.height-1)
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
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height-1)
		a.myTasks = NewMyTasksWithFilter(a.state, a.myTasks.needsDataFilter, a.myTasks.showClosed).Resize(a.width, a.height-1)
		a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height-1)

	case ui.PickerResult:
		// Handle create task list picker
		if a.createListPicker != nil {
			a.createListPicker = nil
			if msg.Cancelled || len(msg.Selected) == 0 {
				return a, nil
			}
			a.createListID = msg.Selected[0].ID
			editor := ui.NewEditor("New Task Name", "")
			a.createEditor = &editor
			return a, a.createEditor.Init()
		}

	case ui.EditorResult:
		// Handle create task name editor
		if a.createEditor != nil {
			a.createEditor = nil
			if msg.Cancelled || msg.Value == "" {
				return a, nil
			}
			name := msg.Value
			listID := a.createListID
			client := a.state.Client
			var assignees []int
			if a.state.CurrentUser != nil {
				assignees = []int{a.state.CurrentUser.ID}
			}
			return a, func() tea.Msg {
				task, err := client.CreateTask(listID, &api.CreateTaskRequest{
					Name:      name,
					Assignees: assignees,
				})
				return TaskCreatedMsg{Task: task, Err: err}
			}
		}

	case TaskCreatedMsg:
		if msg.Err != nil {
			a.statusText = "Create task failed: " + msg.Err.Error()
			return a, nil
		}
		if msg.Task != nil {
			a.state.Tasks = append(a.state.Tasks, *msg.Task)
			a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height-1)
			a.myTasks = NewMyTasksWithFilter(a.state, a.myTasks.needsDataFilter, a.myTasks.showClosed).Resize(a.width, a.height-1)
			a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height-1)
			// Open the new task in detail view
			a.detailFrom = a.view
			a.detail = NewDetail(*msg.Task, a.state).Resize(a.width, a.height-1)
			a.detail.SetPlannedToday(a.isTaskForcedToday(msg.Task.ID))
			a.view = ViewDetail
			a.statusText = "Task created"
			return a, a.detail.Init()
		}

	case ToggleTodayForceMsg:
		if a.cache != nil {
			actions := a.today.TodayActions()
			taskID := msg.TaskID
			forced := false
			if actions[taskID] == "forced" {
				a.today.removeAction(taskID)
			} else {
				a.today.setAction(taskID, "forced")
				forced = true
			}
			a.today.recalculate()
			return a, func() tea.Msg {
				return TodayForceUpdatedMsg{TaskID: taskID, Forced: forced}
			}
		}
		return a, nil

	case tea.KeyMsg:
		// Route to create overlays if active
		if a.createListPicker != nil {
			p := *a.createListPicker
			m, cmd := p.Update(msg)
			np := m.(ui.Picker)
			a.createListPicker = &np
			return a, cmd
		}
		if a.createEditor != nil {
			m, cmd := a.createEditor.Update(msg)
			editor := m.(ui.Editor)
			a.createEditor = &editor
			return a, cmd
		}
		if a.showWeekly {
			switch msg.String() {
			case "q", "esc", "w":
				a.showWeekly = false
			}
			return a, nil
		}

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
			case "tab":
				switch a.view {
				case ViewToday:
					a.view = ViewKanban
				case ViewKanban:
					a.view = ViewMyTasks
				case ViewMyTasks:
					a.view = ViewToday
				}
				return a, nil
			case "shift+tab":
				switch a.view {
				case ViewToday:
					a.view = ViewMyTasks
				case ViewKanban:
					a.view = ViewToday
				case ViewMyTasks:
					a.view = ViewKanban
				}
				return a, nil
			case "w":
				if a.view == ViewToday {
					a.showWeekly = true
				}
				return a, nil
			case "n":
				// Create new task — open list picker
				var items []ui.PickerItem
				for _, l := range a.state.Lists {
					items = append(items, ui.PickerItem{ID: l.ID, Label: l.Name})
				}
				if len(items) > 0 {
					p := ui.NewPicker("Create task in list", items, false)
					a.createListPicker = &p
				}
				return a, nil
			case "t":
				// Jump to running timer task
				if a.state.RunningTaskID != "" {
					for _, task := range a.state.Tasks {
						if task.ID == a.state.RunningTaskID {
							a.detailFrom = a.view
							a.detail = NewDetail(task, a.state).Resize(a.width, a.height-1)
							a.detail.SetPlannedToday(a.isTaskForcedToday(task.ID))
							a.view = ViewDetail
							return a, a.detail.Init()
						}
					}
				}
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
					a.detail = NewDetail(*task, a.state).Resize(a.width, a.height-1)
					a.detail.SetPlannedToday(a.isTaskForcedToday(task.ID))
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
					taskID := a.detail.task.ID
					a.propagateDetailUpdates()
					a.view = a.detailFrom
					if a.view == ViewKanban {
						a.kanban.FocusTask(taskID)
					}
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
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height-1)
				a.detail.SetPlannedToday(a.isTaskForcedToday(sel.ID))
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
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height-1)
				a.detail.SetPlannedToday(a.isTaskForcedToday(sel.ID))
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
				a.detail = NewDetail(*sel, a.state).Resize(a.width, a.height-1)
				a.detail.SetPlannedToday(a.isTaskForcedToday(sel.ID))
				a.view = ViewDetail
				a.myTasks.ClearWantsDetail()
				return a, a.detail.Init()
			}
			return a, cmd

		case ViewDetail:
			newDetail, cmd := a.detail.Update(msg)
			a.detail = newDetail
			a.state.RunningTaskID = a.detail.state.RunningTaskID
			if a.detail.WantsBack() {
				taskID := a.detail.task.ID
				a.propagateDetailUpdates()
				a.view = a.detailFrom
				if a.view == ViewKanban {
					a.kanban.FocusTask(taskID)
				}
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

	// Weekly summary — full screen, skip normal view
	if a.showWeekly {
		weeklyContent := RenderWeeklySummary(a.state, a.width-6, a.height-6)
		hint := lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("\nq/esc/w: close")
		return lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorBorderAct).
			Background(ui.ColorCardBg).
			Padding(1, 2).
			Width(a.width - 2).
			Height(a.height - 2).
			Render(weeklyContent + hint)
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
	header := renderHeader(a.view, a.statusText, a.validationCount(), a.runningTimerLabel(), a.width)

	result := lipgloss.JoinVertical(lipgloss.Left, header, content)

	// Render create task overlays
	if a.createListPicker != nil || a.createEditor != nil {
		var overlayContent string
		if a.createListPicker != nil {
			overlayContent = a.createListPicker.View()
		} else if a.createEditor != nil {
			overlayContent = a.createEditor.View()
		}
		overlayBox := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorBorderAct).
			Background(ui.ColorCardBg).
			Padding(1, 2).
			Width(50).
			Render(overlayContent)

		boxH := lipgloss.Height(overlayBox)
		boxW := lipgloss.Width(overlayBox)
		padTop := max(0, (a.height-boxH)/2)
		padLeft := max(0, (a.width-boxW)/2)
		indent := strings.Repeat(" ", padLeft)
		lines := strings.Split(result, "\n")
		overlayLines := strings.Split(overlayBox, "\n")
		for i, ol := range overlayLines {
			row := padTop + i
			if row < len(lines) {
				lines[row] = indent + ol
			}
		}
		result = strings.Join(lines, "\n")
	}

	return result
}

func (a *App) isTaskForcedToday(taskID string) bool {
	return a.today.TodayActions()[taskID] == "forced"
}

func (a *App) propagateDetailUpdates() {
	// Always sync running timer state back from detail
	a.state.RunningTaskID = a.detail.state.RunningTaskID

	if updated := a.detail.UpdatedTask(); updated != nil {
		task := *updated
		for i, t := range a.state.Tasks {
			if t.ID == task.ID {
				a.state.Tasks[i] = task
				break
			}
		}
		a.kanban = NewKanbanWithOptions(a.state, a.kanban.showClosed, a.kanban.sortMode).Resize(a.width, a.height-1)
		a.myTasks = NewMyTasksWithFilter(a.state, a.myTasks.needsDataFilter, a.myTasks.showClosed).Resize(a.width, a.height-1)
		a.today = NewTodayWithState(a.state, a.cache, a.today.todayActions).Resize(a.width, a.height-1)
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

// runningTimerLabel returns a display string for the running timer, or empty.
func (a *App) runningTimerLabel() string {
	if a.state.RunningTaskID == "" {
		return ""
	}
	for _, t := range a.state.Tasks {
		if t.ID == a.state.RunningTaskID {
			name := t.Name
			if len(name) > 30 {
				name = name[:29] + "…"
			}
			return name
		}
	}
	return "unknown task"
}

func renderHeader(view ViewMode, statusText string, validationCount int, timerLabel string, width int) string {
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

	timerStr := ""
	if timerLabel != "" {
		timerStr = lipgloss.NewStyle().
			Foreground(ui.ColorGreen).
			Bold(true).
			Render("⏱ " + timerLabel + "  ")
	}

	right := "q:quit  r:refresh"
	if timerLabel != "" {
		right = "t:timer  " + right
	}
	rightStyle := lipgloss.NewStyle().
		Foreground(ui.ColorFgDim).
		Render(timerStr + right)

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

	// Check for running timer (non-fatal if it fails)
	if timer, err := client.GetRunningTimer(teamID); err == nil && timer != nil {
		state.RunningTaskID = timer.TaskID
	}

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
