package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KeyBinding represents a key and its associated label for display in the footer.
type KeyBinding struct {
	Key   string
	Label string
}

// RenderFooter renders a context-sensitive keybind bar at the bottom of the screen.
func RenderFooter(bindings []KeyBinding, width int) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1a2e")).
		Background(ColorBlue).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorFgDim)

	var parts []string
	for _, b := range bindings {
		key := keyStyle.Render(b.Key)
		label := labelStyle.Render(" " + b.Label)
		parts = append(parts, key+label)
	}

	content := strings.Join(parts, "  ")

	barStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#24283b")).
		Foreground(ColorFgDim).
		Width(width).
		Padding(0, 1)

	return barStyle.Render(content)
}

// RenderValidationWarning renders "⚠ N tasks need data" if count > 0.
// Returns empty string if count is 0.
func RenderValidationWarning(count int) string {
	if count <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(ColorYellow).
		Render(fmt.Sprintf("⚠ %d tasks need data", count))
}
