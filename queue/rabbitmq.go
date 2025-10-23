package queue

import (
	"V2V/task"
	"encoding/json"
	"log"

	"github.com/streadway/amqp"
)

type MessageQueue interface {
	Publish(task task.VideoTask) error
	Consume() (<-chan task.VideoTask, error)
}

type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbitMQ(uri string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	_, err = ch.QueueDeclare(
		"video_tasks", // queue name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		return nil, err
	}

	return &RabbitMQ{conn: conn, ch: ch}, nil
}

func (r *RabbitMQ) Publish(t task.VideoTask) error {
	body, err := json.Marshal(t)
	if err != nil {
		return err
	}

	return r.ch.Publish(
		"",            // exchange
		"video_tasks", // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	)
}

func (r *RabbitMQ) Consume() (<-chan task.VideoTask, error) {
	msgs, err := r.ch.Consume(
		"video_tasks", // queue
		"",            // consumer
		false,         // auto-ack
		false,         // exclusive
		false,         // no-local
		false,         // no-wait
		nil,           // args
	)
	if err != nil {
		return nil, err
	}

	taskChan := make(chan task.VideoTask)
	go func() {
		for d := range msgs {
			var t task.VideoTask
			if err := json.Unmarshal(d.Body, &t); err == nil {
				taskChan <- t
			}
			d.Ack(false)
		}
		close(taskChan)
	}()

	return taskChan, nil
}

func (r *RabbitMQ) Close() error {
	if r.ch != nil {
		if err := r.ch.Close(); err != nil {
			log.Printf("Failed to close RabbitMQ channel: %v", err)
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			log.Printf("Failed to close RabbitMQ connection: %v", err)
			return err
		}
	}

	return nil
}
