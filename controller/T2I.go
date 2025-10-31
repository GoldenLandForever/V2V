package controller

import (
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"V2V/task"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
)

func SubmitT2ITask(c *gin.Context) {
	var T2IRequest task.T2IRequest
	if err := c.ShouldBindJSON(&T2IRequest); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	taskID, err := snowflake.GetID()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	var T2ITask task.T2ITask
	T2ITask.TaskID = taskID
	T2ITask.UserID = T2IRequest.UserID
	T2ITask.Prompt = T2IRequest.Text
	T2ITask.Status = task.T2IStatusPending
	T2ITask.CreatedAt = time.Now().Unix()
	T2IRequest.TaskID = taskID

	rabbitMQ, err := queue.GetT2IRabbitMQ()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get T2I message queue"})
		return
	}
	b, err := json.Marshal(T2ITask)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to serialize T2I task"})
		return
	}
	err = rabbitMQ.PublishT2ITask(b, T2IRequest.Priority)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to publish T2I task"})
		return
	}
	c.JSON(202, gin.H{"status": "task submitted"})

}
