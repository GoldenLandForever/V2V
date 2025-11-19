package mysql

import (
	"time"

	"V2V/models"
)

// InsertV2TTask 插入一条 V2T 任务记录
func InsertV2TTask(task *models.V2TTask) error {
	query := `INSERT INTO t_v2t_tasks (task_id, user_id, status, result, video_url, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	videoURL := ""
	if (task.V2TRequest != models.V2TRequest{}) {
		videoURL = task.V2TRequest.VideoURL
	}
	_, err := Db.Exec(query, task.TaskID, task.UserID, task.Status, task.Result, videoURL, now, now)
	return err
}
