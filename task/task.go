// task/task.go
package task

type VideoTask struct {
	TaskID    string `json:"task_id"`
	VideoURL  string `json:"video_url"`
	Status    string `json:"status"` // "pending", "processing", "completed", "failed"
	Result    string `json:"result"`
	CreatedAt int64  `json:"created_at"`
}
