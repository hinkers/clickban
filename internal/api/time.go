package api

import "fmt"

func (c *Client) GetTimeEntries(taskID string) ([]TimeEntry, error) {
	var resp TimeEntriesResponse
	if err := c.Get(fmt.Sprintf("/task/%s/time", taskID), &resp); err != nil {
		return nil, fmt.Errorf("get time entries for task %s: %w", taskID, err)
	}
	return resp.Data, nil
}

func (c *Client) CreateTimeEntry(taskID string, req *CreateTimeEntryRequest) error {
	var result map[string]interface{}
	if err := c.Post(fmt.Sprintf("/task/%s/time", taskID), req, &result); err != nil {
		return fmt.Errorf("create time entry for task %s: %w", taskID, err)
	}
	return nil
}

func (c *Client) StartTimer(teamID string, taskID string) error {
	body := map[string]string{"tid": taskID}
	var result map[string]interface{}
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries/start", teamID), body, &result); err != nil {
		return fmt.Errorf("start timer for task %s: %w", taskID, err)
	}
	return nil
}

func (c *Client) StopTimer(teamID string) error {
	var result map[string]interface{}
	if err := c.Post(fmt.Sprintf("/team/%s/time_entries/stop", teamID), nil, &result); err != nil {
		return fmt.Errorf("stop timer: %w", err)
	}
	return nil
}
