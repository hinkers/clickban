package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// RenderWeeklySummary renders the weekly summary overlay content.
func RenderWeeklySummary(state AppState, width, height int) string {
	now := time.Now()
	// Last Monday to last Sunday
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	lastMonday := now.AddDate(0, 0, -(weekday - 1 + 7))
	lastMonday = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, now.Location())
	lastSunday := lastMonday.AddDate(0, 0, 6)
	lastSunday = time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 23, 59, 59, 0, now.Location())

	twoWeeksOut := now.AddDate(0, 0, 14)

	innerW := width - 6
	if innerW < 20 {
		innerW = 20
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(ui.ColorBlue).Bold(true)
	sb.WriteString(headerStyle.Render(fmt.Sprintf("📊 Weekly Summary — %s to %s",
		lastMonday.Format("Mon Jan 2"),
		lastSunday.Format("Mon Jan 2"))))
	sb.WriteString("\n\n")

	// Collect tasks assigned to current user
	var completed []api.Task
	var open []api.Task

	for _, task := range state.Tasks {
		assigned := false
		if state.CurrentUser != nil {
			for _, a := range task.Assignees {
				if a.ID == state.CurrentUser.ID {
					assigned = true
					break
				}
			}
		}
		if !assigned {
			continue
		}

		if isClosedStatus(task.Status) {
			// Check if completed last week
			if task.DateClosed != "" {
				if closed, ok := parseDueDate(task.DateClosed); ok {
					if !closed.Before(lastMonday) && !closed.After(lastSunday) {
						completed = append(completed, task)
					}
				}
			}
		} else {
			open = append(open, task)
		}
	}

	// Sort completed by date closed
	sort.SliceStable(completed, func(i, j int) bool {
		ci, _ := parseDueDate(completed[i].DateClosed)
		cj, _ := parseDueDate(completed[j].DateClosed)
		return ci.Before(cj)
	})

	// Sort open by status, then priority
	sort.SliceStable(open, func(i, j int) bool {
		si := strings.ToLower(open[i].Status.Status)
		sj := strings.ToLower(open[j].Status.Status)
		if si != sj {
			return si < sj
		}
		return priorityRank(open[i].Priority) < priorityRank(open[j].Priority)
	})

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorFgDim)
	nameStyle := lipgloss.NewStyle().Foreground(ui.ColorFg)
	warnStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)

	// Completed last week
	sb.WriteString(sectionStyle.Render(fmt.Sprintf("✅ Completed Last Week (%d)", len(completed))))
	sb.WriteString("\n")
	if len(completed) == 0 {
		sb.WriteString(dimStyle.Render("  No tasks completed last week."))
		sb.WriteString("\n")
	} else {
		for _, task := range completed {
			closedStr := ""
			if closed, ok := parseDueDate(task.DateClosed); ok {
				closedStr = closed.Format("Mon Jan 2")
			}
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				nameStyle.Render(task.Name),
				dimStyle.Render(closedStr)))
		}
	}
	sb.WriteString("\n")

	// Open tasks
	sb.WriteString(sectionStyle.Render(fmt.Sprintf("📋 Open Tasks (%d)", len(open))))
	sb.WriteString("\n")
	if len(open) == 0 {
		sb.WriteString(dimStyle.Render("  No open tasks."))
		sb.WriteString("\n")
	} else {
		lastStatus := ""
		for _, task := range open {
			status := task.Status.Status
			if strings.ToLower(status) != lastStatus {
				lastStatus = strings.ToLower(status)
				statusColor := lipgloss.Color(task.Status.Color)
				if task.Status.Color == "" {
					statusColor = ui.ColorFgDim
				}
				sb.WriteString("\n")
				sb.WriteString(lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render("  " + status))
				sb.WriteString("\n")
			}

			// Due date warning
			dueFlag := ""
			if due, ok := parseDueDate(task.DueDate); ok {
				if due.Before(now) {
					dueFlag = warnStyle.Render(fmt.Sprintf(" ⚠ overdue (%s)", due.Format("Jan 2")))
				} else if due.Before(twoWeeksOut) {
					dueFlag = warnStyle.Render(fmt.Sprintf(" ⏰ due %s", formatRelativeDate(due)))
				}
			}

			// Priority
			priLabel := ""
			if task.Priority != nil {
				label, color := priorityDisplay(task.Priority)
				priLabel = lipgloss.NewStyle().Foreground(color).Render(strings.TrimSpace(label)) + " "
			}

			sb.WriteString(fmt.Sprintf("    %s%s%s\n",
				priLabel,
				nameStyle.Render(task.Name),
				dueFlag))
		}
	}

	// Pad every line to full width to prevent terminal bleed-through
	raw := sb.String()
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lineW := lipgloss.Width(line)
		if lineW < innerW {
			lines[i] = line + strings.Repeat(" ", innerW-lineW)
		}
	}
	return strings.Join(lines, "\n")
}
