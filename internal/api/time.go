package api

import "fmt"

func (c *Client) GetTimeEntries(taskID string) ([]TimeEntry, error) {
	var resp TimeEntriesResponse
	if err := c.Get(fmt.Sprintf("/task/%s/time", taskID), &resp); err != nil {
		return nil, fmt.Errorf("get time entries for task %s: %w", taskID, err)
	}
	return resp.Data, nil
}

func (c *Client) CreateTimeEntry(teamID string, req *CreateTimeEntryRequest) error {
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries", teamID), req, nil); err != nil {
		return fmt.Errorf("create time entry for task %s: %w", req.TaskID, err)
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
