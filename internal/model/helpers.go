package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/ui"
)

// isClosedStatus returns true if the status type indicates a closed/done task.
func isClosedStatus(s api.Status) bool {
	t := strings.ToLower(s.Type)
	return t == "closed" || t == "done"
}

// priorityRank returns a numeric rank for sorting (lower = higher priority).
// 1=urgent, 2=high, 3=normal, 4=low, 99=none.
func priorityRank(p *api.Priority) int {
	if p == nil {
		return 99
	}
	switch strings.ToLower(p.Priority) {
	case "urgent":
		return 1
	case "high":
		return 2
	case "normal":
		return 3
	case "low":
		return 4
	default:
		return 99
	}
}

// priorityDisplay returns (label, color) for rendering a task's priority.
func priorityDisplay(p *api.Priority) (string, lipgloss.Color) {
	if p == nil {
		return "–", ui.ColorFgDim
	}
	switch p.Priority {
	case "urgent":
		return "!! Urgent", ui.ColorRed
	case "high":
		return "! High", ui.ColorYellow
	case "normal":
		return "Normal", ui.ColorGreen
	case "low":
		return "Low", ui.ColorFgDim
	default:
		return p.Priority, ui.ColorFgDim
	}
}

// parseDueDate parses a ClickUp millisecond timestamp string into a time.Time.
// Returns zero time and false if the string is empty or unparseable.
func parseDueDate(ts string) (time.Time, bool) {
	if ts == "" {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms), true
}

// isDueOrOverdue returns true if the task is due today or before today.
func isDueOrOverdue(t api.Task) bool {
	due, ok := parseDueDate(t.DueDate)
	if !ok {
		return false
	}
	now := time.Now()
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return !due.After(endOfToday)
}

// formatRelativeDate formats a due date relative to today.
// Returns "today", "tomorrow", weekday name (if within 7 days), or "Jan 2" format.
func formatRelativeDate(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	days := int(dueDay.Sub(today).Hours() / 24)
	switch {
	case days == -1:
		return "yesterday"
	case days < -1:
		return fmt.Sprintf("%d days ago", -days)
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days < 7:
		return t.Format("Mon")
	default:
		return t.Format("Jan 2")
	}
}

// remainingTimeMs returns the remaining time estimate in milliseconds.
// Returns 0 if time_spent >= time_estimate or if time_estimate is 0.
func remainingTimeMs(t api.Task) int64 {
	if t.TimeEstimate <= 0 {
		return 0
	}
	remaining := t.TimeEstimate - t.TimeSpent
	if remaining < 0 {
		return 0
	}
	return remaining
}

// taskNeedsData returns true if a task is missing priority or time estimate.
func taskNeedsData(t api.Task) bool {
	return t.Priority == nil || t.TimeEstimate <= 0
}
