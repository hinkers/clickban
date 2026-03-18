package api

// User represents a ClickUp user.
type User struct {
	ID             int    `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Color          string `json:"color"`
	ProfilePicture string `json:"profilePicture"`
	Initials       string `json:"initials"`
	Role           int    `json:"role"`
}

// Member represents a workspace member.
type Member struct {
	User User `json:"user"`
}

// Status represents a task status within a list.
type Status struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Color      string `json:"color"`
	Type       string `json:"type"`
	OrderIndex int    `json:"orderindex"`
}

// Space represents a ClickUp space.
type Space struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Color    string   `json:"color"`
	Private  bool     `json:"private"`
	Statuses []Status `json:"statuses"`
	Members  []Member `json:"members"`
}

// SpacesResponse is the API response for listing spaces.
type SpacesResponse struct {
	Spaces []Space `json:"spaces"`
}

// Folder represents a ClickUp folder within a space.
type Folder struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OrderIndex int    `json:"orderindex"`
	Hidden     bool   `json:"hidden"`
	Lists      []List `json:"lists"`
}

// FoldersResponse is the API response for listing folders.
type FoldersResponse struct {
	Folders []Folder `json:"folders"`
}

// List represents a ClickUp list within a folder or space.
type List struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	OrderIndex int      `json:"orderindex"`
	Status     *Status  `json:"status"`
	Statuses   []Status `json:"statuses"`
	FolderID   string   `json:"folder_id,omitempty"`
	SpaceID    string   `json:"space_id,omitempty"`
	Archived   bool     `json:"archived"`
}

// ListsResponse is the API response for listing lists.
type ListsResponse struct {
	Lists []List `json:"lists"`
}

// CustomField represents a custom field value on a task.
type CustomField struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// CustomItem represents a custom task type.
type CustomItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	NamePlural  string `json:"name_plural"`
	Description string `json:"description"`
}

// Task represents a ClickUp task.
type Task struct {
	ID          string       `json:"id"`
	CustomID    string       `json:"custom_id"`
	Name        string       `json:"name"`
	TextContent string       `json:"text_content"`
	Description string       `json:"description"`
	Status      Status       `json:"status"`
	OrderIndex  string       `json:"orderindex"`
	DateCreated string       `json:"date_created"`
	DateUpdated string       `json:"date_updated"`
	DateClosed  string       `json:"date_closed"`
	Creator     User         `json:"creator"`
	Assignees   []User       `json:"assignees"`
	Watchers    []User       `json:"watchers"`
	Tags        []Tag        `json:"tags"`
	Parent      string       `json:"parent"`
	Priority    *Priority    `json:"priority"`
	DueDate     string       `json:"due_date"`
	StartDate   string       `json:"start_date"`
	TimeEstimate int64       `json:"time_estimate"`
	TimeSpent   int64        `json:"time_spent"`
	CustomFields []CustomField `json:"custom_fields"`
	ListID      string       `json:"list_id"`
	FolderID    string       `json:"folder_id"`
	SpaceID     string       `json:"space_id"`
	URL         string       `json:"url"`
	CustomItem  *CustomItem  `json:"custom_item"`
	Subtasks    []Task       `json:"subtasks"`
}

// Tag represents a task tag.
type Tag struct {
	Name    string `json:"name"`
	TagFG   string `json:"tag_fg"`
	TagBG   string `json:"tag_bg"`
	Creator int    `json:"creator"`
}

// Priority represents a task priority.
type Priority struct {
	ID         string `json:"id"`
	Priority   string `json:"priority"`
	Color      string `json:"color"`
	OrderIndex string `json:"orderindex"`
}

// TasksResponse is the API response for listing tasks.
type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}

// TeamsResponse is the API response for listing authorized workspaces.
type TeamsResponse struct {
	Teams []struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Members []Member `json:"members"`
	} `json:"teams"`
}

// CustomItemsResponse is the API response for listing custom task types.
type CustomItemsResponse struct {
	CustomItems []CustomItem `json:"custom_items"`
}

// Comment represents a comment on a task.
type Comment struct {
	ID          string        `json:"id"`
	Comment     []CommentBody `json:"comment"`
	CommentText string        `json:"comment_text"`
	User        User          `json:"user"`
	Resolved    bool          `json:"resolved"`
	Assignee    *User         `json:"assignee"`
	AssignedBy  *User         `json:"assigned_by"`
	Reactions   []Reaction    `json:"reactions"`
	Date        string        `json:"date"`
}

// CommentBody represents the rich text body of a comment.
type CommentBody struct {
	Text       string      `json:"text"`
	Attributes interface{} `json:"attributes,omitempty"`
}

// Reaction represents an emoji reaction on a comment.
type Reaction struct {
	Reaction string `json:"reaction"`
	Date     string `json:"date"`
	User     User   `json:"user"`
}

// CommentsResponse is the API response for listing comments.
type CommentsResponse struct {
	Comments []Comment `json:"comments"`
}

// TimeEntry represents a time tracking entry.
type TimeEntry struct {
	ID          string `json:"id"`
	Task        Task   `json:"task"`
	WID         string `json:"wid"`
	User        User   `json:"user"`
	Billable    bool   `json:"billable"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Duration    string `json:"duration"`
	Description string `json:"description"`
	Tags        []Tag  `json:"tags"`
	Source      string `json:"source"`
	TaskURL     string `json:"task_url"`
}

// TimeEntriesResponse is the API response for listing time entries.
type TimeEntriesResponse struct {
	Data []TimeEntry `json:"data"`
}

// TeamResponse is the API response for getting a team (workspace).
type TeamResponse struct {
	Team struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Color   string   `json:"color"`
		Avatar  string   `json:"avatar"`
		Members []Member `json:"members"`
	} `json:"team"`
}

// AuthorizedUserResponse is the API response for the authorized user endpoint.
type AuthorizedUserResponse struct {
	User User `json:"user"`
}

// --- Request types ---

// CreateCommentRequest is the payload for creating a comment.
type CreateCommentRequest struct {
	CommentText string `json:"comment_text"`
	Assignee    int    `json:"assignee,omitempty"`
	NotifyAll   bool   `json:"notify_all"`
}

// UpdateCommentRequest is the payload for updating a comment.
type UpdateCommentRequest struct {
	CommentText string `json:"comment_text"`
}

// UpdateTaskRequest is the payload for updating a task.
type UpdateTaskRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Status      *string  `json:"status,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	DueDate     *int64   `json:"due_date,omitempty"`
	StartDate   *int64   `json:"start_date,omitempty"`
	Assignees   *Assignees `json:"assignees,omitempty"`
	TimeEstimate *int64  `json:"time_estimate,omitempty"`
}

// Assignees is used in UpdateTaskRequest to add/remove assignees.
type Assignees struct {
	Add    []int `json:"add,omitempty"`
	Remove []int `json:"rem,omitempty"`
}

// CreateTimeEntryRequest is the payload for creating a time entry.
type CreateTimeEntryRequest struct {
	Description string `json:"description,omitempty"`
	Tags        []Tag  `json:"tags,omitempty"`
	Start       *int64 `json:"start,omitempty"`
	Stop        *int64 `json:"stop,omitempty"`
	End         *int64 `json:"end,omitempty"`
	Duration    int64  `json:"duration"`
	Assignee    int    `json:"assignee,omitempty"`
	Billable    bool   `json:"billable,omitempty"`
	TaskID      string `json:"tid,omitempty"`
}
