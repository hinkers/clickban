package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
)

// FormatDuration converts milliseconds to a human-readable duration string.
// e.g. 9000000ms -> "2h30m", 2700000ms -> "45m"
func FormatDuration(ms int64) string {
	totalMinutes := ms / 60000
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// RenderCard renders a kanban card for a task.
func RenderCard(task api.Task, width int, selected bool) string {
	// Choose border color based on selection and priority
	borderColor := ColorBorder
	if selected {
		borderColor = ColorBorderAct
	}

	// Determine left accent color from priority
	accentColor := ColorFgDim
	if task.Priority != nil {
		switch strings.ToLower(task.Priority.Priority) {
		case "urgent":
			accentColor = ColorRed
		case "high":
			accentColor = ColorYellow
		case "normal":
			accentColor = ColorGreen
		case "low":
			accentColor = ColorFgDim
		}
	}

	// Inner width accounting for borders
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Task name line
	name := task.Name
	if len(name) > innerWidth {
		name = name[:innerWidth-1] + "…"
	}

	nameStyle := lipgloss.NewStyle().
		Foreground(ColorFg).
		Bold(true)
	if !selected {
		nameStyle = lipgloss.NewStyle().Foreground(ColorFg)
	}
	nameLine := nameStyle.Render(name)

	// Assignees line
	var assigneeParts []string
	for _, u := range task.Assignees {
		assigneeParts = append(assigneeParts, "@"+u.Username)
	}
	assigneeLine := ""
	if len(assigneeParts) > 0 {
		assigneeLine = lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Render(strings.Join(assigneeParts, " "))
	}

	// Time spent line
	timeLine := ""
	if task.TimeSpent > 0 {
		timeLine = lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Render("⏱ " + FormatDuration(task.TimeSpent))
	}

	// Accent bar on the left; use a different marker when selected
	accentChar := "▌"
	if selected {
		accentChar = "▍"
	}
	accentStyle := lipgloss.NewStyle().
		Foreground(accentColor).
		Render(accentChar)

	// Build content lines
	var lines []string
	lines = append(lines, accentStyle+" "+nameLine)
	if assigneeLine != "" {
		lines = append(lines, "  "+assigneeLine)
	}
	if timeLine != "" {
		lines = append(lines, "  "+timeLine)
	}

	content := strings.Join(lines, "\n")

	cardStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(ColorCardBg).
		Padding(0, 1).
		Width(width - 2)

	return cardStyle.Render(content)
}
