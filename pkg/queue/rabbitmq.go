package queue

import (
	"V2V/dao/store"
	"V2V/models"
	"V2V/pkg/sse"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"
	"google.golang.org/genai"
)

// MessageQueue 是队列最小接口（用于发布与消费）
type MessageQueue interface {
	Publish([]byte, int) error
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
	// 创建死信交换与死信队列
	//
	// 概念回顾（注释说明）：
	// - Dead Letter Queue (DLQ)：用于保存无法被正常处理或需要人工介入的消息。
	// - Dead Letter Exchange (DLX)：当队列对某条消息执行 Nack(requeue=false) 或消息过期/超出长度等情况时，
	//   RabbitMQ 会把该消息路由到配置的 DLX，由 DLX 转发到指定的 DLQ。
	// - 为什么需要交换机（DLX）而不是直接把消息放到队列：
	//   交换机（Exchange）是 RabbitMQ 的路由中心，DLX 允许把不同来源或不同路由 key 的消息进行灵活路由，
	//   例如可以把不同主队列的死信都路由到同一个 DLQ 或不同 DLQ；使用 exchange 可以实现更灵活的拓扑。
	//
	// 本实现：
	// - 建立一个 direct 类型的 DLX（v2t_dlq_exchange），并声明 DLQ 队列（v2t_dlq）绑定到该 DLX；
	// - 在主队列 `v2t_tasks` 的参数中设置 `x-dead-letter-exchange` 与 `x-dead-letter-routing-key`，
	//   当我们对消息执行 `Nack(false,false)`（不重入队）时，消息会被送到 DLX -> DLQ。

	dlxName := "v2t_dlq_exchange"
	dlqName := "v2t_dlq"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	if err := ch.QueueBind(dlqName, dlqName, dlxName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// 为主队列添加 dead-letter 配置（当 Nack requeue=false 时会进入 DLQ）
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": dlqName,
		"x-max-priority":            10,
	}
	// _, _ = ch.QueueDelete("v2t_tasks", false, false, false)
	q, err := ch.QueueDeclare(
		"v2t_tasks", // name
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
	// basic QoS: 设置 prefetch，配合消费者并发数使用以提高吞吐
	// 值可以根据实际负载调整或由环境变量配置
	_ = ch.Qos(10, 0, false)
	return &amqpQueue{conn: conn, ch: ch, queueName: q.Name}, nil
}

func (q *amqpQueue) Publish(b []byte, priority int) error {
	return q.ch.Publish(
		"", q.queueName, false, false,
		amqp.Publishing{ContentType: "application/json", Body: b, DeliveryMode: amqp.Persistent, Priority: uint8(priority)},
	)
}

// publishWithHeaders 发布消息并携带自定义 header（用于重试计数）
func (q *amqpQueue) publishWithHeaders(b []byte, headers amqp.Table) error {
	msg := amqp.Publishing{
		ContentType:  "application/json",
		Body:         b,
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
	}
	return q.ch.Publish("", q.queueName, false, false, msg)
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
		//
		// 处理策略说明（注释）：
		// - 首先把消息反序列化为内部任务结构 `V2TTask`，如果解析失败则认为该消息无效，直接 Nack(requeue=false) -> 进入 DLQ。
		// - 调用外部视频分析 API：
		//   - 若返回永久错误（例如参数错误 / HTTP 400 / INVALID_ARGUMENT），则认为重试没有意义，Nack(false,false) -> DLQ。
		//   - 若返回临时错误（网络、限流等），则用 header `x-attempts` 跟踪重试次数：
		//     - 如果 attempts < maxRetries：把消息带上 attempts+1 重新发布到主队列（publishWithHeaders），并 Ack 原消息。
		//       这里采用 re-publish + ack 的方式而不是 Nack(requeue=true)，以便我们可以在 republish 时修改 header（amqp 内部 redelivery header 无法修改）。
		//     - 如果 attempts >= maxRetries：Nack(false,false) -> DLQ。
		// - 存储结果到 Redis 时也会进行错误判断：存储失败视为临时错误，可重试（使用 del.Redelivered 或 header 来决定是否丢弃）。
		//
		// 设计权衡：
		// - 采用 republish 而不是直接 nack requeue=true 的原因是我们想在重试时增加/修改 header（x-attempts），
		//   以便精确控制最大重试次数。直接 requeue 无法修改 header。
		// - 目前实现是立即重试（republish 立刻入队），若需延迟重试应引入延迟队列/TTL 或 RabbitMQ 的 delayed-message 插件。
		// - 当消息进入 DLQ 时，应该有独立的监控/处理流程用于人工排查或自动补救（例如把修复后的消息回放）。
		go func(del amqp.Delivery) {
			defer func() { <-sem; wg.Done() }()

			var vt models.V2TTask
			if err := json.Unmarshal(del.Body, &vt); err != nil {
				log.Printf("Invalid task payload: %v", err)
				// 非法消息，丢弃或送 DLQ（这里选择不重试）
				// 注：对非法消息直接 Nack(false,false) 会触发队列的 x-dead-letter-exchange 路由到 DLQ
				_ = del.Nack(false, false)
				return
			}

			// 调用分析 API
			taskIDStr := strconv.FormatUint(vt.TaskID, 10)
			text, err := callVideoAnalysisAPI(vt.V2TRequest.VideoURL)
			if err != nil {
				// 将错误分类为永久错误或临时错误
				es := err.Error()
				upper := strings.ToUpper(es)
				isPermanent := strings.Contains(upper, "INVALID_ARGUMENT") || strings.Contains(es, "400")
				if isPermanent {
					// 永久错误：参数不合法等原因，重试无意义，直接送 DLQ
					log.Printf("Permanent error calling video analysis API, task id: %s: %v", taskIDStr, err)
					_ = del.Nack(false, false)
					payload := struct {
						Code   int    `json:"code"`
						UserID uint64 `json:"user_id"`
						TaskID uint64 `json:"task_id"`
						Status string `json:"status"`
						Result string `json:"result,omitempty"`
					}{
						Code:   400,
						UserID: vt.UserID,
						TaskID: vt.TaskID,
						Status: models.StatusFailed,
						Result: err.Error(),
					}
					if hub := sse.GetHub(); hub != nil {
						if b, err := json.Marshal(payload); err == nil {
							hub.PublishTopic(strconv.FormatUint(vt.UserID, 10), b)
						}
					}
					return
				}

				// 检查 header 中的重试计数（x-attempts），决定是否把消息送入 DLQ
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

				// 重试策略：最多重试 maxRetries 次，超过则进入 DLQ
				maxRetries := 1
				if attempts >= maxRetries {
					log.Printf("Exceeded retries, sending to DLQ, task id: %s: %v", taskIDStr, err)
					payload := struct {
						Code   int    `json:"code"`
						UserID uint64 `json:"user_id"`
						TaskID uint64 `json:"task_id"`
						Status string `json:"status"`
						Result string `json:"result,omitempty"`
					}{
						Code:   500,
						UserID: vt.UserID,
						TaskID: vt.TaskID,
						Status: models.StatusFailed,
						Result: err.Error(),
					}
					if hub := sse.GetHub(); hub != nil {
						if b, err := json.Marshal(payload); err == nil {
							hub.PublishTopic(strconv.FormatUint(vt.UserID, 10), b)
						}
					}
					// 发送到死信队列（通过 nack requeue=false 按队列 x-dead-letter 配置路由）
					_ = del.Nack(false, false)
					return
				}

				// 重新发布消息到主队列并增加 attempts header，然后 ack 当前消息以避免重复
				// 说明：使用 republish + ack 而不是 nack(requeue=true) 的原因是我们可以修改 header（x-attempts）
				newHeaders := amqp.Table{}
				for k, v := range del.Headers {
					newHeaders[k] = v
				}
				newHeaders["x-attempts"] = attempts + 1

				if err := q.publishWithHeaders(del.Body, newHeaders); err != nil {
					log.Printf("Failed to republish message for retry, task id: %s: %v", taskIDStr, err)
					// republish 失败，选择将原消息 nack 并不重入（可改为重入或记录为警告）
					_ = del.Nack(false, false)
					return
				}
				log.Printf("Requeued message for retry #%d, task id: %s", attempts+1, taskIDStr)
				_ = del.Ack(false)
				return
			}

			vt.Result = text
			vt.Status = models.StatusCompleted
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
			// 成功存储到 Redis 之后，通过 SSE 通知前端（按 task_id topic 发布）
			// 构造通知载荷（可根据前端约定调整字段）
			payload := struct {
				Code   int    `json:"code"`
				UserID uint64 `json:"user_id"`
				TaskID uint64 `json:"task_id"`
				Status string `json:"status"`
				Result string `json:"result,omitempty"`
			}{
				Code:   200,
				UserID: vt.UserID,
				TaskID: vt.TaskID,
				Status: vt.Status,
				Result: vt.Result,
			}
			if hub := sse.GetHub(); hub != nil {
				if b, err := json.Marshal(payload); err == nil {
					hub.PublishTopic(strconv.FormatUint(vt.UserID, 10), b)
				}
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
镜号：为每个镜头分配唯一编号，方便管理与引用（分镜数量与原视频保持一致）。
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
	//计算执行时间
	starttime := time.Now()
	defer func() {
		elapsed := time.Since(starttime)
		log.Printf("Video analysis API call took %s", elapsed)
	}()
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

func callVideoAnalysisAPIDoubao(url string) (string, error) {
	//计算执行时间
	starttime := time.Now()
	defer func() {
		elapsed := time.Since(starttime)
		log.Printf("Video analysis API call took %s", elapsed)
	}()
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
