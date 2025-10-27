package queue

import (
	"V2V/dao/store"
	"V2V/task"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/streadway/amqp"
	"google.golang.org/genai"
)

// MessageQueue 是队列最小接口（用于发布与消费）
type MessageQueue interface {
	Publish([]byte) error
	Consume() error
	Close() error
}

var (
	rabbitOnce     sync.Once
	rabbitInstance MessageQueue
	rabbitInitErr  error
)

// InitRabbitMQ 使用单例模式初始化 RabbitMQ（首次调用生效，后续调用忽略）
// 如果无法连接到真实 RabbitMQ，会返回错误；可在需要时回退到内存实现。
func InitRabbitMQ(dsn string) error {
	rabbitOnce.Do(func() {
		inst, err := newAMQPQueue(dsn)
		if err != nil {
			rabbitInitErr = err
			log.Printf("failed to init AMQP queue: %v", err)
			return
		}
		rabbitInstance = inst
	})
	return rabbitInitErr
}

// GetRabbitMQ 返回单例的 MessageQueue，如果未初始化或初始化失败会返回错误
func GetRabbitMQ() (MessageQueue, error) {
	if rabbitInstance == nil {
		if rabbitInitErr != nil {
			return nil, rabbitInitErr
		}
		return nil, errors.New("rabbitmq not initialized; call InitRabbitMQ")
	}
	return rabbitInstance, nil
}

// --- AMQP 实现 ---------------------------------------------------------
type amqpQueue struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
}

func newAMQPQueue(dsn string) (MessageQueue, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	q, err := ch.QueueDeclare(
		"v2t_tasks", // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	// basic QoS: 设置 prefetch，配合消费者并发数使用以提高吞吐
	// 值可以根据实际负载调整或由环境变量配置
	_ = ch.Qos(10, 0, false)
	return &amqpQueue{conn: conn, ch: ch, queueName: q.Name}, nil
}

func (q *amqpQueue) Publish(b []byte) error {
	return q.ch.Publish(
		"", q.queueName, false, false,
		amqp.Publishing{ContentType: "application/json", Body: b, DeliveryMode: amqp.Persistent},
	)
}

// ConsumeAndServe 在 AMQP 消费循环中直接执行 handler，每条消息处理成功后 Ack，失败时 Nack (并可重新入队)
// handler 返回 nil 表示处理成功；非 nil 表示处理失败，函数会根据 requeue 参数决定是否重新入队。
func (q *amqpQueue) Consume() error {
	deliveries, err := q.ch.Consume(q.queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	// 并发控制（与上面 ch.Qos 的值配合使用）
	concurrency := 10
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for d := range deliveries {
		sem <- struct{}{}
		wg.Add(1)
		// spawn goroutine 处理每条消息，处理结束后 Ack/Nack
		go func(del amqp.Delivery) {
			defer func() { <-sem; wg.Done() }()

			var vt task.V2TTask
			if err := json.Unmarshal(del.Body, &vt); err != nil {
				log.Printf("Invalid task payload: %v", err)
				// 非法消息，丢弃或送 DLQ（这里选择不重试）
				_ = del.Nack(false, false)
				return
			}

			// 调用分析 API
			taskIDStr := strconv.FormatUint(vt.TaskID, 10)
			text, err := callVideoAnalysisAPI(vt.VideoURL)
			if err != nil {
				// 将错误分类为永久错误或临时错误
				es := err.Error()
				upper := strings.ToUpper(es)
				isPermanent := strings.Contains(upper, "INVALID_ARGUMENT") || strings.Contains(es, "400")
				if isPermanent {
					log.Printf("Permanent error calling video analysis API, task id: %s: %v", taskIDStr, err)
					// 永久错误：不要重试，丢弃或送 DLQ（这里选择不重试）
					_ = del.Nack(false, false)
					return
				}
				// 如果是已重试过的消息，避免无限重试，丢弃或转 DLQ
				if del.Redelivered {
					log.Printf("Repeated failure calling video analysis API, task id: %s: %v", taskIDStr, err)
					_ = del.Nack(false, false)
					return
				}
				log.Printf("Temporary error calling video analysis API, task id: %s: %v; requeueing once", taskIDStr, err)
				_ = del.Nack(false, true)
				return
			}

			vt.Result = text
			vt.Status = task.StatusCompleted
			if err := store.V2TTask(vt); err != nil {
				tid := strconv.FormatUint(vt.TaskID, 10)
				log.Printf("Failed to update redis, task id: %s: %v", tid, err)
				// 存储失败视为临时问题，重入队（或根据需要改为不重试）
				if del.Redelivered {
					// 已经重试过，丢弃
					_ = del.Nack(false, false)
				} else {
					_ = del.Nack(false, true)
				}
				return
			}

			// 成功处理后 ack
			_ = del.Ack(false)
		}(d)
	}

	// 等待所有处理 goroutine 完成
	wg.Wait()
	return nil
}

func (q *amqpQueue) Close() error {
	if q.ch != nil {
		_ = q.ch.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}

const videoAnalysisText = `#角色你是一位专业且经验丰富的影视分镜师，专注于拆解生动的视觉画面。熟练掌握镜头语言、构图、色彩搭配和叙事节奏，擅长为影视制作、广告宣传、动画创作等提供清晰、专业的分镜脚本框架，确保视觉表现力与叙事逻辑兼具。#技能## 技能 1：理解视频内容并构思分镜
理解视频内容：
全面分析视频内容主题、情节结构和目标用户，确保分镜紧密贴合故事主题。
构思镜头序列：
根据内容，还原镜头顺序。
## 技能 2：生成并优化分镜脚本
生成初始分镜脚本：
根据构思的镜头序列，按照以下格式输出分镜脚本：
镜号、景别、画面内容、台词、运镜方式、音效、时长、图片生成提示词、视频生成提示词
对格式的具体要求：
镜号：为每个镜头分配唯一编号，方便管理与引用。
景别：清晰描述镜头距离（近景、中景、远景、特写等），展现主体与背景的关系。
画面内容：详细明确描述场景、人物、动作、细节，助力视觉化创意。
台词 / 旁白：添加角色台词、旁白或文字提示（如需）。
运镜方式：说明镜头操作（推、拉、摇、移等），体现叙事流动性。
时长：合理估算镜头持续时间（秒为单位），确保节奏流畅。
备注：对镜头的情感表达、技术要求或特殊细节进行补充说明。
图片生成提示词：镜号的首帧图片该如何用文字描述生成图片
视频生成提示词：如何用镜号的首帧图片根据提示词生成对应的视频
检查与优化：
检查清晰度与可行性：确保分镜描述简洁清晰、易懂，完全适合拍摄或制作，严格符合影视制作规范。
合理调整：优化镜头设计，充分考量制作成本与技术难度，避免复杂镜头影响实际执行。`

func callVideoAnalysisAPI(url string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return "", err
	}

	parts := []*genai.Part{
		genai.NewPartFromText(videoAnalysisText),
		genai.NewPartFromURI(url, "video/mp4"),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		contents,
		nil,
	)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", errors.New("genai: empty generate response")
	}

	// 打印生成的结果（可选）
	// log.Printf("Generated video analysis result: %s", result.Text())
	return result.Text(), nil
}
