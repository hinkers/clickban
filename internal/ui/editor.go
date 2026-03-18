package ui

import (
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditorResult is the message returned when the editor is confirmed or cancelled.
type EditorResult struct {
	Value     string
	Cancelled bool
}

// ExternalEditorResult is the result of opening an external editor.
type ExternalEditorResult struct {
	Content string
	Err     error
}

// Editor is a Bubble Tea model for inline text editing.
type Editor struct {
	title string
	input textinput.Model
}

// NewEditor creates a new Editor model with the given title and initial content.
func NewEditor(title, initial string) Editor {
	ti := textinput.New()
	ti.SetValue(initial)
	ti.Focus()
	ti.CharLimit = 0 // unlimited
	ti.Width = 50
	ti.Placeholder = "Type here…"

	return Editor{
		title: title,
		input: ti,
	}
}

// Value returns the current value in the text input.
func (e Editor) Value() string {
	return e.input.Value()
}

// Init implements tea.Model.
func (e Editor) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (e Editor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (e Editor) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render(e.title))
	sb.WriteString("\n\n")
	sb.WriteString(e.input.View())
	sb.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(ColorFgDim)
	sb.WriteString(hintStyle.Render("enter: confirm • esc: cancel"))

	return sb.String()
}

// MultiLineEditor is a Bubble Tea model for multiline text editing.
type MultiLineEditor struct {
	title string
	input textarea.Model
}

// NewMultiLineEditor creates a new multiline editor.
func NewMultiLineEditor(title, initial string, width, height int) MultiLineEditor {
	ta := textarea.New()
	ta.SetValue(initial)
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(width - 6)
	ta.SetHeight(height - 8)
	ta.ShowLineNumbers = false

	return MultiLineEditor{
		title: title,
		input: ta,
	}
}

// Init implements tea.Model.
func (e MultiLineEditor) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (e MultiLineEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			return e, func() tea.Msg {
				return EditorResult{Cancelled: true}
			}
		case msg.Type == tea.KeyCtrlS:
			val := e.input.Value()
			return e, func() tea.Msg {
				return EditorResult{Value: val, Cancelled: false}
			}
		}
	}

	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)
	return e, cmd
}

// View implements tea.Model.
func (e MultiLineEditor) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render(e.title))
	sb.WriteString("\n\n")
	sb.WriteString(e.input.View())
	sb.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(ColorFgDim)
	sb.WriteString(hintStyle.Render("ctrl+s: save • esc: cancel"))

	return sb.String()
}

// OpenExternalEditor opens $EDITOR with the given content in a temp file
// and returns a Cmd that yields an ExternalEditorResult when done.
func OpenExternalEditor(content string) tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		// Create temp file
		f, err := os.CreateTemp("", "clickban-*.md")
		if err != nil {
			return ExternalEditorResult{Err: err}
		}
		defer os.Remove(f.Name())

		if _, err := f.WriteString(content); err != nil {
			f.Close()
			return ExternalEditorResult{Err: err}
		}
		f.Close()

		// Open editor
		cmd := exec.Command(editor, f.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return ExternalEditorResult{Err: err}
		}

		// Read back the edited content
		data, err := os.ReadFile(f.Name())
		if err != nil {
			return ExternalEditorResult{Err: err}
		}

		return ExternalEditorResult{Content: string(data)}
	}
}
