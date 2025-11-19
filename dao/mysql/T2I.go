package mysql

import (
	"time"

	"V2V/models"
)

// InsertT2ITask 将 T2I 任务写入数据库表 t2i_tasks
func InsertT2ITask(task *models.T2ITask) error {
	query := `INSERT INTO t2i_tasks (task_id, user_id, status, token, image_url, prompt, error_message, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	// image_url 对应 models.T2ITask.Result
	_, err := Db.Exec(query, task.TaskID, task.UserID, task.Status, task.Token, task.Result, task.Prompt, "", now, now)
	return err
}
