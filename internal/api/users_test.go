package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhinkley/clickban/internal/api"
)

func TestGetCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("expected path /user, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":       123,
				"username": "testuser",
				"email":    "test@example.com",
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	user, err := client.GetCurrentUser()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 123 {
		t.Errorf("expected user ID 123, got %d", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", user.Email)
	}
}

func TestGetWorkspaceMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/team1/member" {
			t.Errorf("expected path /team/team1/member, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"members": []map[string]interface{}{
				{"user": map[string]interface{}{"id": 1, "username": "alice"}},
				{"user": map[string]interface{}{"id": 2, "username": "bob"}},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	members, err := client.GetWorkspaceMembers("team1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].User.Username != "alice" {
		t.Errorf("expected first member 'alice', got %q", members[0].User.Username)
	}
	if members[1].User.Username != "bob" {
		t.Errorf("expected second member 'bob', got %q", members[1].User.Username)
	}
}

func TestGetTaskTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/team1/custom_item" {
			t.Errorf("expected path /team/team1/custom_item, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"custom_items": []map[string]interface{}{
				{"id": 1, "name": "Bug"},
				{"id": 2, "name": "Feature"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	items, err := client.GetTaskTypes("team1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 custom items, got %d", len(items))
	}
	if items[0].Name != "Bug" {
		t.Errorf("expected first item 'Bug', got %q", items[0].Name)
	}
	if items[1].Name != "Feature" {
		t.Errorf("expected second item 'Feature', got %q", items[1].Name)
	}
}
