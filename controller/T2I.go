package controller

import (
	"V2V/dao/store"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"V2V/task"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmitT2ITask 提交文本生成图片任务
// @Summary 提交文本生成图片任务
// @Description 接收任务ID，创建一个新的 T2I 任务并返回任务 ID（输入从V2T得到的任务ID）
// @Tags T2I
// @Accept json
// @Produce json
// @Param request body task.T2IRequest true "T2I 任务请求"
// @Success 202 {object} map[string]interface{} "{"task_id": 123456, "status": "task submitted"}"
// @Failure 400 {object} map[string]string "invalid request"
// @Failure 500 {object} map[string]string "server error"
// @Router /T2I [post]
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
	key := "user:0:task:" + strconv.FormatUint(T2IRequest.TaskID, 10)
	hash, err := store.GetRedis().HGetAll(key).Result()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	var T2ITask task.T2ITask
	T2ITask.TaskID = taskID
	T2ITask.UserID = T2IRequest.UserID
	T2ITask.Prompt = hash["result"]
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
	c.JSON(202, gin.H{"task_id": strconv.FormatUint(taskID, 10), "status": "task submitted"})

}
