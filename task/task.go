// task/task.go
package task

// 状态常量
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// VideoRequest 表示用户提交的请求部分，必须包含 UserID 和 VideoURL
type V2TRequest struct {
	UserID    uint64 `json:"user_id"`
	VideoURL  string `json:"video_url"`
	Priority  int    `json:"priority,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// VideoResponse 是返回给用户的响应部分
type V2TResponse struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Status    string `json:"status"`               // pending/processing/completed/failed
	Result    string `json:"result,omitempty"`     // 处理结果或错误信息
	UpdatedAt int64  `json:"updated_at,omitempty"` // 最后更新时间（Unix 秒）
}

// VideoTask 是内部持久化/传递的任务结构，包含请求和元数据
type V2TTask struct {
	V2TRequest
	TaskID    uint64 `json:"task_id"`
	Status    string `json:"status"`
	Result    string `json:"result,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

type LoraTextRequest struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Result    string `json:"result,omitempty"`
	Priority  int    `json:"priority,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}
