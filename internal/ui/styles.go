package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorBg        = lipgloss.Color("#1a1a2e")
	ColorFg        = lipgloss.Color("#c0caf5")
	ColorFgDim     = lipgloss.Color("#565f89")
	ColorFgBright  = lipgloss.Color("#e0e0e0")
	ColorBlue      = lipgloss.Color("#7aa2f7")
	ColorRed       = lipgloss.Color("#f7768e")
	ColorYellow    = lipgloss.Color("#e0af68")
	ColorGreen     = lipgloss.Color("#9ece6a")
	ColorPurple    = lipgloss.Color("#bb9af7")
	ColorBorder    = lipgloss.Color("#444444")
	ColorBorderAct = lipgloss.Color("#7aa2f7")
	ColorCardBg    = lipgloss.Color("#24283b")
	ColorCardDim   = lipgloss.Color("#1f2335")
)

func PriorityColor(priority int) lipgloss.Color {
	switch priority {
	case 1:
		return ColorRed
	case 2:
		return ColorYellow
	case 3:
		return ColorGreen
	default:
		return ColorFgDim
	}
}

var (
	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)

	ActiveBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderAct)

	TitleStyle = lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)

	DimStyle = lipgloss.NewStyle().
		Foreground(ColorFgDim)

	BadgeStyle = func(fg, bg lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().
			Foreground(fg).
			Background(bg).
			Padding(0, 1)
	}

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(ColorFgDim).
		Background(lipgloss.Color("#24283b")).
		Padding(0, 1)
)
