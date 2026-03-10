package config_test

import (
	"os"
	"testing"

	"github.com/nhinkley/clickban/internal/config"
)

func TestLoad_RequiresAPIToken(t *testing.T) {
	os.Unsetenv("CLICKUP_API_TOKEN")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when CLICKUP_API_TOKEN is unset")
	}
}

func TestLoad_WithToken(t *testing.T) {
	t.Setenv("CLICKUP_API_TOKEN", "test-token")
	t.Setenv("CLICKUP_TEAM_ID", "")
	t.Setenv("CLICKUP_SPACE_ID", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIToken != "test-token" {
		t.Errorf("expected token 'test-token', got %q", cfg.APIToken)
	}
	if cfg.TeamID != "9016771227" {
		t.Errorf("expected default team ID '9016771227', got %q", cfg.TeamID)
	}
	if cfg.SpaceID != "90165823077" {
		t.Errorf("expected default space ID '90165823077', got %q", cfg.SpaceID)
	}
}

func TestLoad_CustomIDs(t *testing.T) {
	t.Setenv("CLICKUP_API_TOKEN", "test-token")
	t.Setenv("CLICKUP_TEAM_ID", "custom-team")
	t.Setenv("CLICKUP_SPACE_ID", "custom-space")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TeamID != "custom-team" {
		t.Errorf("expected team ID 'custom-team', got %q", cfg.TeamID)
	}
	if cfg.SpaceID != "custom-space" {
		t.Errorf("expected space ID 'custom-space', got %q", cfg.SpaceID)
	}
}
