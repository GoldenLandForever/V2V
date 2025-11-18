package queue

import (
	"V2V/dao/store"
	"V2V/models"
	"V2V/pkg/sse"
	"V2V/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// T2IMessageQueue 文字生图像专用队列接口
type T2IMessageQueue interface {
	PublishT2ITask([]byte, int) error
	ConsumeT2I() error
	Close() error
}

var (
	t2iOnce     sync.Once
	t2iInstance T2IMessageQueue
	t2iInitErr  error
)

// InitT2IRabbitMQ 初始化T2I RabbitMQ
func InitT2IRabbitMQ(dsn string) error {
	t2iOnce.Do(func() {
		inst, err := newT2IAMQPQueue(dsn)
		if err != nil {
			t2iInitErr = err
			log.Printf("failed to init T2I AMQP queue: %v", err)
			return
		}
		t2iInstance = inst
	})
	return t2iInitErr
}

// GetT2IRabbitMQ 获取T2I队列实例
func GetT2IRabbitMQ() (T2IMessageQueue, error) {
	if t2iInstance == nil {
		if t2iInitErr != nil {
			return nil, t2iInitErr
		}
		return nil, errors.New("t2i rabbitmq not initialized; call InitT2IRabbitMQ")
	}
	return t2iInstance, nil
}

// --- T2I AMQP 实现 ---
type t2iAMQPQueue struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
}

func newT2IAMQPQueue(dsn string) (T2IMessageQueue, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// T2I专用死信交换机和队列
	dlxName := "t2i_dlq_exchange"
	dlqName := "t2i_dlq"

	// 声明死信交换机
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 声明死信队列
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 绑定死信队列
	if err := ch.QueueBind(dlqName, dlqName, dlxName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 主队列参数，设置死信路由
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": dlqName,
		"x-max-priority":            10,
	}

	// 声明T2I任务队列

	// _, _ = ch.QueueDelete("t2i_tasks", false, false, false)
	q, err := ch.QueueDeclare(
		"t2i_tasks", // 队列名称不同
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		args,        // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 设置QoS
	_ = ch.Qos(5, 0, false) // T2I任务可能更耗资源，并发数可以小一些

	return &t2iAMQPQueue{conn: conn, ch: ch, queueName: q.Name}, nil
}

// PublishT2ITask 发布T2I任务
func (q *t2iAMQPQueue) PublishT2ITask(b []byte, priority int) error {

	return q.ch.Publish(
		"", q.queueName, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         b,
			DeliveryMode: amqp.Persistent,
			Priority:     uint8(priority),
		},
	)
}

// publishWithHeaders 带header发布（用于重试）
func (q *t2iAMQPQueue) publishWithHeaders(b []byte, headers amqp.Table) error {
	msg := amqp.Publishing{
		ContentType:  "application/json",
		Body:         b,
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Priority:     5, // 默认中等优先级
	}
	return q.ch.Publish("", q.queueName, false, false, msg)
}

// ConsumeT2I 消费T2I任务
func (q *t2iAMQPQueue) ConsumeT2I() error {
	deliveries, err := q.ch.Consume(q.queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	concurrency := 10 // T2I任务较耗资源，并发数减少
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for d := range deliveries {
		sem <- struct{}{}
		wg.Add(1)

		go func(del amqp.Delivery) {
			defer func() { <-sem; wg.Done() }()

			var t2iTask models.T2ITask
			if err := json.Unmarshal(del.Body, &t2iTask); err != nil {
				log.Printf("Invalid T2I task payload: %v", err)
				_ = del.Nack(false, false) // 进入DLQ
				return
			}

			taskIDStr := strconv.FormatUint(t2iTask.TaskID, 10)

			// 更新任务状态为处理中
			t2iTask.Status = models.StatusProcessing
			if err := store.T2ITask(t2iTask); err != nil {
				log.Printf("Failed to update T2I task status to processing, task id: %s: %v", taskIDStr, err)
				_ = del.Nack(false, true) // 重试
				return
			}

			// 调用文字生图像API
			t2iTaskresp, err := T2IHandler(t2iTask)
			if err != nil {
				// 错误分类处理
				es := err.Error()
				upper := strings.ToUpper(es)

				// 永久错误：提示词不合法等
				isPermanent := strings.Contains(upper, "INVALID") ||
					strings.Contains(upper, "SAFETY") ||
					strings.Contains(es, "400")

				if isPermanent {
					log.Printf("Permanent error in T2I API, task id: %s: %v", taskIDStr, err)
					t2iTask.Status = models.StatusFailed
					store.T2ITask(t2iTask)     // 忽略存储错误
					_ = del.Nack(false, false) // 进入DLQ
					return
				}

				// 检查重试次数
				attempts := 0
				if h, ok := del.Headers["x-attempts"]; ok {
					switch v := h.(type) {
					case int:
						attempts = v
					case int32:
						attempts = int(v)
					case int64:
						attempts = int(v)
					case string:
						if n, err := strconv.Atoi(v); err == nil {
							attempts = n
						}
					}
				}

				maxRetries := 3
				if attempts >= maxRetries {
					log.Printf("T2I task exceeded retries, sending to DLQ, task id: %s: %v", taskIDStr, err)
					t2iTask.Status = models.StatusFailed
					store.T2ITask(t2iTask)
					_ = del.Nack(false, false)
					return
				}

				// 重试
				newHeaders := amqp.Table{"x-attempts": attempts + 1}
				for k, v := range del.Headers {
					if k != "x-attempts" {
						newHeaders[k] = v
					}
				}

				if err := q.publishWithHeaders(del.Body, newHeaders); err != nil {
					log.Printf("Failed to republish T2I message for retry, task id: %s: %v", taskIDStr, err)
					_ = del.Nack(false, false)
					return
				}

				log.Printf("Requeued T2I message for retry #%d, task id: %s", attempts+1, taskIDStr)
				_ = del.Ack(false)
				return
			}

			// 处理成功
			var url string
			for i, image := range t2iTaskresp.Data {

				if image.Url != nil {
					//下载图片存储到public/pic目录下
					err = util.DownloadImages(*image.Url, strconv.FormatUint(t2iTask.TaskID, 10), i)
					url = url + *image.Url + "|z|k|x|"
				}
			}
			t2iTask.Result = url
			t2iTask.Status = models.StatusCompleted
			t2iTask.GeneratedImages = t2iTaskresp.Usage.GeneratedImages

			// 存储结果
			if err := store.T2ITask(t2iTask); err != nil {
				log.Printf("Failed to update T2I task result, task id: %s: %v", taskIDStr, err)
				if del.Redelivered {
					_ = del.Nack(false, false)
				} else {
					_ = del.Nack(false, true)
				}
				return
			}

			if err != nil {
				log.Printf("Failed to download images for T2I task, task id: %s: %v", taskIDStr, err)
			}

			// SSE通知
			payload := struct {
				Code            int    `json:"code"`
				UserID          uint64 `json:"user_id"`
				TaskID          uint64 `json:"task_id"`
				Status          string `json:"status"`
				Result          string `json:"result,omitempty"`
				GeneratedImages int64  `json:"generated_images"`
			}{
				Code:            200,
				UserID:          t2iTask.UserID,
				TaskID:          t2iTask.TaskID,
				Status:          t2iTask.Status,
				Result:          t2iTask.Result,
				GeneratedImages: t2iTask.GeneratedImages,
			}

			if hub := sse.GetHub(); hub != nil {
				if b, err := json.Marshal(payload); err == nil {
					hub.PublishTopic(strconv.FormatUint(t2iTask.UserID, 10), b)
				}
			}

			_ = del.Ack(false)
			log.Printf("T2I task completed successfully, task id: %s", taskIDStr)

		}(d)
	}

	wg.Wait()
	return nil
}

// T2I图像生成函数类型（可以替换为实际的AI服务调用）

func T2IHandler(T2IRequest models.T2ITask) (model.ImagesResponse, error) {
	client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
	ctx := context.Background()

	var sequentialImageGeneration model.SequentialImageGeneration = "auto"
	maxImages := 15
	generateReq := model.GenerateImagesRequest{
		Model:                     "doubao-seedream-4-0-250828",
		Prompt:                    "请按照分镜数生成图像数" + T2IRequest.Prompt,
		Size:                      volcengine.String("1K"),
		ResponseFormat:            volcengine.String(model.GenerateImagesResponseFormatURL),
		Watermark:                 volcengine.Bool(true),
		Seed:                      volcengine.Int64(42),
		SequentialImageGeneration: &sequentialImageGeneration,
		SequentialImageGenerationOptions: &model.SequentialImageGenerationOptions{
			MaxImages: &maxImages,
		},
	}
	//计算执行时间
	starttime := time.Now()
	defer func() {
		elapsed := time.Since(starttime)
		fmt.Printf("T2I API call took %s\n", elapsed)
	}()
	t2iTaskresp, err := client.GenerateImages(ctx, generateReq)
	if err != nil {
		fmt.Printf("call GenerateImages error: %v\n", err)
		return t2iTaskresp, err
	}
	if t2iTaskresp.Error != nil {
		fmt.Printf("API returned error: %s - %s\n", t2iTaskresp.Error.Code, t2iTaskresp.Error.Message)
		return t2iTaskresp, errors.New(t2iTaskresp.Error.Message)
	}
	// 输出生成的图片信息
	fmt.Printf("Generated %d images:\n", len(t2iTaskresp.Data))
	for i, image := range t2iTaskresp.Data {
		var url string
		if image.Url != nil {
			url = *image.Url
		} else {
			url = "N/A"
		}
		fmt.Printf("Image %d: Size: %s, URL: %s\n", i+1, image.Size, url)
	}
	return t2iTaskresp, nil
}

func (q *t2iAMQPQueue) Close() error {
	if q.ch != nil {
		_ = q.ch.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
