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

func (c *Client) GetAllLists(spaceID string) ([]List, error) {
	folders, err := c.GetFolders(spaceID)
	if err != nil {
		return nil, err
	}
	var allLists []List
	for _, folder := range folders {
		allLists = append(allLists, folder.Lists...)
	}
	folderless, err := c.GetFolderlessLists(spaceID)
	if err != nil {
		return nil, err
	}
	allLists = append(allLists, folderless...)
	return allLists, nil
}
