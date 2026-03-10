package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhinkley/clickban/internal/api"
)

func TestGetTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/list/l1/task" {
			t.Errorf("expected path /list/l1/task, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("subtasks") != "true" {
			t.Errorf("expected query param subtasks=true, got %q", r.URL.Query().Get("subtasks"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tasks": []map[string]interface{}{
				{"id": "t1", "name": "Task 1"},
				{"id": "t2", "name": "Task 2"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	tasks, err := client.GetTasks("l1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "t1" {
		t.Errorf("expected task ID 't1', got %q", tasks[0].ID)
	}
}

func TestGetTaskWithSubtasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1" {
			t.Errorf("expected path /task/t1, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("include_subtasks") != "true" {
			t.Errorf("expected query param include_subtasks=true, got %q", r.URL.Query().Get("include_subtasks"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "t1",
			"name": "Parent Task",
			"subtasks": []map[string]interface{}{
				{"id": "t1a", "name": "Sub 1"},
				{"id": "t1b", "name": "Sub 2"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	task, err := client.GetTaskWithSubtasks("t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID != "t1" {
		t.Errorf("expected task ID 't1', got %q", task.ID)
	}
	if len(task.Subtasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(task.Subtasks))
	}
}

func TestResolveLeafTasks_NoSubtasks(t *testing.T) {
	client := api.NewClient("pk_test")
	tasks := []api.Task{
		{ID: "t1", Name: "Task 1"},
	}
	leaves := client.ResolveLeafTasks(tasks, 5)
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	if leaves[0].ID != "t1" {
		t.Errorf("expected leaf ID 't1', got %q", leaves[0].ID)
	}
}

func TestResolveLeafTasks_WithSubtasks(t *testing.T) {
	client := api.NewClient("pk_test")
	tasks := []api.Task{
		{
			ID:   "parent",
			Name: "Parent",
			Subtasks: []api.Task{
				{ID: "child1", Name: "Child 1"},
				{ID: "child2", Name: "Child 2"},
			},
		},
	}
	leaves := client.ResolveLeafTasks(tasks, 5)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves, got %d", len(leaves))
	}
}

func TestResolveLeafTasks_DepthLimit(t *testing.T) {
	client := api.NewClient("pk_test")
	tasks := []api.Task{
		{
			ID:   "parent",
			Name: "Parent",
			Subtasks: []api.Task{
				{
					ID:   "child1",
					Name: "Child 1",
					Subtasks: []api.Task{
						{ID: "grandchild1", Name: "Grandchild 1"},
					},
				},
			},
		},
	}
	// depth limit 1: should stop at child1 and not go to grandchild1
	leaves := client.ResolveLeafTasks(tasks, 1)
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf (depth limit 1 stops at children), got %d", len(leaves))
	}
	if leaves[0].ID != "child1" {
		t.Errorf("expected leaf ID 'child1', got %q", leaves[0].ID)
	}
}

func TestUpdateTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1" {
			t.Errorf("expected path /task/t1, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "done" {
			t.Errorf("expected status 'done', got %v", body["status"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "t1",
			"name": "Task 1",
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	status := "done"
	req := &api.UpdateTaskRequest{Status: &status}
	err := client.UpdateTask("t1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
