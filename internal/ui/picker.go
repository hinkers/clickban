package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PickerItem represents an item in the picker list.
type PickerItem struct {
	ID    string
	Label string
}

// PickerResult is the message returned when the picker is confirmed or cancelled.
type PickerResult struct {
	Selected  []PickerItem
	Cancelled bool
}

// Picker is a Bubble Tea model for single or multi-select dropdowns.
type Picker struct {
	title    string
	items    []PickerItem
	multi    bool
	cursor   int
	selected map[int]bool
}

// NewPicker creates a new Picker model.
func NewPicker(title string, items []PickerItem, multi bool) Picker {
	return Picker{
		title:    title,
		items:    items,
		multi:    multi,
		cursor:   0,
		selected: make(map[int]bool),
	}
}

// CursorIndex returns the current cursor position.
func (p Picker) CursorIndex() int {
	return p.cursor
}

// IsSelected returns whether the item at the given index is selected.
func (p Picker) IsSelected(index int) bool {
	return p.selected[index]
}

// SetSelected sets the selection state of the item at the given index.
func (p *Picker) SetSelected(index int, val bool) {
	if p.selected == nil {
		p.selected = make(map[int]bool)
	}
	p.selected[index] = val
}

// Init implements tea.Model.
func (p Picker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if len(p.items) > 0 {
				p.cursor = (p.cursor + 1) % len(p.items)
			}

		case "k", "up":
			if len(p.items) > 0 {
				p.cursor = (p.cursor - 1 + len(p.items)) % len(p.items)
			}

		case " ":
			if p.multi {
				p.selected[p.cursor] = !p.selected[p.cursor]
			}

		case "enter":
			var picked []PickerItem
			if p.multi {
				for i, item := range p.items {
					if p.selected[i] {
						picked = append(picked, item)
					}
				}
			} else {
				if len(p.items) > 0 {
					picked = []PickerItem{p.items[p.cursor]}
				}
			}
			return p, func() tea.Msg {
				return PickerResult{Selected: picked, Cancelled: false}
			}

		case "esc":
			return p, func() tea.Msg {
				return PickerResult{Cancelled: true}
			}
		}
	}
	return p, nil
}

// View implements tea.Model.
func (p Picker) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)
	sb.WriteString(titleStyle.Render(p.title))
	sb.WriteString("\n\n")

	for i, item := range p.items {
		cursor := "  "
		if i == p.cursor {
			cursor = "> "
		}

		check := "[ ]"
		if p.multi {
			if p.selected[i] {
				check = "[x]"
			}
		} else {
			check = ""
		}

		itemStyle := lipgloss.NewStyle().Foreground(ColorFg)
		if i == p.cursor {
			itemStyle = lipgloss.NewStyle().Foreground(ColorBlue).Bold(true)
		}

		line := cursor
		if check != "" {
			line += check + " "
		}
		line += item.Label

		sb.WriteString(itemStyle.Render(line))
		sb.WriteString("\n")
	}

	return sb.String()
}
