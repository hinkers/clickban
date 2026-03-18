package api

import "fmt"

func (c *Client) GetTasks(listID string) ([]Task, error) {
	var resp TasksResponse
	if err := c.Get(fmt.Sprintf("/list/%s/task?subtasks=true", listID), &resp); err != nil {
		return nil, fmt.Errorf("get tasks for list %s: %w", listID, err)
	}
	return resp.Tasks, nil
}

func (c *Client) GetTaskWithSubtasks(taskID string) (*Task, error) {
	var task Task
	if err := c.Get(fmt.Sprintf("/task/%s?include_subtasks=true", taskID), &task); err != nil {
		return nil, fmt.Errorf("get task %s with subtasks: %w", taskID, err)
	}
	return &task, nil
}

func (c *Client) CreateTask(listID string, req *CreateTaskRequest) (*Task, error) {
	var task Task
	if err := c.Post(fmt.Sprintf("/list/%s/task", listID), req, &task); err != nil {
		return nil, fmt.Errorf("create task in list %s: %w", listID, err)
	}
	return &task, nil
}

func (c *Client) UpdateTask(taskID string, req *UpdateTaskRequest) error {
	var result Task
	if err := c.Put(fmt.Sprintf("/task/%s", taskID), req, &result); err != nil {
		return fmt.Errorf("update task %s: %w", taskID, err)
	}
	return nil
}

// ResolveLeafTasks walks the task tree and returns only leaf tasks (those with no children).
// It uses already-fetched subtask data. If depth limit is reached, the task at that depth
// is treated as a leaf regardless of whether it has children.
func ResolveLeafTasks(tasks []Task, maxDepth int) []Task {
	var leaves []Task
	for _, task := range tasks {
		leaves = append(leaves, resolveLeaves(task, 0, maxDepth)...)
	}
	return leaves
}

func resolveLeaves(task Task, depth, maxDepth int) []Task {
	if len(task.Subtasks) == 0 || depth >= maxDepth {
		return []Task{task}
	}
	var leaves []Task
	for _, sub := range task.Subtasks {
		leaves = append(leaves, resolveLeaves(sub, depth+1, maxDepth)...)
	}
	return leaves
}
