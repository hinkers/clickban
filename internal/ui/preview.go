package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinkers/clickban/internal/api"
)

// formatTimestamp formats a ClickUp millisecond timestamp string to a readable date.
func formatTimestamp(ms string) string {
	if ms == "" {
		return ""
	}
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return ""
	}
	t := time.UnixMilli(n)
	return t.Format("2 Jan 2006 3:04pm")
}

// RenderPreview renders the task detail preview pane.
func RenderPreview(task api.Task, width, height int, listName string) string {
	if width < 4 {
		width = 4
	}
	innerW := width - 4

	var sb strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(ColorFgDim)
	valueStyle := lipgloss.NewStyle().Foreground(ColorFg).Width(innerW)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true).
		Width(innerW)
	sb.WriteString(titleStyle.Render(task.Name))
	sb.WriteString("\n\n")

	// Badges row: status, priority
	var badges []string
	statusColor := lipgloss.Color(task.Status.Color)
	if task.Status.Color == "" {
		statusColor = ColorFgDim
	}
	badges = append(badges, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1a2e")).
		Background(statusColor).
		Padding(0, 1).
		Render(task.Status.Status))

	if task.Priority != nil && task.Priority.Priority != "" {
		pColor := lipgloss.Color(task.Priority.Color)
		if task.Priority.Color == "" {
			pColor = ColorYellow
		}
		badges = append(badges, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a2e")).
			Background(pColor).
			Padding(0, 1).
			Render(task.Priority.Priority))
	}
	sb.WriteString(strings.Join(badges, " "))
	sb.WriteString("\n\n")

	// Fields
	if listName != "" {
		sb.WriteString(labelStyle.Render("List: ") + valueStyle.Render(listName) + "\n")
	}
	if len(task.Assignees) > 0 {
		var names []string
		for _, u := range task.Assignees {
			names = append(names, "@"+u.Username)
		}
		sb.WriteString(labelStyle.Render("Assignees: ") + valueStyle.Render(strings.Join(names, ", ")) + "\n")
	}
	if task.TimeSpent > 0 {
		sb.WriteString(labelStyle.Render("Time: ") + valueStyle.Render(FormatDuration(task.TimeSpent)) + "\n")
	}
	if ts := formatTimestamp(task.DateCreated); ts != "" {
		sb.WriteString(labelStyle.Render("Created: ") + valueStyle.Render(ts) + "\n")
	}
	if ts := formatTimestamp(task.DateUpdated); ts != "" {
		sb.WriteString(labelStyle.Render("Updated: ") + valueStyle.Render(ts) + "\n")
	}

	// Description (truncated)
	if task.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorFgBright).Bold(true).Render("Description"))
		sb.WriteString("\n")
		maxChars := (height - 14) * innerW
		if maxChars < 100 {
			maxChars = 100
		}
		if maxChars > 800 {
			maxChars = 800
		}
		desc := task.Description
		if len(desc) > maxChars {
			desc = desc[:maxChars-1] + "…"
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorFg).Width(innerW).Render(desc))
	}

	// Wrap in a border
	panelStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 1).
		Width(width - 2)

	return panelStyle.Render(sb.String())
}
