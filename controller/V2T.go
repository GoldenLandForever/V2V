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

// SubmitV2TTask 提交视频转文字任务
// @Summary 提交视频转文字任务
// @Description 接收视频URL，创建一个新的 V2T 任务并返回任务 ID （目前没有登陆注册，所以就只要求输入视频链接）
// @Tags V2T
// @Accept json
// @Produce json
// @Param request body task.V2TRequest true "V2T 任务请求"
// @Success 202 {object} map[string]interface{} "{"task_id": "123456", "status": "submitted"}"
// @Failure 400 {object} map[string]string "invalid request"
// @Failure 500 {object} map[string]string "server error"
// @Router /V2T [post]
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
	//把任务ID转成字符串返回给前端
	c.JSON(202, gin.H{"task_id": strconv.FormatUint(taskID, 10), "status": "submitted"})
}

// GetV2TTaskResult 获取 V2T 任务结果
// @Summary 获取 V2T 任务结果
// @Description 通过任务 ID 获取 V2T 任务的当前状态和结果 （输入任务ID即可）
// @Tags V2T
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} map[string]interface{} "{"task_id": 123456, "status": "completed", "result": "..."}"
// @Failure 404 {object} map[string]string "task not found"
// @Failure 500 {object} map[string]string "server error"
// @Router /V2T/{task_id} [get]
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
	result.TaskID = taskID
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

// LoraText 更新任务 Lora 文本
// @Summary 更新任务 Lora 文本
// @Description 为指定任务更新 Lora 相关的文本提示词 （同时输入任务ID与更新后的提示词即可）
// @Tags V2T
// @Accept json
// @Produce json
// @Param request body task.LoraTextRequest true "Lora 文本更新请求"
// @Success 200 {object} map[string]interface{} "{"task_id": 123456, "status": "task updated"}"
// @Failure 400 {object} map[string]string "invalid request"
// @Failure 500 {object} map[string]string "server error"
// @Router /V2T/LoraText [post]
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
	c.JSON(200, gin.H{"task_id": LoraTextReq.TaskID, "status": "task updated"})
}
