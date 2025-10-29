package store

import (
	"V2V/task"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

// TaskStore 是任务持久化最小接口
type TaskStore interface {
	SetTask(v interface{}) error
	GetTask(taskID string, out interface{}) error
}

var (
	Client *redis.Client
)

func Init(cfg string) (err error) {
	Client = redis.NewClient(&redis.Options{
		Addr: cfg,
	})

	_, err = Client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func GetRedis() *redis.Client {
	return Client
}
func V2TTask(t task.V2TTask) error {
	//将任务存储到redis中
	key := "user:" + strconv.FormatUint(t.UserID, 10) + ":task:" + strconv.FormatUint(t.TaskID, 10)
	// 一次设置多个字段（HSet 支持 map）
	fields := map[string]interface{}{
		"video_url":  t.VideoURL,
		"status":     t.Status,
		"result":     t.Result,
		"priority":   t.Priority,
		"created_at": t.CreatedAt,
	}
	// 使用 pipeline（或 TxPipeline）把 HSet 和 Expire 放在同一个请求组里
	pipe := Client.Pipeline()
	pipe.HMSet(key, fields) // 或 pipe.HSet(key, fields) 视版本而定
	pipe.Expire(key, 24*time.Hour)
	_, err := pipe.Exec()
	if err != nil {
		//日志报错
		log.Printf("Failed to store task %s: %v", t.TaskID, err)
		return err
	}
	return nil
}

// func GetTask(taskID string, out interface{}) error {

// 	return json.Unmarshal(b, out)
// }

func T2IImage(T2IRequest task.T2IRequest, url string) error {
	//将任务存储到redis中
	key := "user:" + strconv.FormatUint(T2IRequest.UserID, 10) + ":task:" + strconv.FormatUint(T2IRequest.TaskID, 10)
	err := Client.Set(key, url, 24*time.Hour).Err()
	if err != nil {
		//日志报错
		log.Printf("Failed to store t2i image for prompt %s: %v", T2IRequest.Text, err)
		return err
	}
	return nil
}

func I2VTaskVideoURL(taskID string, videoURL string) error {
	//将任务存储到redis中
	key := "i2v:task:" + taskID + ":video_url"
	err := Client.Set(key, videoURL, 24*time.Hour).Err()
	if err != nil {
		//日志报错
		log.Printf("Failed to store i2v video url for task %s: %v", taskID, err)
		return err
	}
	return nil
}
