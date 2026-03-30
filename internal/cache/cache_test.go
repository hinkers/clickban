package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.MkdirAll(filepath.Join(dir, ".cache", "clickban"), 0o755)
	c, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSetAndGetTodayActions(t *testing.T) {
	c := testCache(t)
	today := time.Now().Format("2006-01-02")
	if err := c.SetTodayAction("task1", "forced", today); err != nil {
		t.Fatalf("SetTodayAction: %v", err)
	}
	if err := c.SetTodayAction("task2", "ignored", today); err != nil {
		t.Fatalf("SetTodayAction: %v", err)
	}
	actions, err := c.GetTodayActions(today)
	if err != nil {
		t.Fatalf("GetTodayActions: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions["task1"] != "forced" {
		t.Errorf("task1: expected forced, got %s", actions["task1"])
	}
	if actions["task2"] != "ignored" {
		t.Errorf("task2: expected ignored, got %s", actions["task2"])
	}
}

func TestClearExpiredTodayState(t *testing.T) {
	c := testCache(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	c.SetTodayAction("old_task", "forced", yesterday)
	c.SetTodayAction("new_task", "forced", today)
	if err := c.ClearExpiredTodayState(today); err != nil {
		t.Fatalf("ClearExpiredTodayState: %v", err)
	}
	actions, _ := c.GetTodayActions(today)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action after clear, got %d", len(actions))
	}
	if _, ok := actions["new_task"]; !ok {
		t.Error("new_task should survive clear")
	}
}

func TestGetLastSessionActions(t *testing.T) {
	c := testCache(t)
	friday := "2026-03-27"
	monday := "2026-03-30"
	c.SetTodayAction("task1", "forced", friday)
	c.SetTodayAction("task2", "ignored", friday)
	c.SetTodayAction("task3", "forced", "2026-03-26") // older, should not be returned

	actions, date, err := c.GetLastSessionActions(monday)
	if err != nil {
		t.Fatalf("GetLastSessionActions: %v", err)
	}
	if date != friday {
		t.Errorf("expected date %s, got %s", friday, date)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions["task1"] != "forced" {
		t.Errorf("task1: expected forced, got %s", actions["task1"])
	}
}

func TestGetLastSessionActionsNoHistory(t *testing.T) {
	c := testCache(t)
	actions, date, err := c.GetLastSessionActions("2026-03-30")
	if err != nil {
		t.Fatalf("GetLastSessionActions: %v", err)
	}
	if date != "" {
		t.Errorf("expected empty date, got %s", date)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
}

func TestIsTodayPlanned(t *testing.T) {
	c := testCache(t)
	today := time.Now().Format("2006-01-02")

	planned, err := c.IsTodayPlanned(today)
	if err != nil {
		t.Fatalf("IsTodayPlanned: %v", err)
	}
	if planned {
		t.Error("expected not planned initially")
	}

	c.SetTodayAction("_planned", "done", today)
	planned, err = c.IsTodayPlanned(today)
	if err != nil {
		t.Fatalf("IsTodayPlanned: %v", err)
	}
	if !planned {
		t.Error("expected planned after setting sentinel")
	}
}

func TestRemoveTodayAction(t *testing.T) {
	c := testCache(t)
	today := time.Now().Format("2006-01-02")
	c.SetTodayAction("task1", "forced", today)
	if err := c.RemoveTodayAction("task1"); err != nil {
		t.Fatalf("RemoveTodayAction: %v", err)
	}
	actions, _ := c.GetTodayActions(today)
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions after remove, got %d", len(actions))
	}
}
