package ws

import (
	"context"
	"log/slog"
	"sync"
)

// MessageHandler 处理客户端上行帧；由上层注入，便于后续替换为 gRPC 调用 Agent。
type MessageHandler func(ctx context.Context, c *Client, f Frame)

// Hub 管理网关持有的全部长连接，按 sessionID 建索引，支持同会话多端订阅。
type Hub struct {
	mu        sync.RWMutex
	sessions  map[string]map[*Client]struct{}
	onMessage MessageHandler
	log       *slog.Logger
}

func NewHub(log *slog.Logger, onMessage MessageHandler) *Hub {
	return &Hub{
		sessions:  make(map[string]map[*Client]struct{}),
		onMessage: onMessage,
		log:       log,
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[c.SessionID] == nil {
		h.sessions[c.SessionID] = make(map[*Client]struct{})
	}
	h.sessions[c.SessionID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.sessions[c.SessionID]
	if !ok {
		return
	}
	if _, ok := conns[c]; !ok {
		return
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.sessions, c.SessionID)
	}
}

// Broadcast 向某会话的全部在线连接推送帧。
func (h *Hub) Broadcast(sessionID string, f Frame) {
	h.mu.RLock()
	conns := make([]*Client, 0, len(h.sessions[sessionID]))
	for c := range h.sessions[sessionID] {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		c.Send(f)
	}
}

// SetMessageHandler 在 Hub 构造后注入业务处理函数，避免 handler 与 hub 循环依赖。
func (h *Hub) SetMessageHandler(fn MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMessage = fn
}

// Dispatch 将上行帧交给业务处理器。
func (h *Hub) Dispatch(ctx context.Context, c *Client, f Frame) {
	if h.onMessage == nil {
		return
	}
	h.onMessage(ctx, c, f)
}

// Count 当前在线连接数，用于指标暴露。
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, conns := range h.sessions {
		n += len(conns)
	}
	return n
}
