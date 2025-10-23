package api

import (
	"V2V/queue"
	"V2V/store"
	"V2V/task"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	queue queue.MessageQueue
	store store.TaskStore
}

func NewHandler(q queue.MessageQueue, s store.TaskStore) *Handler {
	return &Handler{queue: q, store: s}
}

func (h *Handler) SubmitVideoTask(c *gin.Context) {
	var req struct {
		VideoURL string `json:"video_url" binding:"required,url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskID := uuid.New().String()
	newTask := task.VideoTask{
		TaskID:    taskID,
		VideoURL:  req.VideoURL,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}

	// 保存初始状态
	if err := h.store.SetTask(newTask); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	// 发送到消息队列
	if err := h.queue.Publish(newTask); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "queue error"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID,
		"status":  "submitted",
	})
}

func (h *Handler) GetTaskResult(c *gin.Context) {
	taskID := c.Param("task_id")
	t, err := h.store.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	response := gin.H{
		"task_id": t.TaskID,
		"status":  t.Status,
	}

	if t.Status == "completed" {
		response["result"] = t.Result
	}

	c.JSON(http.StatusOK, response)
}
