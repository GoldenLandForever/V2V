package task

type I2VRequest struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Prompt    string `json:"prompt"`
	ImageURL  string `json:"image_url"`
	Priority  int    `json:"priority,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

type I2VResponse struct {
	UserID    uint64 `json:"user_id"`
	TaskID    uint64 `json:"task_id"`
	Status    string `json:"status"`               // pending/processing/completed/failed
	Result    string `json:"result,omitempty"`     // 处理结果或错误信息
	UpdatedAt int64  `json:"updated_at,omitempty"` // 最后更新时间（Unix 秒）
}
