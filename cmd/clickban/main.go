package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinkers/clickban/internal/api"
	"github.com/hinkers/clickban/internal/config"
	"github.com/hinkers/clickban/internal/model"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	client := api.NewClient(cfg.APIToken)

	app := model.NewApp(client, cfg.TeamID, cfg.SpaceID)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
		os.Exit(1)
	}
}
