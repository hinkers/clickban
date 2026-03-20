package ui

import (
	"strings"
	"testing"

	"github.com/hinkers/clickban/internal/api"
)

func TestRenderPreview_ContainsTitle(t *testing.T) {
	task := api.Task{
		Name:   "Implement OAuth",
		Status: api.Status{Status: "in progress"},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if !strings.Contains(result, "Implement OAuth") {
		t.Errorf("expected preview to contain task title, got:\n%s", result)
	}
}

func TestRenderPreview_ContainsDescription(t *testing.T) {
	task := api.Task{
		Name:        "Task with description",
		Description: "This is the task description.",
		Status:      api.Status{Status: "open"},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if !strings.Contains(result, "This is the task description.") {
		t.Errorf("expected preview to contain description, got:\n%s", result)
	}
}

func TestRenderPreview_TruncatesLongDescription(t *testing.T) {
	longDesc := strings.Repeat("A", 2000)
	task := api.Task{
		Name:        "Task",
		Description: longDesc,
		Status:      api.Status{Status: "open"},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if len(result) >= len(longDesc) {
		t.Error("expected preview to truncate long description")
	}
}

func TestRenderPreview_ContainsStatus(t *testing.T) {
	task := api.Task{
		Name:   "Status task",
		Status: api.Status{Status: "in progress"},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if !strings.Contains(result, "in progress") {
		t.Errorf("expected preview to contain status 'in progress', got:\n%s", result)
	}
}

func TestRenderPreview_ContainsPriority(t *testing.T) {
	task := api.Task{
		Name:     "Priority task",
		Status:   api.Status{Status: "open"},
		Priority: &api.Priority{Priority: "high"},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if !strings.Contains(result, "high") {
		t.Errorf("expected preview to contain priority 'high', got:\n%s", result)
	}
}

func TestRenderPreview_ContainsListName(t *testing.T) {
	task := api.Task{
		Name:   "Listed task",
		Status: api.Status{Status: "open"},
	}
	result := RenderPreview(task, 60, 20, "Sprint 1", "")
	if !strings.Contains(result, "Sprint 1") {
		t.Errorf("expected preview to contain list name 'Sprint 1', got:\n%s", result)
	}
}

func TestRenderPreview_ContainsAssignees(t *testing.T) {
	task := api.Task{
		Name:   "Assigned task",
		Status: api.Status{Status: "open"},
		Assignees: []api.User{
			{ID: 1, Username: "bob"},
		},
	}
	result := RenderPreview(task, 60, 20, "", "")
	if !strings.Contains(result, "bob") {
		t.Errorf("expected preview to contain assignee 'bob', got:\n%s", result)
	}
}
