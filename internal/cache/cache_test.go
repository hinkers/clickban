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
