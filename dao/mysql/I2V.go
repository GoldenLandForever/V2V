package mysql

import (
	"time"
)

// InsertI2VTask 插入一条 I2V 任务记录
func InsertI2VTask(taskID int, index int, video_id string, userID uint64, prompt string) error {
	query := "INSERT INTO i2v_task_main (task_id, user_id, status, video_id, `index`, prompt, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	now := time.Now()
	_, err := Db.Exec(query, taskID, userID, "pending", video_id, index, prompt, now, now)
	return err
}

func UpdateI2VTask(video_id string, video_url string, token int) error {
	query := `UPDATE i2v_task_main SET status = ?,video_url = ?,token = ? , updated_at = ? WHERE video_id = ?`
	now := time.Now()
	_, err := Db.Exec(query, "succeed", video_url, token, now, video_id)
	return err
}
