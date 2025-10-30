package task

type T2IRequest struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Text      string `json:"text"`
	Priority  int    `json:"priority,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

type T2ITask struct {
	TaskID    uint64 `json:"task_id"`
	UserID    uint64 `json:"user_id"`
	Prompt    string `json:"prompt"`
	Priority  int    `json:"priority,omitempty"`
	Status    string `json:"status"` // pending, processing, completed, failed
	Result    string `json:"result"` // 生成的图片URL或base64
	CreatedAt int64  `json:"created_at"`
}

const (
	T2IStatusPending    = "pending"
	T2IStatusProcessing = "processing"
	T2IStatusCompleted  = "completed"
	T2IStatusFailed     = "failed"
)
