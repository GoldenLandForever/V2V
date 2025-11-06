package queue

import (
	"V2V/dao/store"
	"V2V/task"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/streadway/amqp"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// TODO： 实现I2V消息队列接口
// 实现I2V延迟队列接口
type I2VMessageQueue interface {
	PublishI2VTask([]byte, int) error
	ConsumeI2V() error
	Close() error
}

var (
	i2vOnce     sync.Once
	i2vInstance I2VMessageQueue
	i2vInitErr  error
)

// InitI2VRabbitMQ 初始化I2V RabbitMQ
func InitI2VRabbitMQ(dsn string) error {
	i2vOnce.Do(func() {
		inst, err := newI2VAMQPQueue(dsn)
		if err != nil {
			i2vInitErr = err
			return
		}
		i2vInstance = inst
	})
	return i2vInitErr
}

// GetI2VRabbitMQ 获取I2V队列实例
func GetI2VRabbitMQ() (I2VMessageQueue, error) {
	if i2vInstance == nil {
		if i2vInitErr != nil {
			return nil, i2vInitErr
		}
		return nil, errors.New("i2v rabbitmq not initialized; call InitI2VRabbitMQ")
	}
	return i2vInstance, nil
}

// --- I2V AMQP 实现 ---
type i2vAMQPQueue struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
}

func newI2VAMQPQueue(dsn string) (I2VMessageQueue, error) {
	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// I2V专用死信交换机和队列
	err = ch.ExchangeDeclare(
		"i2v_dead_letter_exchange",
		"direct",
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

	queueName := "i2v_task_queue"
	_, err = ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "i2v_dead_letter_exchange",
			"x-dead-letter-routing-key": queueName + "_dlq",
		},
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &i2vAMQPQueue{
		conn:      conn,
		ch:        ch,
		queueName: queueName,
	}, nil
}
func (q *i2vAMQPQueue) PublishI2VTask(body []byte, priority int) error {
	return q.ch.Publish(
		"",
		q.queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Priority:    uint8(priority),
		},
	)
}
func (q *i2vAMQPQueue) ConsumeI2V() error {
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
			// 处理I2V任务消息
			var i2vTask task.I2VTask
			err := json.Unmarshal(d.Body, &i2vTask)
			if err != nil {
				fmt.Printf("Failed to unmarshal I2V task: %v\n", err)
				d.Nack(false, false)
				continue
			}

			// 创建I2V任务
			err = createI2VTask(i2vTask.ImageURL, i2vTask.Prompt, i2vTask.Index, int(i2vTask.TaskID))
			if err != nil {
				fmt.Printf("Failed to create I2V task: %v\n", err)
				d.Nack(false, true) // 重试
				continue
			}

			d.Ack(false)
			fmt.Printf("I2V task processed successfully: UserID=%d, TaskID=%d, Index=%d\n", i2vTask.UserID, i2vTask.TaskID, i2vTask.Index)
		}
	}()

	return nil
}
func (q *i2vAMQPQueue) Close() error {
	if err := q.ch.Close(); err != nil {
		return err
	}
	return q.conn.Close()
}

func createI2VTask(refImg, prompts string, index, taskID int) error {
	// err := createI2VTask(img, prompts, idx+1, int(taskID))
	client := arkruntime.NewClientWithApiKey(
		os.Getenv("ARK_API_KEY"),
		arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"),
	)
	ctx := context.Background()
	modelEp := "doubao-seedance-1-0-pro-250528"

	fmt.Println("----- create content generation task -----")

	createReq := model.CreateContentGenerationTaskRequest{
		Model: modelEp,
		Content: []*model.CreateContentGenerationContentItem{
			{
				Type: model.ContentGenerationContentItemTypeText,
				Text: volcengine.String("根据文本与参考图生成第" + strconv.FormatInt(int64(index), 10) + "张分镜的视频" + prompts + " --resolution 720p"),
			},
			{
				Type: model.ContentGenerationContentItemTypeImage,
				ImageURL: &model.ImageURL{
					URL: refImg,
				},
			},
		},
	}

	createResponse, err := client.CreateContentGenerationTask(ctx, createReq)
	if err != nil {
		fmt.Printf("create content generation error: %v", err)
		return err
	}
	fmt.Printf("Task Created with ID: %s \n", createResponse.ID)
	err = store.I2VTaskID(taskID, index, createResponse.ID)
	// 将任务放入延迟队列，等待处理结果
	delayedI2VAMQPQueueInstance, err := GetDelayedI2VQueue()
	if err != nil {
		fmt.Printf("Failed to get delayed I2V queue: %v\n", err)
		return err
	}
	checkTask := struct {
		TaskID    string `json:"task_id"`
		SubTaskID string `json:"sub_task_id"`
	}{
		TaskID:    strconv.Itoa(taskID),
		SubTaskID: createResponse.ID,
	}
	body, err := json.Marshal(checkTask)
	if err != nil {
		fmt.Printf("Failed to marshal delayed check task: %v\n", err)
		return err
	}
	err = delayedI2VAMQPQueueInstance.PublishDelayedCheck(body) // 延迟60秒检查
	if err != nil {
		fmt.Printf("Failed to publish delayed check task: %v\n", err)
	}
	return err
}
