package models

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type V2TRequest struct {
	VideoURL string `json:"video_url"`
}

type V2TTask struct {
	UserID     uint64     `json:"user_id"`
	TaskID     uint64     `json:"task_id"`
	Status     string     `json:"status"`
	Result     string     `json:"result"`
	Priority   int        `json:"priority,omitempty"`
	V2TRequest V2TRequest `json:"v2t_request"`
}

type V2TResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Result string `json:"result"`
}

type LoraTextRequest struct {
	TaskID uint64 `json:"task_id"`
	Prompt string `json:"prompt"`
}
