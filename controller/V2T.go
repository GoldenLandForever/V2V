package controller

import (
	"V2V/dao/store"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"V2V/task"
	"encoding/json"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

func SubmitV2TTask(c *gin.Context) {
	//解析前端请求并提交任务
	var taskReq *task.V2TRequest
	if err := c.ShouldBindJSON(&taskReq); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	//创建新任务
	//获得任务ID

	taskID, err := snowflake.GetID()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	V2TTask := task.V2TTask{
		TaskID:     taskID,
		Status:     task.StatusPending,
		Result:     "",
		CreatedAt:  taskReq.CreatedAt,
		UpdatedAt:  taskReq.CreatedAt,
		V2TRequest: *taskReq,
	}
	err = store.V2TTask(V2TTask)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to store task"})
		return
	}
	//将任务发送到消息队列
	rabbitMQ, err := queue.GetRabbitMQ()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get message queue"})
		return
	}
	b, err := json.Marshal(V2TTask)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to serialize task"})
		return
	}
	err = rabbitMQ.Publish([]byte(b), V2TTask.Priority)
	if err != nil {
		//回溯redis
		c.JSON(500, gin.H{"error": "failed to publish task"})
		return
	}
	c.JSON(202, gin.H{"task_id": V2TTask.TaskID, "status": "submitted"})
}

func GetV2TTaskResult(c *gin.Context) {
	//获取任务结果
	taskID := c.Param("task_id")
	key := "user:0:task:" + taskID
	log.Printf("Raw taskID: %q", taskID)
	hash, err := store.GetRedis().HGetAll(key).Result()
	if err != nil {
		log.Printf("Failed to get task %s: %v", taskID, err)
		c.JSON(404, gin.H{"error": "task not found"})
		return
	}
	var result task.V2TResponse
	result.Status = hash["status"]
	// parse task_id (stored as a string in Redis) into uint64

	if err != nil {
		log.Printf("Invalid task_id for task %s: %v", taskID, err)
		c.JSON(500, gin.H{"error": "invalid task id"})
		return
	}
	result.TaskID = 1
	result.Result = hash["result"]
	// result.UpdatedAt = hash["updated_at"]
	log.Printf("Fetched task %s: status=%s", taskID, result.Status)
	c.JSON(200, gin.H{
		"task_id":    result.TaskID,
		"status":     result.Status,
		"result":     result.Result,
		"updated_at": result.UpdatedAt,
	})
}

func LoraText(c *gin.Context) {
	var LoraTextReq task.LoraTextRequest
	if err := c.ShouldBindJSON(&LoraTextReq); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	//对应任务更新到redis
	key := "user:" + strconv.FormatUint(LoraTextReq.UserID, 10) + ":task:" + strconv.FormatUint(LoraTextReq.TaskID, 10)
	//对应key查找redis,修改result字段
	err := store.GetRedis().HSet(key, "result", LoraTextReq.Result).Err()
	if err != nil {
		log.Printf("Failed to update task %d in redis: %v", LoraTextReq.TaskID, err)
		c.JSON(500, gin.H{"error": "failed to update task"})
		return
	}
	c.JSON(200, gin.H{"status": "task updated"})
}
