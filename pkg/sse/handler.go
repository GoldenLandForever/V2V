package sse

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServeSSE 处理 SSE（Server-Sent Events）连接
// @Summary 订阅服务器事件流（SSE）
// @Description 建立 SSE 长连接以接收服务端推送的事件。需要通过查询参数 `userid` 指定订阅的主题/用户 ID，例如 `/events?userid=12345`。会在V2T，T2I任务结束后推送消息。
// @Tags SSE
// @Accept  json
// @Produce text/event-stream
// @Param userid query string true "User ID / topic to subscribe"
// @Success 200 {string} string "event stream"
// @Failure 400 {string} string "missing topic"
// @Failure 500 {string} string "server error"
// @Router /events [get]
func ServeSSE(c *gin.Context) {
	topic := c.Query("userid")
	if topic == "" {
		c.String(http.StatusBadRequest, "missing topic")
		return
	}

	h := GetHub()
	if h == nil {
		c.String(http.StatusInternalServerError, "sse hub not initialized")
		return
	}

	// 设置 SSE 必要的响应头，确保浏览器或代理以流式方式处理
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming unsupported")
		return
	}

	// 创建每个连接专用的消息通道（缓冲 16），用于接收 hub 转发的事件。
	// 注意：调用方（handler）负责在不再使用时取消订阅并关闭通道。
	msgCh := make(chan []byte, 16)
	// 订阅 topic
	h.Subscribe(msgCh, topic)
	defer h.Unsubscribe(msgCh, topic)

	notify := c.Request.Context().Done()
	// 发送一个注释（: connected）作为初次握手 / 保活 ping，部分代理需要保持连接活跃
	fmt.Fprintf(c.Writer, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-notify:
			return
		case msg := <-msgCh:
			// 将消息以 SSE 格式发送（data: <payload>\n\n）
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(msg))
			//记录日志
			log.Printf("Sent message to topic %s: %s", topic, string(msg))
			flusher.Flush()
		}
	}
}
