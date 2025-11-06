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
	// pipe.Expire(key, 24*time.Hour)
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

func T2ITask(t2iTask task.T2ITask) error {
	//将任务存储到redis中

	key := "user:" + strconv.FormatUint(t2iTask.UserID, 10) + ":task:" + strconv.FormatUint(t2iTask.TaskID, 10)
	// 一次设置多个字段（HSet 支持 map）
	fields := map[string]interface{}{
		"prompt":     t2iTask.Prompt,
		"status":     t2iTask.Status,
		"result":     t2iTask.Result,
		"priority":   t2iTask.Priority,
		"created_at": t2iTask.CreatedAt,
	}
	// 使用 pipeline（或 TxPipeline）把 HSet 和 Expire 放在同一个请求组里
	pipe := Client.Pipeline()
	pipe.HMSet(key, fields) // 或 pipe.HSet(key, fields) 视版本而定
	// pipe.Expire(key, 24*time.Hour)
	_, err := pipe.Exec()
	if err != nil {
		//日志报错
		log.Printf("Failed to store t2i task %s: %v", t2iTask.TaskID, err)
		return err
	}
	return nil
}

// bug :
// 这个地方应该在I2V初始化一下
// 而不是在FFmepeg初始化
func I2VTaskVideoURL(taskID string, videoURL string) error {
	//将任务存储到redis中
	key := "user:0:i2vtask:" + taskID + ":video_url"
	err := Client.Set(key, videoURL, 24*time.Hour).Err()
	if err != nil {
		//日志报错
		log.Printf("Failed to store i2v video url for task %s: %v", taskID, err)
		return err
	}
	return nil
}

func I2VTaskID(taskID, index int, callbacktaskID string) error {
	//将任务存储到redis中
	key := "user:0:i2vtask:" + strconv.Itoa(taskID)
	// 使用 ZAdd 存储分镜任务ID，成员为 callbacktaskID，分数为 index
	err := Client.ZAdd(key, redis.Z{
		Score:  float64(index),
		Member: callbacktaskID,
	}).Err()

	if err != nil {
		//日志报错
		log.Printf("Failed to store i2v task id for task %d part %d: %v", taskID, index, err)
		return err
	}
	err = I2VTaskVideoURL(callbacktaskID, "")
	if err != nil {
		log.Printf("Failed to initialize i2v video url for task %d: %v", taskID, err)
		return err
	}
	return nil
}
