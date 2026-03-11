package api

import "fmt"

func (c *Client) GetCurrentUser() (*User, error) {
	var resp AuthorizedUserResponse
	if err := c.Get("/user", &resp); err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	return &resp.User, nil
}

func (c *Client) GetWorkspaceMembers(teamID string) ([]Member, error) {
	var resp TeamsResponse
	if err := c.Get("/team", &resp); err != nil {
		return nil, fmt.Errorf("get workspace members: %w", err)
	}
	for _, team := range resp.Teams {
		if team.ID == teamID {
			return team.Members, nil
		}
	}
	return nil, fmt.Errorf("team %s not found", teamID)
}

func (c *Client) GetTaskTypes(teamID string) ([]CustomItem, error) {
	var resp CustomItemsResponse
	if err := c.Get(fmt.Sprintf("/team/%s/custom_item", teamID), &resp); err != nil {
		return nil, fmt.Errorf("get task types: %w", err)
	}
	return resp.CustomItems, nil
}
