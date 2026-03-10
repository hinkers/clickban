package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nhinkley/clickban/internal/api"
)

// RenderPreview renders the task detail preview pane.
func RenderPreview(task api.Task, width, height int) string {
	if width < 4 {
		width = 4
	}

	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true).
		Width(width - 4)
	sb.WriteString(titleStyle.Render(task.Name))
	sb.WriteString("\n\n")

	// Badges row: status, priority, custom item type
	var badges []string

	// Status badge
	statusColor := lipgloss.Color(task.Status.Color)
	if task.Status.Color == "" {
		statusColor = ColorFgDim
	}
	statusBadge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1a2e")).
		Background(statusColor).
		Padding(0, 1).
		Render(task.Status.Status)
	badges = append(badges, statusBadge)

	// Priority badge
	if task.Priority != nil && task.Priority.Priority != "" {
		priorityColor := lipgloss.Color(task.Priority.Color)
		if task.Priority.Color == "" {
			priorityColor = ColorYellow
		}
		priorityBadge := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a2e")).
			Background(priorityColor).
			Padding(0, 1).
			Render(task.Priority.Priority)
		badges = append(badges, priorityBadge)
	}

	// Custom item type badge
	if task.CustomItem != nil {
		typeBadge := lipgloss.NewStyle().
			Foreground(ColorFg).
			Background(ColorCardDim).
			Padding(0, 1).
			Render(task.CustomItem.Name)
		badges = append(badges, typeBadge)
	}

	if len(badges) > 0 {
		sb.WriteString(strings.Join(badges, " "))
		sb.WriteString("\n\n")
	}

	// Assignees
	if len(task.Assignees) > 0 {
		var usernames []string
		for _, u := range task.Assignees {
			usernames = append(usernames, "@"+u.Username)
		}
		assigneeLine := lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Render("Assignees: " + strings.Join(usernames, ", "))
		sb.WriteString(assigneeLine)
		sb.WriteString("\n")
	}

	// Time spent
	if task.TimeSpent > 0 {
		timeLine := lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Render("Time spent: " + FormatDuration(task.TimeSpent))
		sb.WriteString(timeLine)
		sb.WriteString("\n")
	}

	// Description header
	if task.Description != "" {
		sb.WriteString("\n")
		descHeader := lipgloss.NewStyle().
			Foreground(ColorFgBright).
			Bold(true).
			Render("Description")
		sb.WriteString(descHeader)
		sb.WriteString("\n")

		// Truncate description to fit available height
		// Estimate characters available: remaining lines * (width - 4)
		// Use a safe maximum of 1000 chars or remaining height * width chars
		maxChars := (height - 8) * (width - 4)
		if maxChars < 100 {
			maxChars = 100
		}
		if maxChars > 1000 {
			maxChars = 1000
		}

		desc := task.Description
		if len(desc) > maxChars {
			desc = desc[:maxChars-1] + "…"
		}

		descStyle := lipgloss.NewStyle().
			Foreground(ColorFg).
			Width(width - 4)
		sb.WriteString(descStyle.Render(desc))
	}

	// Wrap in a border
	panelStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 1).
		Width(width - 2)

	return panelStyle.Render(sb.String())
}
