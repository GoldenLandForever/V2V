package queue

import (
	"V2V/dao/mysql"
	"V2V/dao/store"
	"V2V/pkg/sse"
	"V2V/util"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
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
				UserID    uint64 `json:"user_id"`
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
			key := "user:" + strconv.FormatUint(checkTask.UserID, 10) + ":i2vtask:" + checkTask.SubTaskID + ":video_url"
			key2 := "user:" + strconv.FormatUint(checkTask.UserID, 10) + ":i2vtaskstatus:" + checkTask.TaskID
			status, err := redisClient.HGet(key, "status").Result()
			if err != nil {
				fmt.Printf("Failed to get task status from Redis: %v\n", err)
				fmt.Println("key:", key)
				d.Nack(false, false) // 重试
				continue
			} else {
				fmt.Println("succeed key:", key)
			}

			// 如果状态不是终态，则查询最新状态
			if status != "succeeded" && status != "failed" {
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
					log.Println("succeed subtask id:", checkTask.SubTaskID)
					contentURL = resp.Content.VideoURL
					//暂时不扣费
					// temptaskid, _ := strconv.ParseUint(checkTask.TaskID, 10, 64)
					// mysql.DeductTokensForTask(checkTask.UserID, temptaskid, int64(resp.Usage.CompletionTokens))
				}

				// 使用与I2VCallback相同的Lua脚本更新状态
				_, err = redisClient.Eval(`
					local key = KEYS[1]
					local field = ARGV[1]
					local new = ARGV[2]
					local video_url = ARGV[3]
					local key2 = ARGV[4]
					local old = redis.call('HGET', key, field)
					if old == 'succeeded' or old == 'failed' then
						return {redis.call('HGET', key2, 'succeeded'), redis.call('HGET', key2, 'failed')}
					end
					redis.call('HSET', key, field, new)
					if new == 'succeeded' then
						redis.call('HINCRBY', key2, 'succeeded', 1)
						redis.call('HSET', key, 'video_url', video_url)
					elseif new == 'failed' then
						redis.call('HINCRBY', key2, 'failed', 1)
					end
					return {redis.call('HGET', key2, 'succeeded'), redis.call('HGET', key2, 'failed')}
				`, []string{key}, "status", strings.ToLower(resp.Status), contentURL, key2).Result()

				if err != nil {
					fmt.Printf("Failed to update Redis status: %v\n", err)
					d.Nack(false, true)
					continue
				}
				// 获取任务统计信息
				succeededStr, err := redisClient.HGet(key2, "succeeded").Result()
				failedStr, err := redisClient.HGet(key2, "failed").Result()
				totalStr, err := redisClient.HGet(key2, "total").Result()

				if err != nil {
					fmt.Printf("Failed to get task statistics key2: %s from Redis: %v\n", key2, err)
					d.Ack(false)
					continue
				}

				if resp.Status == "succeeded" {
					mysql.UpdateI2VTask(checkTask.SubTaskID, contentURL, resp.Usage.CompletionTokens)
					//暂时不扣费
					// temptaskid, _ := strconv.ParseUint(checkTask.TaskID, 10, 64)
					// mysql.DeductTokensForTask(checkTask.UserID, temptaskid, int64(resp.Usage.CompletionTokens))
				}

				// 解析计数值
				succeeded := int64(0)
				failed := int64(0)
				total := int64(0)

				if succeededStr != "" {
					succeeded, _ = strconv.ParseInt(succeededStr, 10, 64)
				}
				if failedStr != "" {
					failed, _ = strconv.ParseInt(failedStr, 10, 64)
				}
				if totalStr != "" {
					total, _ = strconv.ParseInt(totalStr, 10, 64)
				}

				// 构建 SSE 消息
				var sseMsg map[string]interface{}

				// 检查是否有失败的任务
				if failed > 0 {
					sseMsg = map[string]interface{}{
						"code":      500,
						"status":    "failed",
						"task_id":   checkTask.TaskID,
						"succeeded": succeeded,
						"failed":    failed,
						"total":     total,
					}
				} else if succeeded+failed == total && total > 0 {
					// 所有任务完成且没有失败
					util.FFmpeg(checkTask.UserID, checkTask.TaskID)
					sseMsg = map[string]interface{}{
						"code":      200,
						"status":    "success",
						"task_id":   checkTask.TaskID,
						"succeeded": succeeded,
						"failed":    failed,
						"total":     total,
					}
				}

				// 发送 SSE 消息给前端
				if sseMsg != nil {
					msgBytes, err := json.Marshal(sseMsg)
					if err != nil {
						fmt.Printf("Failed to marshal SSE message: %v\n", err)
					} else {
						topic := strconv.FormatUint(checkTask.UserID, 10)
						hub := sse.GetHub()
						if hub != nil {
							hub.PublishTopic(topic, msgBytes)
							fmt.Printf("Published SSE message for user %s: %s\n", topic, string(msgBytes))
						}
					}
				}
			}
			//获得拿到Redis的状态后发送SSE事件给前端

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
