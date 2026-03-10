package api

import "fmt"

func (c *Client) GetComments(taskID string) ([]Comment, error) {
	var resp CommentsResponse
	if err := c.Get(fmt.Sprintf("/task/%s/comment", taskID), &resp); err != nil {
		return nil, fmt.Errorf("get comments for task %s: %w", taskID, err)
	}
	return resp.Comments, nil
}

func (c *Client) CreateComment(taskID string, text string) error {
	req := CreateCommentRequest{CommentText: text}
	if err := c.Post(fmt.Sprintf("/task/%s/comment", taskID), req, nil); err != nil {
		return fmt.Errorf("create comment on task %s: %w", taskID, err)
	}
	return nil
}

func (c *Client) UpdateComment(commentID string, text string) error {
	req := UpdateCommentRequest{CommentText: text}
	if err := c.Put(fmt.Sprintf("/comment/%s", commentID), req, nil); err != nil {
		return fmt.Errorf("update comment %s: %w", commentID, err)
	}
	return nil
}
