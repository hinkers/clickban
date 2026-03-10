package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nhinkley/clickban/internal/api"
)

func TestGetComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1/comment" {
			t.Errorf("expected path /task/t1/comment, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"comments": []map[string]interface{}{
				{"id": "c1", "comment_text": "Hello"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	comments, err := client.GetComments("t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].ID != "c1" {
		t.Errorf("expected comment ID 'c1', got %q", comments[0].ID)
	}
}

func TestCreateComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1/comment" {
			t.Errorf("expected path /task/t1/comment, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["comment_text"] != "New comment" {
			t.Errorf("expected comment_text 'New comment', got %v", body["comment_text"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "c2"})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	err := client.CreateComment("t1", "New comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comment/c1" {
			t.Errorf("expected path /comment/c1, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["comment_text"] != "Updated comment" {
			t.Errorf("expected comment_text 'Updated comment', got %v", body["comment_text"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "c1"})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	err := client.UpdateComment("c1", "Updated comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
