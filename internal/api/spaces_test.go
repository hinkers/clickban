package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hinkers/clickban/internal/api"
)

func TestGetSpace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/space/space1" {
			t.Errorf("expected path /space/space1, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "space1",
			"name": "Test Space",
			"statuses": []map[string]interface{}{
				{"status": "Open", "type": "open"},
				{"status": "In Progress", "type": "custom"},
				{"status": "Closed", "type": "closed"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	space, err := client.GetSpace("space1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if space.ID != "space1" {
		t.Errorf("expected space ID 'space1', got %q", space.ID)
	}
	if len(space.Statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(space.Statuses))
	}
}

func TestGetFolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/space/space1/folder" {
			t.Errorf("expected path /space/space1/folder, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"folders": []map[string]interface{}{
				{
					"id":   "folder1",
					"name": "Test Folder",
					"lists": []map[string]interface{}{
						{"id": "list1", "name": "List 1"},
						{"id": "list2", "name": "List 2"},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	folders, err := client.GetFolders("space1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if len(folders[0].Lists) != 2 {
		t.Errorf("expected 2 lists in folder, got %d", len(folders[0].Lists))
	}
}

func TestGetFolderlessLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/space/space1/list" {
			t.Errorf("expected path /space/space1/list, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lists": []map[string]interface{}{
				{"id": "list3", "name": "Folderless List"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	lists, err := client.GetFolderlessLists("space1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("expected 1 list, got %d", len(lists))
	}
	if lists[0].ID != "list3" {
		t.Errorf("expected list ID 'list3', got %q", lists[0].ID)
	}
}

func TestGetAllLists(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/space/space1/folder", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"folders": []map[string]interface{}{
				{
					"id":   "folder1",
					"name": "Test Folder",
					"lists": []map[string]interface{}{
						{"id": "list1", "name": "List 1"},
					},
				},
			},
		})
	})
	mux.HandleFunc("/space/space1/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lists": []map[string]interface{}{
				{"id": "list2", "name": "Folderless List"},
			},
		})
	})
	mux.HandleFunc("/list/list1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "list1", "name": "List 1",
			"statuses": []map[string]interface{}{
				{"id": "s1", "status": "open", "type": "open"},
			},
		})
	})
	mux.HandleFunc("/list/list2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "list2", "name": "Folderless List",
			"statuses": []map[string]interface{}{
				{"id": "s2", "status": "to do", "type": "open"},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := api.NewClient("pk_test", api.WithBaseURL(server.URL))
	lists, err := client.GetAllLists("space1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 2 {
		t.Errorf("expected 2 total lists, got %d", len(lists))
	}
}
