package config

import (
	"fmt"
	"os"
)

const (
	DefaultTeamID  = "9016771227"
	DefaultSpaceID = "90165823077"
)

type Config struct {
	APIToken string
	TeamID   string
	SpaceID  string
}

func Load() (*Config, error) {
	token := os.Getenv("CLICKUP_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("CLICKUP_API_TOKEN environment variable is required")
	}

	teamID := os.Getenv("CLICKUP_TEAM_ID")
	if teamID == "" {
		teamID = DefaultTeamID
	}

	spaceID := os.Getenv("CLICKUP_SPACE_ID")
	if spaceID == "" {
		spaceID = DefaultSpaceID
	}

	return &Config{
		APIToken: token,
		TeamID:   teamID,
		SpaceID:  spaceID,
	}, nil
}
