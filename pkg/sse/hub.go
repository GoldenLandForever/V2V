package sse

import (
	"sync"
)

// Hub 管理基于 topic 的 SSE 订阅者。
//
// 说明：
//   - 每个 topic 对应一组客户端通道（chan []byte），Hub 会把发布到该 topic 的消息广播
//     到所有订阅该 topic 的通道上。
//   - Hub 使用三个内部控制通道（subscribe/unsubscribe/publish）在单个 goroutine 中
//     串行化对 topics 数据结构的访问，从而避免在外部并发访问时出现竞态。
type Hub struct {
	// topics maps topic -> set of client channels
	// topics 保存 topic -> 客户端 channel 集合，channel 的所有者（SSE handler）负责关闭该 channel，Hub 仅负责向其发送消息。
	topics map[string]map[chan []byte]bool

	subscribe   chan subscription
	unsubscribe chan subscription
	publish     chan topicMessage

	mu sync.Mutex
}

type subscription struct {
	ch    chan []byte
	topic string
}

type topicMessage struct {
	topic string
	msg   []byte
}

var defaultHub *Hub

// NewHub 创建并返回一个新的 SSE Hub 实例。
//
// 注意：publish 通道具有缓冲（100），用于缓冲短时突发的发布操作，避免发布者短时间内被阻塞。
func NewHub() *Hub {
	return &Hub{
		topics:      make(map[string]map[chan []byte]bool),
		subscribe:   make(chan subscription),
		unsubscribe: make(chan subscription),
		publish:     make(chan topicMessage, 100),
	}
}

// SetDefaultHub sets the package-level default hub
func SetDefaultHub(h *Hub) {
	defaultHub = h
}

// GetHub returns the default hub (may be nil if not set)
func GetHub() *Hub {
	return defaultHub
}

// Run 启动 Hub 的事件循环，负责处理订阅、取消订阅与消息发布操作。
//
// 该方法应在单独的 goroutine 中运行，例如：
//
//	hub := sse.NewHub()
//	go hub.Run()
//
// 它保证对 topics 的所有修改都在同一 goroutine 中进行，避免并发读写冲突。
func (h *Hub) Run() {
	for {
		select {
		case s := <-h.subscribe:
			h.mu.Lock()
			subs, ok := h.topics[s.topic]
			if !ok {
				subs = make(map[chan []byte]bool)
				h.topics[s.topic] = subs
			}
			subs[s.ch] = true
			h.mu.Unlock()
		case s := <-h.unsubscribe:
			h.mu.Lock()
			if subs, ok := h.topics[s.topic]; ok {
				delete(subs, s.ch)
				if len(subs) == 0 {
					delete(h.topics, s.topic)
				}
			}
			h.mu.Unlock()
		case tm := <-h.publish:
			h.mu.Lock()
			if subs, ok := h.topics[tm.topic]; ok {
				for ch := range subs {
					select {
					case ch <- tm.msg:
					default:
						// drop if client not reading
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// PublishTopic 将消息发布到指定 topic 的所有订阅者。
//
// 说明：该调用会把消息写入 hub 的 publish 缓冲通道，由 Run 循环负责把消息分发到订阅者。
func (h *Hub) PublishTopic(topic string, msg []byte) {
	h.publish <- topicMessage{topic: topic, msg: msg}
}

// Subscribe 将指定通道注册为 topic 的订阅者。
//
// 使用约定：调用方应提供一个有缓冲的 channel（例如缓冲 16），并且在不再需要时负责取消订阅
// 并关闭通道。Hub 不会关闭订阅者提供的通道。
func (h *Hub) Subscribe(ch chan []byte, topic string) {
	h.subscribe <- subscription{ch: ch, topic: topic}
}

// Unsubscribe 取消某个通道对 topic 的订阅。
func (h *Hub) Unsubscribe(ch chan []byte, topic string) {
	h.unsubscribe <- subscription{ch: ch, topic: topic}
}
