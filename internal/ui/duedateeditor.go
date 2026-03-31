package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DueDateEditor is a specialized editor for entering due dates with
// pre-filled year/month and day-of-week preview.
type DueDateEditor struct {
	input textinput.Model
}

// NewDueDateEditor creates a new DueDateEditor pre-filled with the current YYYY-MM-.
func NewDueDateEditor() DueDateEditor {
	now := time.Now()
	prefix := fmt.Sprintf("%04d-%02d-", now.Year(), now.Month())

	ti := textinput.New()
	ti.SetValue(prefix)
	ti.SetCursor(len(prefix))
	ti.Focus()
	ti.CharLimit = 10 // YYYY-MM-DD
	ti.Width = 50
	ti.Placeholder = "YYYY-MM-DD"

	return DueDateEditor{input: ti}
}

// Init implements tea.Model.
func (e DueDateEditor) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (e DueDateEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := e.input.Value()
			return e, func() tea.Msg {
				return EditorResult{Value: val, Cancelled: false}
			}
		case tea.KeyEsc:
			return e, func() tea.Msg {
				return EditorResult{Cancelled: true}
			}
		}
	}

	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)
	return e, cmd
}

// View implements tea.Model.
func (e DueDateEditor) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render("Due Date"))
	sb.WriteString("\n\n")
	sb.WriteString(e.input.View())
	sb.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(ColorFgDim)

	// Show day-of-week preview if the current value parses as a valid date.
	val := strings.TrimSpace(e.input.Value())
	if t, err := time.ParseInLocation("2006-01-02", val, time.Now().Location()); err == nil {
		dayStyle := lipgloss.NewStyle().Foreground(ColorBlue)
		sb.WriteString(dayStyle.Render(t.Format("Monday, January 2")))
		sb.WriteString("\n\n")
	}

	sb.WriteString(hintStyle.Render("Format: YYYY-MM-DD"))
	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("enter: confirm • esc: cancel"))

	return sb.String()
}
