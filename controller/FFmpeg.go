package controller

import (
	"V2V/util"

	"github.com/gin-gonic/gin"
)

func FFmpegHandler(c *gin.Context) {
	// 调用 FFmpeg 相关功能的代码
	util.FFmpeg()
	c.JSON(200, gin.H{"message": "FFmpeg handler is not yet implemented"})
}
