package task

type T2IRequest struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Text      string `json:"text"`
	Priority  int    `json:"priority,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}
