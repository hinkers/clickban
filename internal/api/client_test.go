package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hinkers/clickban/internal/api"
)

func TestClient_Get_SetsAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "pk_test123" {
			t.Errorf("expected auth header 'pk_test123', got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := api.NewClient("pk_test123", api.WithBaseURL(server.URL))
	var result map[string]string
	err := client.Get("/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", result["status"])
	}
}

func TestClient_Get_ReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_025"}`))
	}))
	defer server.Close()

	client := api.NewClient("bad-token", api.WithBaseURL(server.URL))
	var result map[string]string
	err := client.Get("/test", &result)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestClient_Put_SendsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "updated" {
			t.Errorf("expected name 'updated', got %q", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := api.NewClient("pk_test123", api.WithBaseURL(server.URL))
	body := map[string]string{"name": "updated"}
	var result map[string]string
	err := client.Put("/test", body, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Post_SendsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "new-id"})
	}))
	defer server.Close()

	client := api.NewClient("pk_test123", api.WithBaseURL(server.URL))
	body := map[string]string{"text": "hello"}
	var result map[string]string
	err := client.Post("/test", body, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "new-id" {
		t.Errorf("expected id 'new-id', got %q", result["id"])
	}
}
