package queue

import (
	"V2V/dao/store"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/streadway/amqp"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// 延迟队列相关的方法和结构

type DelayedI2VQueue interface {
	PublishDelayedCheck(b []byte) error
	ConsumeDelayedChecks() error
}

// --- 延迟队列 AMQP 实现 ---
type delayedI2VAMQPQueue struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
}

func NewDelayedI2VAMQPQueue(dsn string) (DelayedI2VQueue, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// 声明延迟队列的交换机（x-delayed-message类型）
	err = ch.ExchangeDeclare(
		"i2v_delayed_exchange",
		"x-delayed-message",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-delayed-type": "direct",
		},
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("Failed to declare delayed exchange: %v", err)
	}

	// 声明延迟队列
	queueName := "i2v_delayed_check_queue"
	_, err = ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 绑定延迟队列到交换机
	err = ch.QueueBind(
		queueName,
		queueName,
		"i2v_delayed_exchange",
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &delayedI2VAMQPQueue{
		conn:      conn,
		ch:        ch,
		queueName: queueName,
	}, nil
}

// PublishDelayedCheck 发布延迟检查消息
func (q *delayedI2VAMQPQueue) PublishDelayedCheck(b []byte) error {
	return q.ch.Publish(
		"i2v_delayed_exchange",
		q.queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        b,
			Headers: amqp.Table{
				"x-delay": 60000, // 5分钟 = 300000毫秒
			},
		},
	)
}

// ConsumeDelayedChecks 消费延迟检查消息
func (q *delayedI2VAMQPQueue) ConsumeDelayedChecks() error {
	msgs, err := q.ch.Consume(
		q.queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			var checkTask struct {
				TaskID    string `json:"task_id"`
				SubTaskID string `json:"sub_task_id"`
			}

			if err := json.Unmarshal(d.Body, &checkTask); err != nil {
				fmt.Printf("Failed to unmarshal delayed check task: %v\n", err)
				d.Nack(false, false)
				continue
			}

			// 检查Redis中的状态
			redisClient := store.GetRedis()
			key := "user:0:i2vtask:" + checkTask.SubTaskID + ":video_url"
			status, err := redisClient.Get(key).Result()
			if err != nil {
				fmt.Printf("Failed to get task status from Redis: %v\n", err)
				fmt.Println("key:", key)
				d.Nack(false, false) // 重试
				continue
			} else {
				fmt.Println("succeed key:", key)
			}

			// 如果状态不是终态，则查询最新状态
			// if status != "succeeded" && status != "failed" {
			if status == "" {
				client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
				ctx := context.Background()

				req := model.GetContentGenerationTaskRequest{}
				req.ID = checkTask.SubTaskID

				resp, err := client.GetContentGenerationTask(ctx, req)
				if err != nil {
					fmt.Printf("Failed to get task result: %v\n", err)
					d.Nack(false, true) // 重试
					continue
				}

				// 更新Redis中的状态（使用相同的Lua脚本保持原子性）
				contentURL := ""
				if resp.Status == "succeeded" {
					contentURL = resp.Content.VideoURL
				}

				// 使用与I2VCallback相同的Lua脚本更新状态
				_, err = redisClient.Eval(`
					local key = KEYS[1]
					local field = ARGV[1]
					local new = ARGV[2]
					local video_url = ARGV[3]
					local old = redis.call('HGET', key, field)
					if old == 'succeeded' or old == 'failed' then
						return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed')}
					end
					redis.call('HSET', key, field, new)
					if new == 'succeeded' then
						redis.call('HINCRBY', key, 'succeeded', 1)
						redis.call('SET', 'i2v:task:'..field..':video_url', video_url, 'EX', 86400)
					elseif new == 'failed' then
						redis.call('HINCRBY', key, 'failed', 1)
					end
					return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed')}
				`, []string{key}, checkTask.SubTaskID, strings.ToLower(resp.Status), contentURL).Result()

				if err != nil {
					fmt.Printf("Failed to update Redis status: %v\n", err)
					d.Nack(false, true)
					continue
				}
			}

			d.Ack(false)
		}
	}()

	return nil
}

// 全局延迟队列实例
var (
	delayedInstance DelayedI2VQueue
	delayedInitErr  error
)

// InitDelayedI2VQueue 初始化延迟队列
func InitDelayedI2VQueue(dsn string) error {
	if delayedInstance != nil {
		return nil
	}

	inst, err := NewDelayedI2VAMQPQueue(dsn)
	if err != nil {
		delayedInitErr = err
		return err
	}

	delayedInstance = inst
	// 启动消费者
	return inst.ConsumeDelayedChecks()
}

// GetDelayedI2VQueue 获取延迟队列实例
func GetDelayedI2VQueue() (DelayedI2VQueue, error) {
	if delayedInstance == nil {
		return nil, fmt.Errorf("delayed queue not initialized")
	}
	return delayedInstance, nil
}
