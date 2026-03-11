package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hinkers/clickban/internal/api"
)

func TestGetTimeEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1/time" {
			t.Errorf("expected path /task/t1/time, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "te1", "duration": "3600000"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	entries, err := client.GetTimeEntries("t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 time entry, got %d", len(entries))
	}
	if entries[0].ID != "te1" {
		t.Errorf("expected entry ID 'te1', got %q", entries[0].ID)
	}
}

func TestCreateTimeEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task/t1/time" {
			t.Errorf("expected path /task/t1/time, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["duration"] == nil {
			t.Error("expected duration in request body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"id": "te2"}})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	req := &api.CreateTimeEntryRequest{
		Start:    1000000,
		Duration: 3600000,
	}
	err := client.CreateTimeEntry("t1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartTimer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/team1/time_entries/start" {
			t.Errorf("expected path /team/team1/time_entries/start, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["tid"] != "t1" {
			t.Errorf("expected tid 't1', got %v", body["tid"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"id": "te3"}})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	err := client.StartTimer("team1", "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopTimer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/team1/time_entries/stop" {
			t.Errorf("expected path /team/team1/time_entries/stop, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"id": "te3"}})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	err := client.StopTimer("team1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
