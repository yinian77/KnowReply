package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	sendBufferSize = 64
)

// Client 一条 WS 长连接。写操作只允许在 writePump 协程内执行。
type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	SessionID string
	TenantID  uint64
	UserID    string

	send      chan []byte
	done      chan struct{}
	log       *slog.Logger
	closeOnce sync.Once
}

func NewClient(hub *Hub, conn *websocket.Conn, log *slog.Logger, sessionID string, tenantID uint64, userID string) *Client {
	return &Client{
		Hub:       hub,
		Conn:      conn,
		SessionID: sessionID,
		TenantID:  tenantID,
		UserID:    userID,
		send:      make(chan []byte, sendBufferSize),
		done:      make(chan struct{}),
		log:       log,
	}
}

// Send 非阻塞投递；缓冲打满说明客户端消费过慢，丢弃并记录，避免拖垮网关。
// 注意：只关闭 done、不关闭 send，避免并发 Send 触发 send on closed channel。
func (c *Client) Send(f Frame) {
	b, err := json.Marshal(f)
	if err != nil {
		c.log.Error("marshal frame failed", "err", err, "type", f.Type)
		return
	}
	select {
	case <-c.done:
		return
	default:
	}

	select {
	case c.send <- b:
	default:
		c.log.Warn("send buffer full, frame dropped",
			"sessionId", c.SessionID, "type", f.Type, "buffered", len(c.send))
	}
}

// ReadPump 只负责读，退出时注销并关闭连接。
func (c *Client) ReadPump(ctx context.Context, readLimit int64) {
	defer func() {
		c.Hub.Unregister(c)
		c.Close()
	}()

	c.Conn.SetReadLimit(readLimit)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Info("ws closed unexpectedly", "sessionId", c.SessionID, "err", err)
			}
			return
		}

		var f Frame
		if err := json.Unmarshal(payload, &f); err != nil {
			c.Send(Frame{Type: TypeError, Code: "bad_frame", Message: "invalid json frame"})
			continue
		}
		if f.Type != TypeUserMessage {
			c.Send(Frame{Type: TypeError, Code: "unsupported_type", Message: "only user_message is accepted"})
			continue
		}
		if f.Content == "" {
			c.Send(Frame{Type: TypeError, Code: "empty_content", Message: "content must not be empty"})
			continue
		}
		if f.SessionID == "" {
			f.SessionID = c.SessionID
		}
		c.Hub.Dispatch(ctx, c, f)
	}
}

// WritePump 串行化写，保持心跳。
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-c.done:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case msg := <-c.send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.Conn.Close()
	})
}
