package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hinkers/clickban/internal/api"
)

// TimeEntryAction is emitted when the user acts on a time entry.
type TimeEntryAction struct {
	Action string        // "edit", "delete", "close"
	Entry  api.TimeEntry // the selected entry (zero value for "close")
}

// TimeEntriesList is a scrollable list of time entries with edit/delete bindings.
type TimeEntriesList struct {
	entries []api.TimeEntry
	cursor  int
}

// NewTimeEntriesList creates a new TimeEntriesList from the given entries.
// Entries are displayed as-is (caller should sort newest-first before passing).
func NewTimeEntriesList(entries []api.TimeEntry) TimeEntriesList {
	return TimeEntriesList{entries: entries}
}

// Init implements tea.Model.
func (l TimeEntriesList) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (l TimeEntriesList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if l.cursor < len(l.entries)-1 {
				l.cursor++
			}
		case "k", "up":
			if l.cursor > 0 {
				l.cursor--
			}
		case "e":
			if len(l.entries) > 0 {
				entry := l.entries[l.cursor]
				return l, func() tea.Msg {
					return TimeEntryAction{Action: "edit", Entry: entry}
				}
			}
		case "d":
			if len(l.entries) > 0 {
				entry := l.entries[l.cursor]
				return l, func() tea.Msg {
					return TimeEntryAction{Action: "delete", Entry: entry}
				}
			}
		case "q", "esc":
			return l, func() tea.Msg {
				return TimeEntryAction{Action: "close"}
			}
		}
	}
	return l, nil
}

// RemoveEntry removes an entry by ID and adjusts the cursor.
func (l *TimeEntriesList) RemoveEntry(id string) {
	for i, e := range l.entries {
		if e.ID == id {
			l.entries = append(l.entries[:i], l.entries[i+1:]...)
			if l.cursor >= len(l.entries) && l.cursor > 0 {
				l.cursor--
			}
			return
		}
	}
}

// View implements tea.Model.
func (l TimeEntriesList) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render("Time Entries"))
	sb.WriteString("\n\n")

	if len(l.entries) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ColorFgDim)
		sb.WriteString(dimStyle.Render("No time entries"))
		sb.WriteString("\n\n")
	} else {
		for i, entry := range l.entries {
			line := formatTimeEntryRow(entry)
			if i == l.cursor {
				selectedStyle := lipgloss.NewStyle().
					Foreground(ColorBlue).
					Bold(true)
				sb.WriteString(selectedStyle.Render("> " + line))
			} else {
				normalStyle := lipgloss.NewStyle().Foreground(ColorFg)
				sb.WriteString(normalStyle.Render("  " + line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(ColorFgDim)
	sb.WriteString(hintStyle.Render("j/k: navigate  •  e: edit  •  d: delete  •  q/esc: close"))

	return sb.String()
}

// formatTimeEntryRow formats a single time entry as a display row.
func formatTimeEntryRow(entry api.TimeEntry) string {
	// Duration
	durationMs, _ := strconv.ParseInt(entry.Duration, 10, 64)
	durStr := FormatDuration(durationMs)

	// Time range or "manual"
	startMs, _ := strconv.ParseInt(entry.Start, 10, 64)
	endMs, _ := strconv.ParseInt(entry.End, 10, 64)
	startTime := time.UnixMilli(startMs)

	var rangeStr string
	if startMs == endMs || endMs == 0 {
		rangeStr = "manual"
	} else {
		endTime := time.UnixMilli(endMs)
		rangeStr = fmt.Sprintf("%s - %s",
			startTime.Format("3:04pm"),
			endTime.Format("3:04pm"),
		)
	}

	// Date
	dateStr := startTime.Format("Jan 2 (Mon)")

	// User
	userStr := "@" + entry.User.Username

	return fmt.Sprintf("%-8s %-22s %-14s %s", durStr, rangeStr, dateStr, userStr)
}
