// store/redis_store.go
package store

import (
	"context"
	"encoding/json"
	"time"

	"V2V/task"

	"github.com/go-redis/redis/v8"
)

type TaskStore interface {
	SetTask(task task.VideoTask) error
	GetTask(taskID string) (task.VideoTask, error)
}

type RedisStore struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
		ctx: context.Background(),
	}
}

func (r *RedisStore) SetTask(t task.VideoTask) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return r.client.Set(r.ctx, "task:"+t.TaskID, data, 24*time.Hour).Err()
}

func (r *RedisStore) GetTask(taskID string) (task.VideoTask, error) {
	data, err := r.client.Get(r.ctx, "task:"+taskID).Bytes()
	if err != nil {
		return task.VideoTask{}, err
	}

	var t task.VideoTask
	err = json.Unmarshal(data, &t)
	return t, err
}
