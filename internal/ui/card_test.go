package ui

import (
	"strings"
	"testing"

	"github.com/nhinkley/clickban/internal/api"
)

func TestRenderCard_ContainsTaskName(t *testing.T) {
	task := api.Task{
		Name:   "Fix login bug",
		Status: api.Status{Status: "in progress"},
	}
	result := RenderCard(task, 40, false)
	if !strings.Contains(result, "Fix login bug") {
		t.Errorf("expected card to contain task name, got:\n%s", result)
	}
}

func TestRenderCard_ContainsAssigneeUsername(t *testing.T) {
	task := api.Task{
		Name:   "Some task",
		Status: api.Status{Status: "open"},
		Assignees: []api.User{
			{ID: 1, Username: "nick"},
			{ID: 2, Username: "alice"},
		},
	}
	result := RenderCard(task, 40, false)
	if !strings.Contains(result, "nick") {
		t.Errorf("expected card to contain assignee 'nick', got:\n%s", result)
	}
}

func TestRenderCard_SelectedLooksDifferent(t *testing.T) {
	task := api.Task{
		Name:   "Task A",
		Status: api.Status{Status: "open"},
	}
	notSelected := RenderCard(task, 40, false)
	selected := RenderCard(task, 40, true)
	if notSelected == selected {
		t.Error("expected selected card to look different from unselected card")
	}
}

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	// 2h30m = 2*3600000 + 30*60000 = 7200000 + 1800000 = 9000000 ms
	result := FormatDuration(9000000)
	if result != "2h30m" {
		t.Errorf("expected '2h30m', got '%s'", result)
	}
}

func TestFormatDuration_HoursOnly(t *testing.T) {
	// 3h = 3*3600000 = 10800000 ms
	result := FormatDuration(10800000)
	if result != "3h0m" {
		t.Errorf("expected '3h0m', got '%s'", result)
	}
}

func TestFormatDuration_MinutesOnly(t *testing.T) {
	// 45m = 45*60000 = 2700000 ms
	result := FormatDuration(2700000)
	if result != "45m" {
		t.Errorf("expected '45m', got '%s'", result)
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	result := FormatDuration(0)
	if result != "0m" {
		t.Errorf("expected '0m', got '%s'", result)
	}
}

func TestRenderCard_ShowsTimeSpent(t *testing.T) {
	task := api.Task{
		Name:      "Task with time",
		Status:    api.Status{Status: "open"},
		TimeSpent: 9000000, // 2h30m
	}
	result := RenderCard(task, 40, false)
	if !strings.Contains(result, "2h30m") {
		t.Errorf("expected card to contain '2h30m', got:\n%s", result)
	}
}

func TestRenderCard_WithPriority(t *testing.T) {
	task := api.Task{
		Name:     "Urgent task",
		Status:   api.Status{Status: "open"},
		Priority: &api.Priority{Priority: "urgent"},
	}
	result := RenderCard(task, 40, false)
	// Just verify it renders without panic and contains the task name
	if !strings.Contains(result, "Urgent task") {
		t.Errorf("expected card to contain task name with priority set, got:\n%s", result)
	}
}
