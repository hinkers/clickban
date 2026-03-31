package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func (c *Client) GetTimeEntries(taskID string) ([]TimeEntry, error) {
	var resp TimeEntriesResponse
	if err := c.Get(fmt.Sprintf("/task/%s/time", taskID), &resp); err != nil {
		return nil, fmt.Errorf("get time entries for task %s: %w", taskID, err)
	}
	// Flatten per-user groups into a single list of entries
	var entries []TimeEntry
	for _, group := range resp.Data {
		for _, interval := range group.Intervals {
			interval.User = group.User
			entries = append(entries, interval)
		}
	}
	return entries, nil
}

func (c *Client) CreateTimeEntry(teamID string, req *CreateTimeEntryRequest) error {
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries", teamID), req, nil); err != nil {
		return fmt.Errorf("create time entry for task %s: %w", req.TaskID, err)
	}
	return nil
}

func (c *Client) DeleteTimeEntry(teamID, entryID string) error {
	if err := c.Delete(fmt.Sprintf("/team/%s/time_entries/%s", teamID, entryID), nil); err != nil {
		return fmt.Errorf("delete time entry %s: %w", entryID, err)
	}
	return nil
}

func (c *Client) UpdateTimeEntry(teamID, entryID string, req *UpdateTimeEntryRequest) error {
	if err := c.Put(fmt.Sprintf("/team/%s/time_entries/%s", teamID, entryID), req, nil); err != nil {
		return fmt.Errorf("update time entry %s: %w", entryID, err)
	}
	return nil
}

func (c *Client) StartTimer(teamID string, taskID string) error {
	body := struct {
		TID string `json:"tid"`
	}{TID: taskID}
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries/start", teamID), body, nil); err != nil {
		return fmt.Errorf("start timer for task %s: %w", taskID, err)
	}
	return nil
}

func (c *Client) StopTimer(teamID string) error {
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries/stop", teamID), nil, nil); err != nil {
		return fmt.Errorf("stop timer: %w", err)
	}
	return nil
}

func (c *Client) GetRunningTimer(teamID string) (*RunningTimer, error) {
	var raw struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.Get(fmt.Sprintf("/team/%s/time_entries/current", teamID), &raw); err != nil {
		return nil, fmt.Errorf("get running timer: %w", err)
	}

	// Try array first, then single object
	var entries []RunningTimerEntry
	if err := json.Unmarshal(raw.Data, &entries); err != nil {
		var single RunningTimerEntry
		if err2 := json.Unmarshal(raw.Data, &single); err2 != nil {
			return nil, nil // no running timer
		}
		entries = []RunningTimerEntry{single}
	}

	if len(entries) == 0 {
		return nil, nil
	}

	entry := entries[0]
	if entry.Task.ID == "" {
		return nil, nil
	}
	startMs, err := strconv.ParseInt(entry.Start, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse timer start time %q: %w", entry.Start, err)
	}
	return &RunningTimer{
		TaskID: entry.Task.ID,
		Start:  time.UnixMilli(startMs),
	}, nil
}
