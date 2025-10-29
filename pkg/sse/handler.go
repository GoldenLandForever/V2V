package sse

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServeSSE 处理 SSE（Server-Sent Events）连接，期望的查询参数：?topic=<task_id>
// 示例：/events?topic=12345
//
// 行为说明：
// - 为浏览器/客户端设置标准的 SSE headers，并以流式方式写入事件。
// - 每个连接会创建一个缓冲通道（默认缓冲 16），并订阅指定的 topic；当连接关闭时会自动取消订阅。
// - 如果客户端断线重连，SSE 本身不保证补发丢失的事件；如需可靠投递应在 payload 中加入序列号并在服务端保存历史记录供客户端补取。
func ServeSSE(c *gin.Context) {
	topic := c.Query("topic")
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
			flusher.Flush()
		}
	}
}
