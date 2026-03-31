package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// DetailFocus controls which panel is focused in the detail view.
type DetailFocus int

const (
	FocusMain     DetailFocus = iota
	FocusComments             // comment thread panel
)

// DetailOverlay represents which (if any) overlay is currently shown.
type DetailOverlay int

const (
	OverlayNone DetailOverlay = iota
	OverlayStatus
	OverlayPriority
	OverlayType
	OverlayAssignees
	OverlayTitle
	OverlayDescription
	OverlayComment   // add new comment
	OverlayEditComment // edit an existing comment
	OverlayTimer
	OverlayTimeEstimate
	OverlayDueDate
	OverlayTimeEntries
)

// Detail is the task detail model.
type Detail struct {
	state       AppState
	task        api.Task
	comments    []api.Comment
	loadingCmts bool

	focus   DetailFocus
	overlay DetailOverlay
	cmtCursor int

	// overlay sub-models
	picker         ui.Picker
	editor         ui.Editor
	multiEditor    *ui.MultiLineEditor
	dueDateEditor  ui.DueDateEditor
	timer          ui.TimerInput
	runningTimer    *api.RunningTimer // non-nil when a timer is running for this task
	timeEntries     []api.TimeEntry
	timeEntriesList ui.TimeEntriesList
	editingEntryID  string // entry ID being edited, used during edit flow

	wantsBack   bool
	updatedTask *api.Task
	statusMsg   string
	mainScroll  int
	width       int
	height      int
}

// NewDetail creates a Detail view for the given task.
func NewDetail(task api.Task, state AppState) Detail {
	d := Detail{
		state: state,
		task:  task,
	}
	return d
}

// Resize sets terminal dimensions.
func (d Detail) Resize(w, h int) Detail {
	d.width = w
	d.height = h
	return d
}

// HasOverlay returns true if an overlay is currently open.
func (d *Detail) HasOverlay() bool {
	return d.overlay != OverlayNone
}

// WantsBack returns true if the detail view wants to return to the parent.
func (d *Detail) WantsBack() bool {
	return d.wantsBack
}

// ClearWantsBack clears the back flag.
func (d *Detail) ClearWantsBack() {
	d.wantsBack = false
}

// UpdatedTask returns the updated task if it was modified, else nil.
func (d *Detail) UpdatedTask() *api.Task {
	return d.updatedTask
}

// commentsLoadedMsg is an internal message for loaded comments.
type commentsLoadedMsg struct {
	taskID   string
	comments []api.Comment
	err      error
}

// runningTimerMsg is an internal message for the loaded running timer.
type runningTimerMsg struct {
	timer *api.RunningTimer
	err   error
}

// taskRefreshedMsg is an internal message for a refreshed task.
type taskRefreshedMsg struct {
	task *api.Task
	err  error
}

// timerTickMsg triggers a re-render for the live timer display.
type timerTickMsg struct{}

// timeEntriesLoadedMsg is the result of loading time entries.
type timeEntriesLoadedMsg struct {
	entries []api.TimeEntry
	err     error
}

// Init implements tea.Model — load comments.
func (d Detail) Init() tea.Cmd {
	return tea.Batch(d.loadComments(), d.loadRunningTimer(), d.loadTimeEntries())
}

func (d Detail) loadComments() tea.Cmd {
	client := d.state.Client
	taskID := d.task.ID
	return func() tea.Msg {
		comments, err := client.GetComments(taskID)
		return commentsLoadedMsg{taskID: taskID, comments: comments, err: err}
	}
}

func (d Detail) loadTask() tea.Cmd {
	client := d.state.Client
	taskID := d.task.ID
	return func() tea.Msg {
		task, err := client.GetTask(taskID)
		return taskRefreshedMsg{task: task, err: err}
	}
}

func (d Detail) loadRunningTimer() tea.Cmd {
	client := d.state.Client
	teamID := d.state.TeamID
	return func() tea.Msg {
		timer, err := client.GetRunningTimer(teamID)
		return runningTimerMsg{timer: timer, err: err}
	}
}

func (d Detail) loadTimeEntries() tea.Cmd {
	client := d.state.Client
	taskID := d.task.ID
	return func() tea.Msg {
		entries, err := client.GetTimeEntries(taskID)
		return timeEntriesLoadedMsg{entries: entries, err: err}
	}
}

// Update implements tea.Model.
func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {

	case taskRefreshedMsg:
		if msg.err != nil {
			d.statusMsg = "refresh failed: " + msg.err.Error()
		} else if msg.task != nil {
			d.task = *msg.task
			d.statusMsg = "Refreshed"
			updated := d.task
			d.updatedTask = &updated
		}
		return d, nil

	case commentsLoadedMsg:
		if msg.taskID == d.task.ID {
			if msg.err == nil {
				d.comments = msg.comments
			}
			d.loadingCmts = false
		}

	case runningTimerMsg:
		if msg.err != nil {
			d.statusMsg = "timer check failed: " + msg.err.Error()
			return d, nil
		}
		if msg.timer != nil {
			d.state.RunningTaskID = msg.timer.TaskID
			if msg.timer.TaskID == d.task.ID {
				d.runningTimer = msg.timer
				return d, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{} })
			}
		}
		return d, nil

	case timeEntriesLoadedMsg:
		if msg.err == nil {
			d.timeEntries = msg.entries
		}
		return d, nil

	case timerTickMsg:
		if d.runningTimer != nil {
			return d, tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{} })
		}
		return d, nil

	case StatusMsg:
		d.statusMsg = msg.Text
		return d, nil

	case ui.PickerResult:
		return d.handlePickerResult(msg)

	case ui.EditorResult:
		return d.handleEditorResult(msg)

	case ui.TimerResult:
		return d.handleTimerResult(msg)

	case ui.TimeEntryAction:
		return d.handleTimeEntryAction(msg)

	case ui.ExternalEditorResult:
		return d.handleExternalEditorResult(msg)

	case tea.KeyMsg:
		if d.overlay != OverlayNone {
			return d.updateOverlay(msg)
		}
		return d.updateMain(msg)
	}

	// Delegate to active overlay sub-models for non-key messages
	if d.overlay != OverlayNone {
		return d.delegateToOverlay(msg)
	}

	return d, nil
}

func (d Detail) delegateToOverlay(msg tea.Msg) (Detail, tea.Cmd) {
	switch d.overlay {
	case OverlayStatus, OverlayPriority, OverlayType, OverlayAssignees:
		m, cmd := d.picker.Update(msg)
		d.picker = m.(ui.Picker)
		return d, cmd
	case OverlayDescription:
		if d.multiEditor != nil {
			m, cmd := d.multiEditor.Update(msg)
			me := m.(ui.MultiLineEditor)
			d.multiEditor = &me
			return d, cmd
		}
		return d, nil
	case OverlayTitle, OverlayComment, OverlayEditComment, OverlayTimeEstimate:
		m, cmd := d.editor.Update(msg)
		d.editor = m.(ui.Editor)
		return d, cmd
	case OverlayDueDate:
		m, cmd := d.dueDateEditor.Update(msg)
		d.dueDateEditor = m.(ui.DueDateEditor)
		return d, cmd
	case OverlayTimer:
		m, cmd := d.timer.Update(msg)
		d.timer = m.(ui.TimerInput)
		return d, cmd
	case OverlayTimeEntries:
		m, cmd := d.timeEntriesList.Update(msg)
		d.timeEntriesList = m.(ui.TimeEntriesList)
		return d, cmd
	}
	return d, nil
}

func (d Detail) updateMain(msg tea.KeyMsg) (Detail, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		d.wantsBack = true

	case "tab":
		if d.focus == FocusMain {
			d.focus = FocusComments
		} else {
			d.focus = FocusMain
		}

	case "r":
		if d.focus == FocusMain {
			d.statusMsg = "Refreshing…"
			return d, tea.Batch(d.loadTask(), d.loadComments())
		}

	case "j", "down":
		if d.focus == FocusMain {
			d.mainScroll++
		} else if d.focus == FocusComments {
			if d.cmtCursor < len(d.comments)-1 {
				d.cmtCursor++
			}
		}

	case "k", "up":
		if d.focus == FocusMain {
			if d.mainScroll > 0 {
				d.mainScroll--
			}
		} else if d.focus == FocusComments {
			if d.cmtCursor > 0 {
				d.cmtCursor--
			}
		}

	// Field edit keys (main focus only)
	case "i":
		if d.focus == FocusMain {
			d.editor = ui.NewEditor("Edit Title", d.task.Name)
			d.overlay = OverlayTitle
			return d, d.editor.Init()
		}

	case "e":
		if d.focus == FocusMain {
			// multiline editor for description — use overlay-relative dimensions
			overlayWidth := max(60, d.width-10)
			me := ui.NewMultiLineEditor("Edit Description", d.task.Description, overlayWidth-6, d.height)
			d.multiEditor = &me
			d.overlay = OverlayDescription
			return d, d.multiEditor.Init()
		} else if d.focus == FocusComments && len(d.comments) > 0 {
			// edit own comment
			cmt := d.comments[d.cmtCursor]
			if d.state.CurrentUser != nil && cmt.User.ID == d.state.CurrentUser.ID {
				d.editor = ui.NewEditor("Edit Comment", cmt.CommentText)
				d.overlay = OverlayEditComment
				return d, d.editor.Init()
			}
		}

	case "E":
		if d.focus == FocusMain {
			// open external editor for description
			d.overlay = OverlayDescription
			return d, ui.OpenExternalEditor(d.task.Description)
		}

	case "s":
		if d.focus == FocusMain {
			d.overlay = OverlayStatus
			items := d.statusPickerItems()
			d.picker = ui.NewPicker("Set Status", items, false)
		}

	case "p":
		if d.focus == FocusMain {
			d.overlay = OverlayPriority
			items := []ui.PickerItem{
				{ID: "1", Label: "Urgent"},
				{ID: "2", Label: "High"},
				{ID: "3", Label: "Normal"},
				{ID: "4", Label: "Low"},
			}
			d.picker = ui.NewPicker("Set Priority", items, false)
		}

	case "y":
		if d.focus == FocusMain {
			d.overlay = OverlayType
			items := d.typePickerItems()
			d.picker = ui.NewPicker("Set Task Type", items, false)
		}

	case "a":
		if d.focus == FocusMain {
			d.overlay = OverlayAssignees
			items, selectedIndices := d.assigneePickerItems()
			p := ui.NewPicker("Set Assignees", items, true)
			for _, idx := range selectedIndices {
				p.SetSelected(idx, true)
			}
			d.picker = p
		}

	case "t":
		if d.focus == FocusMain {
			d.timer = ui.NewTimerInputWithRunning(d.runningTimer != nil)
			d.overlay = OverlayTimer
		}

	case "T":
		if d.focus == FocusMain {
			d.overlay = OverlayTimeEstimate
			d.editor = ui.NewEditor("Time Estimate (e.g. 2h30m)", "")
			return d, d.editor.Init()
		}

	case "D":
		if d.focus == FocusMain {
			d.overlay = OverlayDueDate
			d.dueDateEditor = ui.NewDueDateEditor()
			return d, d.dueDateEditor.Init()
		}

	case "L":
		if d.focus == FocusMain {
			d.timeEntriesList = ui.NewTimeEntriesList(d.timeEntries)
			d.overlay = OverlayTimeEntries
			return d, nil
		}

	case "c":
		// Add comment (works in either focus)
		d.editor = ui.NewEditor("Add Comment", "")
		d.overlay = OverlayComment
		return d, d.editor.Init()

	}
	return d, nil
}

func (d Detail) updateOverlay(msg tea.KeyMsg) (Detail, tea.Cmd) {
	return d.delegateToOverlay(msg)
}

func (d Detail) handlePickerResult(res ui.PickerResult) (Detail, tea.Cmd) {
	if res.Cancelled {
		d.overlay = OverlayNone
		return d, nil
	}

	overlay := d.overlay
	d.overlay = OverlayNone

	if len(res.Selected) == 0 {
		return d, nil
	}

	client := d.state.Client
	taskID := d.task.ID

	switch overlay {
	case OverlayStatus:
		newStatus := res.Selected[0].Label
		newStatusLower := strings.ToLower(newStatus)
		d.task.Status.Status = newStatus
		d.task.Status.ID = res.Selected[0].ID
		// Look up the status type so isClosedStatus works immediately
		d.task.Status.Type = d.lookupStatusType(res.Selected[0].ID)
		updated := d.task
		d.updatedTask = &updated
		return d, func() tea.Msg {
			err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Status: &newStatusLower})
			if err != nil {
				return StatusMsg{Text: "status update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Status → " + newStatus}
		}

	case OverlayPriority:
		priID := res.Selected[0].ID
		priInt, err := strconv.Atoi(priID)
		if err == nil {
			d.task.Priority = &api.Priority{ID: priID, Priority: strings.ToLower(res.Selected[0].Label)}
			return d, func() tea.Msg {
				if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Priority: &priInt}); err != nil {
					return StatusMsg{Text: "priority update failed: " + err.Error()}
				}
				return StatusMsg{Text: "Priority updated"}
			}
		}

	case OverlayType:
		typeID, err := strconv.Atoi(res.Selected[0].ID)
		if err == nil {
			d.task.CustomItem = &api.CustomItem{ID: typeID, Name: res.Selected[0].Label}
			updated := d.task
			d.updatedTask = &updated
			return d, func() tea.Msg {
				body := map[string]interface{}{"custom_item_id": typeID}
				if err := client.Put(fmt.Sprintf("/task/%s", taskID), body, nil); err != nil {
					return StatusMsg{Text: "task type update failed: " + err.Error()}
				}
				return StatusMsg{Text: "Task type updated"}
			}
		}

	case OverlayAssignees:
		// Compute add/remove lists
		var addIDs, removeIDs []int
		currentIDs := make(map[int]bool)
		for _, u := range d.task.Assignees {
			currentIDs[u.ID] = true
		}

		pickerItems, _ := d.assigneePickerItems()
		selectedIDs := make(map[string]bool)
		for _, sel := range res.Selected {
			selectedIDs[sel.ID] = true
		}

		for _, item := range pickerItems {
			id, err := strconv.Atoi(item.ID)
			if err != nil {
				continue
			}
			wasSelected := currentIDs[id]
			nowSelected := selectedIDs[item.ID]
			if nowSelected && !wasSelected {
				addIDs = append(addIDs, id)
			} else if !nowSelected && wasSelected {
				removeIDs = append(removeIDs, id)
			}
		}

		// Optimistically update local assignees
		var newAssignees []api.User
		for _, item := range pickerItems {
			if selectedIDs[item.ID] {
				id, _ := strconv.Atoi(item.ID)
				newAssignees = append(newAssignees, api.User{ID: id, Username: item.Label})
			}
		}
		d.task.Assignees = newAssignees
		updated := d.task
		d.updatedTask = &updated

		assignees := &api.Assignees{Add: addIDs, Remove: removeIDs}
		return d, func() tea.Msg {
			if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Assignees: assignees}); err != nil {
				return StatusMsg{Text: "assignees update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Assignees updated"}
		}
	}
	return d, nil
}

func (d Detail) handleEditorResult(res ui.EditorResult) (Detail, tea.Cmd) {
	if res.Cancelled {
		d.overlay = OverlayNone
		return d, nil
	}

	overlay := d.overlay
	d.overlay = OverlayNone

	client := d.state.Client
	taskID := d.task.ID
	value := res.Value

	switch overlay {
	case OverlayTitle:
		d.task.Name = value
		return d, func() tea.Msg {
			if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Name: value}); err != nil {
				return StatusMsg{Text: "title update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Title updated"}
		}

	case OverlayDescription:
		d.task.Description = value
		return d, func() tea.Msg {
			if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Description: value}); err != nil {
				return StatusMsg{Text: "description update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Description updated"}
		}

	case OverlayComment:
		if value == "" {
			return d, nil
		}
		return d, func() tea.Msg {
			if err := client.CreateComment(taskID, value); err != nil {
				return StatusMsg{Text: "comment failed: " + err.Error()}
			}
			// Reload comments
			comments, _ := client.GetComments(taskID)
			return commentsLoadedMsg{taskID: taskID, comments: comments}
		}

	case OverlayEditComment:
		if len(d.comments) == 0 {
			return d, nil
		}
		cmt := d.comments[d.cmtCursor]
		cmtID := cmt.ID
		return d, func() tea.Msg {
			if err := client.UpdateComment(cmtID, value); err != nil {
				return StatusMsg{Text: "comment edit failed: " + err.Error()}
			}
			comments, _ := client.GetComments(taskID)
			return commentsLoadedMsg{taskID: taskID, comments: comments}
		}

	case OverlayTimeEstimate:
		ms, err := ui.ParseDuration(value)
		if err != nil {
			d.statusMsg = "Invalid duration: " + value
			return d, nil
		}
		d.task.TimeEstimate = ms
		updated := d.task
		d.updatedTask = &updated
		return d, func() tea.Msg {
			err := client.UpdateTask(taskID, &api.UpdateTaskRequest{TimeEstimate: &ms})
			if err != nil {
				return StatusMsg{Text: "time estimate update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Time estimate updated"}
		}

	case OverlayDueDate:
		t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Now().Location())
		if err != nil {
			d.statusMsg = "Invalid date (use YYYY-MM-DD): " + value
			return d, nil
		}
		ms := t.UnixMilli()
		d.task.DueDate = fmt.Sprintf("%d", ms)
		updated := d.task
		d.updatedTask = &updated
		return d, func() tea.Msg {
			err := client.UpdateTask(taskID, &api.UpdateTaskRequest{DueDate: &ms})
			if err != nil {
				return StatusMsg{Text: "due date update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Due date updated"}
		}
	}
	return d, nil
}

func (d Detail) handleExternalEditorResult(res ui.ExternalEditorResult) (Detail, tea.Cmd) {
	if res.Err != nil {
		d.statusMsg = "editor error: " + res.Err.Error()
		return d, nil
	}
	value := strings.TrimSpace(res.Content)
	d.task.Description = value
	client := d.state.Client
	taskID := d.task.ID
	return d, func() tea.Msg {
		if err := client.UpdateTask(taskID, &api.UpdateTaskRequest{Description: value}); err != nil {
			return StatusMsg{Text: "description update failed: " + err.Error()}
		}
		return StatusMsg{Text: "Description updated"}
	}
}

func (d Detail) handleTimeEntryAction(action ui.TimeEntryAction) (Detail, tea.Cmd) {
	switch action.Action {
	case "close":
		d.overlay = OverlayNone
		return d, nil

	case "delete":
		client := d.state.Client
		teamID := d.state.TeamID
		entryID := action.Entry.ID
		durationMs, _ := strconv.ParseInt(action.Entry.Duration, 10, 64)

		// Remove from local list
		d.timeEntriesList.RemoveEntry(entryID)
		for i, e := range d.timeEntries {
			if e.ID == entryID {
				d.timeEntries = append(d.timeEntries[:i], d.timeEntries[i+1:]...)
				break
			}
		}
		d.task.TimeSpent -= durationMs
		if d.task.TimeSpent < 0 {
			d.task.TimeSpent = 0
		}
		updated := d.task
		d.updatedTask = &updated

		return d, func() tea.Msg {
			if err := client.DeleteTimeEntry(teamID, entryID); err != nil {
				return StatusMsg{Text: "delete failed: " + err.Error()}
			}
			return StatusMsg{Text: "Time entry deleted"}
		}

	case "edit":
		d.editingEntryID = action.Entry.ID
		d.overlay = OverlayTimer
		d.timer = ui.NewTimerInputWithRunning(false)
		return d, nil
	}

	return d, nil
}

func (d Detail) handleTimerResult(res ui.TimerResult) (Detail, tea.Cmd) {
	d.overlay = OverlayNone
	if res.Cancelled {
		d.editingEntryID = ""
		return d, nil
	}

	client := d.state.Client
	taskID := d.task.ID
	teamID := d.state.TeamID

	// If editing an existing entry, update it instead of creating new
	if d.editingEntryID != "" {
		entryID := d.editingEntryID
		d.editingEntryID = ""

		var newStart, newEnd int64
		var newDurationMs int64

		if res.Mode == ui.TimerModeDuration {
			newDurationMs = res.DurationMs
			now := time.Now()
			newStart = now.Add(-time.Duration(newDurationMs) * time.Millisecond).UnixMilli()
			newEnd = now.UnixMilli()
		} else if res.Mode == ui.TimerModeTimeRange {
			newStart = res.Start.UnixMilli()
			newEnd = res.End.UnixMilli()
			newDurationMs = res.End.Sub(res.Start).Milliseconds()
		} else {
			// Live mode doesn't apply to edits
			return d, nil
		}

		// Update local state: adjust time spent
		for i, e := range d.timeEntries {
			if e.ID == entryID {
				oldDurationMs, _ := strconv.ParseInt(e.Duration, 10, 64)
				d.task.TimeSpent = d.task.TimeSpent - oldDurationMs + newDurationMs
				d.timeEntries[i].Duration = fmt.Sprintf("%d", newDurationMs)
				d.timeEntries[i].Start = fmt.Sprintf("%d", newStart)
				d.timeEntries[i].End = fmt.Sprintf("%d", newEnd)
				break
			}
		}
		updated := d.task
		d.updatedTask = &updated

		return d, func() tea.Msg {
			req := &api.UpdateTimeEntryRequest{
				Start:    newStart,
				End:      newEnd,
				Duration: newDurationMs,
			}
			if err := client.UpdateTimeEntry(teamID, entryID, req); err != nil {
				return StatusMsg{Text: "update failed: " + err.Error()}
			}
			return StatusMsg{Text: "Time entry updated"}
		}
	}

	if res.Mode == ui.TimerModeLive {
		if res.Action == "start" {
			d.runningTimer = &api.RunningTimer{TaskID: taskID, Start: time.Now()}
			d.state.RunningTaskID = taskID
			return d, tea.Batch(
				func() tea.Msg {
					if err := client.StartTimer(teamID, taskID); err != nil {
						return StatusMsg{Text: "start timer failed: " + err.Error()}
					}
					return StatusMsg{Text: "Timer started"}
				},
				tea.Tick(time.Second, func(t time.Time) tea.Msg { return timerTickMsg{} }),
			)
		}
		elapsed := time.Since(d.runningTimer.Start).Milliseconds()
		d.runningTimer = nil
		d.state.RunningTaskID = ""
		d.task.TimeSpent += elapsed
		updated := d.task
		d.updatedTask = &updated
		return d, func() tea.Msg {
			if err := client.StopTimer(teamID); err != nil {
				return StatusMsg{Text: "stop timer failed: " + err.Error()}
			}
			return StatusMsg{Text: fmt.Sprintf("Timer stopped — logged %s", ui.FormatDuration(elapsed))}
		}
	}

	if res.Mode == ui.TimerModeDuration {
		ms := res.DurationMs
		d.task.TimeSpent += ms
		updated := d.task
		d.updatedTask = &updated
		return d, func() tea.Msg {
			now := time.Now()
			startMs := now.Add(-time.Duration(ms) * time.Millisecond).UnixMilli()
			endMs := now.UnixMilli()
			req := &api.CreateTimeEntryRequest{
				Start:    startMs,
				End:      endMs,
				Duration: ms,
				TaskID:   taskID,
			}
			if err := client.CreateTimeEntry(teamID, req); err != nil {
				return StatusMsg{Text: "time entry failed: " + err.Error()}
			}
			return StatusMsg{Text: fmt.Sprintf("Logged %s", ui.FormatDuration(ms))}
		}
	}

	// Time range mode
	start := res.Start
	end := res.End
	durationMs := end.Sub(start).Milliseconds()
	d.task.TimeSpent += durationMs
	updated := d.task
	d.updatedTask = &updated
	return d, func() tea.Msg {
		req := &api.CreateTimeEntryRequest{
			Start:    start.UnixMilli(),
			End:      end.UnixMilli(),
			Duration: durationMs,
			TaskID:   taskID,
		}
		if err := client.CreateTimeEntry(teamID, req); err != nil {
			return StatusMsg{Text: "time entry failed: " + err.Error()}
		}
		return StatusMsg{Text: fmt.Sprintf("Logged %s", ui.FormatDuration(durationMs))}
	}
}

// View implements tea.Model.
func (d Detail) View() string {
	if d.overlay != OverlayNone {
		return d.renderWithOverlay()
	}

	mainW := d.mainWidth()
	cmtW := d.width - mainW
	bodyH := d.height - 4

	mainPanel := d.renderMain(mainW, bodyH)
	cmtPanel := d.renderComments(cmtW, bodyH)

	body := lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, cmtPanel)

	var footerBindings []ui.KeyBinding
	if d.focus == FocusMain {
		timerLabel := "log time"
		if d.runningTimer != nil {
			timerLabel = "timer"
		}
		footerBindings = []ui.KeyBinding{
			{Key: "j/k", Label: "scroll"},
			{Key: "r", Label: "refresh"},
			{Key: "i", Label: "title"},
			{Key: "e", Label: "desc"},
			{Key: "E", Label: "desc ($EDITOR)"},
			{Key: "s", Label: "status"},
			{Key: "p", Label: "priority"},
			{Key: "a", Label: "assignees"},
			{Key: "t", Label: timerLabel},
			{Key: "T", Label: "estimate"},
			{Key: "D", Label: "due date"},
			{Key: "L", Label: "time log"},
			{Key: "c", Label: "comment"},
			{Key: "tab", Label: "comments"},
			{Key: "q", Label: "back"},
		}
	} else {
		footerBindings = []ui.KeyBinding{
			{Key: "j/k", Label: "navigate"},
			{Key: "e", Label: "edit comment"},
			{Key: "c", Label: "add comment"},
			{Key: "tab", Label: "main"},
			{Key: "q", Label: "back"},
		}
	}
	footer := ui.RenderFooter(footerBindings, d.width)

	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (d Detail) renderWithOverlay() string {
	// Build a dimmed background
	bg := d.renderMain(d.mainWidth(), d.height-4)

	var overlayContent string
	switch d.overlay {
	case OverlayStatus, OverlayPriority, OverlayType, OverlayAssignees:
		overlayContent = d.picker.View()
	case OverlayDescription:
		if d.multiEditor != nil {
			overlayContent = d.multiEditor.View()
		}
	case OverlayTitle, OverlayComment, OverlayEditComment, OverlayTimeEstimate:
		overlayContent = d.editor.View()
	case OverlayDueDate:
		overlayContent = d.dueDateEditor.View()
	case OverlayTimer:
		overlayContent = d.timer.View()
	case OverlayTimeEntries:
		overlayContent = d.timeEntriesList.View()
	}

	overlayWidth := 60
	if d.overlay == OverlayDescription {
		overlayWidth = max(60, d.width-10)
	}
	overlayStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorBlue).
		Background(ui.ColorCardBg).
		Padding(1, 2).
		Width(overlayWidth)

	overlayBox := overlayStyle.Render(overlayContent)

	// Center the overlay on top of the background
	bgLines := strings.Split(bg, "\n")
	bgH := len(bgLines)
	ovH := lipgloss.Height(overlayBox)
	ovW := lipgloss.Width(overlayBox)
	topPad := (bgH - ovH) / 2
	leftPad := (d.width - ovW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	if topPad < 0 {
		topPad = 0
	}

	padding := strings.Repeat("\n", topPad)
	leftStr := strings.Repeat(" ", leftPad)

	// Indent every line of the overlay box
	lines := strings.Split(overlayBox, "\n")
	for i, line := range lines {
		lines[i] = leftStr + line
	}

	return padding + strings.Join(lines, "\n")
}

func (d Detail) renderMain(width, height int) string {
	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ui.ColorBlue).
		Bold(true).
		Width(width - 4)
	sb.WriteString(titleStyle.Render(d.task.Name))
	sb.WriteString("\n\n")

	// Badges
	var badges []string
	statusColor := lipgloss.Color(d.task.Status.Color)
	if d.task.Status.Color == "" {
		statusColor = ui.ColorFgDim
	}
	statusBadge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1a2e")).
		Background(statusColor).
		Padding(0, 1).
		Render(d.task.Status.Status)
	badges = append(badges, statusBadge)

	if d.task.Priority != nil {
		pColor := lipgloss.Color(d.task.Priority.Color)
		if d.task.Priority.Color == "" {
			pColor = ui.ColorYellow
		}
		pb := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a2e")).
			Background(pColor).
			Padding(0, 1).
			Render(d.task.Priority.Priority)
		badges = append(badges, pb)
	}

	if d.task.CustomItem != nil {
		tb := lipgloss.NewStyle().
			Foreground(ui.ColorFg).
			Background(ui.ColorCardDim).
			Padding(0, 1).
			Render(d.task.CustomItem.Name)
		badges = append(badges, tb)
	}

	sb.WriteString(strings.Join(badges, " "))
	sb.WriteString("\n\n")

	// Assignees
	if len(d.task.Assignees) > 0 {
		var names []string
		for _, u := range d.task.Assignees {
			names = append(names, "@"+u.Username)
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(
			"Assignees: " + strings.Join(names, ", "),
		))
		sb.WriteString("\n")
	}

	// Time
	if d.task.TimeSpent > 0 {
		timeStr := "Time spent: " + ui.FormatDuration(d.task.TimeSpent)
		if len(d.timeEntries) > 0 {
			timeStr += fmt.Sprintf(" (%d entries)", len(d.timeEntries))
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render(timeStr))
		sb.WriteString("\n")
	}

	// Running timer
	if d.runningTimer != nil {
		elapsed := time.Since(d.runningTimer.Start).Milliseconds()
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true).Render(
			"Timer: " + ui.FormatDurationWithSeconds(elapsed) + " (running)",
		))
		sb.WriteString("\n")
	}

	// Time estimate
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim)
	valueStyle := lipgloss.NewStyle().Foreground(ui.ColorFg)
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

	// Description
	if d.task.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFgBright).Bold(true).Render("Description"))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFg).Width(width-4).Render(d.task.Description))
	}

	// Status message
	if d.statusMsg != "" {
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorYellow).Width(width-4).Render(d.statusMsg))
	}

	// Apply scroll offset and truncate to fit panel height.
	// lipgloss Height() only pads — it does not truncate overflow,
	// so we must manually window the content.
	content := sb.String()
	lines := strings.Split(content, "\n")
	// Border + padding consume ~2 lines, so visible area is height - 2
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	scroll := d.mainScroll
	maxScroll := len(lines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + visible
	if end > len(lines) {
		end = len(lines)
	}
	lines = lines[scroll:end]
	content = strings.Join(lines, "\n")

	borderStyle := ui.BorderStyle
	if d.focus == FocusMain {
		borderStyle = ui.ActiveBorderStyle
	}
	return borderStyle.Width(width-2).Height(height).Render(content)
}

func (d Detail) renderComments(width, height int) string {
	var sb strings.Builder

	header := lipgloss.NewStyle().Foreground(ui.ColorFgBright).Bold(true).Render("Comments")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if d.loadingCmts {
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("Loading…"))
	} else if len(d.comments) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorFgDim).Render("No comments."))
	} else {
		for i, cmt := range d.comments {
			selected := d.focus == FocusComments && i == d.cmtCursor
			isOwn := d.state.CurrentUser != nil && cmt.User.ID == d.state.CurrentUser.ID

			prefix := "  "
			if selected {
				prefix = "> "
			}

			userStyle := lipgloss.NewStyle().Foreground(ui.ColorBlue)
			if isOwn {
				userStyle = lipgloss.NewStyle().Foreground(ui.ColorGreen)
			}
			userLine := userStyle.Render(prefix + "@" + cmt.User.Username)

			textStyle := lipgloss.NewStyle().Foreground(ui.ColorFg)
			if selected {
				textStyle = textStyle.Bold(true)
			}
			textLine := "  " + textStyle.Width(width-6).Render(cmt.CommentText)

			sb.WriteString(userLine + "\n" + textLine + "\n\n")
		}
	}

	borderStyle := ui.BorderStyle
	if d.focus == FocusComments {
		borderStyle = ui.ActiveBorderStyle
	}
	return borderStyle.Width(width-2).Height(height).Render(sb.String())
}

func (d Detail) mainWidth() int {
	if d.width < 80 {
		return d.width
	}
	return d.width * 2 / 3
}

// lookupStatusType finds the type (e.g. "done", "closed", "active") for a status ID.
func (d Detail) lookupStatusType(statusID string) string {
	for _, list := range d.state.Lists {
		if list.ID == d.task.List.ID {
			for _, s := range list.Statuses {
				if s.ID == statusID {
					return s.Type
				}
			}
		}
	}
	for _, s := range d.state.Statuses {
		if s.ID == statusID {
			return s.Type
		}
	}
	return ""
}

// statusPickerItems returns picker items for the task's list statuses.
func (d Detail) statusPickerItems() []ui.PickerItem {
	// Use the task's list statuses if available
	for _, list := range d.state.Lists {
		if list.ID == d.task.List.ID {
			var items []ui.PickerItem
			for _, s := range list.Statuses {
				items = append(items, ui.PickerItem{ID: s.ID, Label: s.Status})
			}
			return items
		}
	}
	// Fall back to space-level statuses
	var items []ui.PickerItem
	for _, s := range d.state.Statuses {
		items = append(items, ui.PickerItem{ID: s.ID, Label: s.Status})
	}
	return items
}

// typePickerItems returns picker items for available task types.
func (d Detail) typePickerItems() []ui.PickerItem {
	var items []ui.PickerItem
	for _, t := range d.state.TaskTypes {
		items = append(items, ui.PickerItem{ID: strconv.Itoa(t.ID), Label: t.Name})
	}
	return items
}

// assigneePickerItems returns picker items for all workspace members, plus
// a slice of indices that are currently assigned to the task.
func (d Detail) assigneePickerItems() ([]ui.PickerItem, []int) {
	currentIDs := make(map[int]bool)
	for _, u := range d.task.Assignees {
		currentIDs[u.ID] = true
	}

	var items []ui.PickerItem
	var selected []int

	// Add current user first
	if d.state.CurrentUser != nil {
		items = append(items, ui.PickerItem{ID: strconv.Itoa(d.state.CurrentUser.ID), Label: d.state.CurrentUser.Username})
		if currentIDs[d.state.CurrentUser.ID] {
			selected = append(selected, 0)
		}
	}

	for _, m := range d.state.Members {
		u := m.User
		if d.state.CurrentUser != nil && u.ID == d.state.CurrentUser.ID {
			continue // already added first
		}
		idx := len(items)
		items = append(items, ui.PickerItem{ID: strconv.Itoa(u.ID), Label: u.Username})
		if currentIDs[u.ID] {
			selected = append(selected, idx)
		}
	}
	return items, selected
}
