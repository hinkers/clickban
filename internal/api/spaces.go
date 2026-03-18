package api

import "fmt"

func (c *Client) GetSpace(spaceID string) (*Space, error) {
	var space Space
	if err := c.Get(fmt.Sprintf("/space/%s", spaceID), &space); err != nil {
		return nil, fmt.Errorf("get space: %w", err)
	}
	return &space, nil
}

func (c *Client) GetFolders(spaceID string) ([]Folder, error) {
	var resp FoldersResponse
	if err := c.Get(fmt.Sprintf("/space/%s/folder", spaceID), &resp); err != nil {
		return nil, fmt.Errorf("get folders: %w", err)
	}
	return resp.Folders, nil
}

func (c *Client) GetFolderlessLists(spaceID string) ([]List, error) {
	var resp ListsResponse
	if err := c.Get(fmt.Sprintf("/space/%s/list", spaceID), &resp); err != nil {
		return nil, fmt.Errorf("get folderless lists: %w", err)
	}
	return resp.Lists, nil
}

func (c *Client) GetList(listID string) (*List, error) {
	var list List
	if err := c.Get(fmt.Sprintf("/list/%s", listID), &list); err != nil {
		return nil, fmt.Errorf("get list %s: %w", listID, err)
	}
	return &list, nil
}

func (c *Client) GetAllLists(spaceID string) ([]List, error) {
	folders, err := c.GetFolders(spaceID)
	if err != nil {
		return nil, err
	}
	// Collect list IDs from folders
	var listIDs []string
	for _, folder := range folders {
		for _, l := range folder.Lists {
			listIDs = append(listIDs, l.ID)
		}
	}
	folderless, err := c.GetFolderlessLists(spaceID)
	if err != nil {
		return nil, err
	}
	for _, l := range folderless {
		listIDs = append(listIDs, l.ID)
	}

	// Fetch each list individually to get statuses
	var allLists []List
	for _, id := range listIDs {
		list, err := c.GetList(id)
		if err != nil {
			return nil, err
		}
		allLists = append(allLists, *list)
	}
	return allLists, nil
}
