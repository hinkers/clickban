package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TimerMode controls which time entry mode is active.
type TimerMode int

const (
	TimerModeMenu      TimerMode = iota // sub-menu choosing mode
	TimerModeLive                       // start/stop timer
	TimerModeDuration                   // e.g. "2h30m"
	TimerModeTimeRange                  // e.g. "10:30am to 12:00pm"
)

// TimerResult is the message returned when the timer input is confirmed or cancelled.
type TimerResult struct {
	// For duration mode
	DurationMs int64
	// For time range mode
	Start time.Time
	End   time.Time
	// For live mode
	Action string // "start" or "stop"
	// Set true if user cancelled
	Cancelled bool
	// Which mode produced the result
	Mode TimerMode
}

// TimerInput is a Bubble Tea model for entering time durations or ranges.
type TimerInput struct {
	mode         TimerMode
	input        textinput.Model
	timerRunning bool
}

// NewTimerInput creates a new TimerInput model showing the mode menu.
func NewTimerInput() TimerInput {
	ti := textinput.New()
	ti.Placeholder = "e.g. 2h30m or 10:30am to 12:00pm"
	ti.CharLimit = 64

	return TimerInput{
		mode: TimerModeMenu,
	}
}

// NewTimerInputWithRunning creates a TimerInput that knows if a timer is running.
func NewTimerInputWithRunning(running bool) TimerInput {
	t := NewTimerInput()
	t.timerRunning = running
	return t
}

// Init implements tea.Model.
func (t TimerInput) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (t TimerInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch t.mode {
		case TimerModeMenu:
			switch msg.String() {
			case "1":
				action := "start"
				if t.timerRunning {
					action = "stop"
				}
				return t, func() tea.Msg {
					return TimerResult{Mode: TimerModeLive, Action: action}
				}
			case "2":
				t.mode = TimerModeDuration
				t.input.Placeholder = "e.g. 2h30m, 1h, 45m"
				t.input.Focus()
				return t, textinput.Blink
			case "3":
				t.mode = TimerModeTimeRange
				t.input.Placeholder = "e.g. 10:30am to 12:00pm"
				t.input.Focus()
				return t, textinput.Blink
			case "esc":
				return t, func() tea.Msg {
					return TimerResult{Cancelled: true}
				}
			}
			return t, nil

		default:
			// Duration or TimeRange input mode
			switch msg.Type {
			case tea.KeyEnter:
				raw := strings.TrimSpace(t.input.Value())
				if t.mode == TimerModeDuration {
					if ms, err := ParseDuration(raw); err == nil {
						return t, func() tea.Msg {
							return TimerResult{DurationMs: ms, Mode: TimerModeDuration}
						}
					}
				} else {
					start, end, err := ParseTimeRange(raw, time.Now())
					if err == nil {
						return t, func() tea.Msg {
							return TimerResult{Start: start, End: end, Mode: TimerModeTimeRange}
						}
					}
				}
				return t, nil
			case tea.KeyEsc:
				t.mode = TimerModeMenu
				t.input.SetValue("")
				return t, nil
			}
		}
	}

	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return t, cmd
}

// View implements tea.Model.
func (t TimerInput) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render("Log Time"))
	sb.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(ColorFgDim)

	switch t.mode {
	case TimerModeMenu:
		itemStyle := lipgloss.NewStyle().Foreground(ColorFg)
		timerLabel := "Start timer"
		if t.timerRunning {
			timerLabel = "Stop timer"
		}
		sb.WriteString(itemStyle.Render("1. " + timerLabel))
		sb.WriteString("\n")
		sb.WriteString(itemStyle.Render("2. Manual duration"))
		sb.WriteString("\n")
		sb.WriteString(itemStyle.Render("3. Time range"))
		sb.WriteString("\n\n")
		sb.WriteString(hintStyle.Render("1/2/3: select  •  esc: cancel"))

	case TimerModeDuration:
		sb.WriteString(t.input.View())
		sb.WriteString("\n\n")
		sb.WriteString(hintStyle.Render("Format: 2h30m, 3h, 45m"))
		sb.WriteString("\n")
		sb.WriteString(hintStyle.Render("enter: confirm  •  esc: back"))

	case TimerModeTimeRange:
		sb.WriteString(t.input.View())
		sb.WriteString("\n\n")
		sb.WriteString(hintStyle.Render("Format: 10:30am to 12:00pm  •  10:30am to now"))
		sb.WriteString("\n")
		sb.WriteString(hintStyle.Render("enter: confirm  •  esc: back"))
	}

	return sb.String()
}

// durationRegexp matches patterns like "2h30m", "3h", "45m", "0h30m"
var durationRegexp = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?$`)

// ParseDuration parses a duration string like "2h30m", "3h", or "45m" into milliseconds.
func ParseDuration(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	matches := durationRegexp.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format %q: expected e.g. 2h30m, 3h, 45m", s)
	}

	// At least one group must be non-empty
	if matches[1] == "" && matches[2] == "" {
		return 0, fmt.Errorf("invalid duration format %q: no hours or minutes found", s)
	}

	var hours, minutes int64
	if matches[1] != "" {
		h, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid hours %q: %w", matches[1], err)
		}
		hours = h
	}
	if matches[2] != "" {
		m, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid minutes %q: %w", matches[2], err)
		}
		minutes = m
	}

	return (hours*3600 + minutes*60) * 1000, nil
}

// timeRegexp matches times like "10:30am", "12:00pm", "9am"
var timeRegexp = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?(am|pm)$`)

// parseTimeOfDay parses a time-of-day string relative to a reference date.
func parseTimeOfDay(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	matches := timeRegexp.FindStringSubmatch(s)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid time format %q: expected e.g. 10:30am, 2pm", s)
	}

	hour, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour %q", matches[1])
	}

	minute := 0
	if matches[2] != "" {
		minute, err = strconv.Atoi(matches[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid minute %q", matches[2])
		}
	}

	ampm := strings.ToLower(matches[3])
	if ampm == "pm" && hour != 12 {
		hour += 12
	} else if ampm == "am" && hour == 12 {
		hour = 0
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("time out of range: %02d:%02d", hour, minute)
	}

	return time.Date(ref.Year(), ref.Month(), ref.Day(), hour, minute, 0, 0, ref.Location()), nil
}

// ParseTimeRange parses a time range string like "10:30am to 12:00pm" or "10:30am to now"
// relative to ref. Returns (start, end, error).
func ParseTimeRange(s string, ref time.Time) (time.Time, time.Time, error) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)

	idx := strings.Index(lower, " to ")
	if idx < 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid time range %q: missing ' to ' separator", s)
	}

	startStr := strings.TrimSpace(s[:idx])
	endStr := strings.TrimSpace(s[idx+4:])

	start, err := parseTimeOfDay(startStr, ref)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	var end time.Time
	if strings.ToLower(endStr) == "now" {
		end = ref
	} else {
		end, err = parseTimeOfDay(endStr, ref)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end time: %w", err)
		}
	}

	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end time %v is before start time %v", end, start)
	}

	return start, end, nil
}
