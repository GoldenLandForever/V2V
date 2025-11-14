package models

type T2IRequest struct {
	TaskID string `json:"task_id"`
}

type T2ITask struct {
	TaskID          uint64 `json:"task_id"`
	UserID          uint64 `json:"user_id"`
	Prompt          string `json:"prompt"`
	Priority        int    `json:"priority,omitempty"`
	Status          string `json:"status"` // pending, processing, completed, failed
	Result          string `json:"result"` // 生成的图片URL或base64
	CreatedAt       int64  `json:"created_at"`
	GeneratedImages int64  `json:"generated_images"`
}

type T2IResponse struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	GeneratedImages int64  `json:"generated_images"`
	Result          string `json:"result"`
}
