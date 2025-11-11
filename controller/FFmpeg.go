package controller

import (
	"V2V/util"

	"github.com/gin-gonic/gin"
)

// FFmpegHandler FFmpeg 处理器
// @Summary FFmpeg 处理器
// @Description 处理 FFmpeg 相关的视频处理任务 （在一切视频都生成后输入任务I2V的ID进行拼接）
// @Tags FFmpeg
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} map[string]string "{"message": "..."}"
// @Router /FFmpeg/{task_id} [get]
func FFmpegHandler(c *gin.Context) {
	// 调用 FFmpeg 相关功能的代码，返回输出文件路径（位于 ./public/videos）
	taskID := c.Param("task_id")
	outPath := util.FFmpeg(taskID)

	// 构造前端可访问的 URL（包含 scheme）
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	videoURL := scheme + "://" + host + "/videos/" + taskID + ".mp4"

	c.JSON(200, gin.H{"message": "video ready", "video_url": videoURL, "path": outPath})
}
