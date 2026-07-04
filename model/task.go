package model

// Task represent a task - the building block of the TaskManager app
type Task struct {
	ID        int64  `storm:"id,increment",json:"id"`
	ProjectID int64  `storm:"index",json:"project_id"`
	UUID      string `storm:"unique",json:"uuid,omitempty"`
	Title     string `json:"text"`
	Details   string `json:"notes"`
	Completed bool   `storm:"index",json:"completed"`
	DueDate   int64  `storm:"index",json:"due_date,omitempty"`
	Rank      int64  `storm:"index",json:"rank"`
	// Priority follows org-mode: 1=A (highest), 2=B, 3=C (lowest). 0 means unset
	// and is treated as B, the default.
	Priority int64 `json:"priority,omitempty"`
}
