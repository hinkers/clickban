package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/hinkers/clickban/internal/api"
)

func makeTask(id, name string, priority int, estimateHrs float64, dueDaysFromNow int) api.Task {
	t := api.Task{
		ID:           id,
		Name:         name,
		TimeEstimate: int64(estimateHrs * 3600000),
	}
	if priority > 0 {
		priNames := map[int]string{1: "urgent", 2: "high", 3: "normal", 4: "low"}
		t.Priority = &api.Priority{Priority: priNames[priority]}
	}
	due := time.Now().AddDate(0, 0, dueDaysFromNow)
	t.DueDate = fmt.Sprintf("%d", due.UnixMilli())
	t.Status = api.Status{Status: "open", Type: "open"}
	return t
}

func TestCalculate_PriorityOrder(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Low", 4, 1, -1),
		makeTask("2", "Urgent", 1, 2, 30),
		makeTask("3", "High", 2, 1.5, 30),
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) < 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result))
	}
	if result[0].ID != "1" {
		t.Errorf("expected overdue task first, got %s", result[0].ID)
	}
	if result[1].ID != "2" {
		t.Errorf("expected urgent second, got %s", result[1].ID)
	}
}

func TestCalculate_CapAt7Hours(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Big", 1, 4, 30),
		makeTask("2", "Medium", 2, 3, 30),
		makeTask("3", "Small", 3, 2, 30),
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) != 2 {
		t.Fatalf("expected 2 tasks (7h cap), got %d", len(result))
	}
}

func TestCalculate_DueTodayAlwaysIncluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Due today big", 4, 10, 0),
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) != 1 {
		t.Fatalf("due-today task must always be included")
	}
}

func TestCalculate_IgnoredExcluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Task A", 1, 2, 30),
		makeTask("2", "Task B", 2, 2, 30),
	}
	actions := map[string]string{"1": "ignored"}
	result := calculateTodayList(tasks, actions)
	if len(result) != 1 || result[0].ID != "2" {
		t.Errorf("ignored task should be excluded")
	}
}

func TestCalculate_ForcedAlwaysIncluded(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Forced low", 4, 1, 30),
		makeTask("2", "Urgent big", 1, 7, 30),
	}
	actions := map[string]string{"1": "forced"}
	result := calculateTodayList(tasks, actions)
	found := false
	for _, t := range result {
		if t.ID == "1" {
			found = true
		}
	}
	if !found {
		t.Error("forced task must be included")
	}
}

func TestCalculate_NoEstimateIncludedButNotCounted(t *testing.T) {
	tasks := []api.Task{
		makeTask("1", "Has est", 1, 7, 30),
		makeTask("2", "No est", 2, 0, 30),
	}
	result := calculateTodayList(tasks, map[string]string{})
	if len(result) != 2 {
		t.Fatalf("no-estimate task should be included, got %d tasks", len(result))
	}
}
