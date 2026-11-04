package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/yinian77/KnowReply/knowreply-gateway/internal/store"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 上线前改为按域名白名单校验，避免任意站点建立长连接。
	CheckOrigin: func(r *http.Request) bool { return true },
}

type createSessionRequest struct {
	UserID string `json:"userId"`
}

// CreateSession 创建会话，返回业务会话 ID（s_ 前缀）。
func (h *Handler) CreateSession(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	var req createSessionRequest
	_ = c.ShouldBindJSON(&req) // 允许空 body
	userID := req.UserID
	if userID == "" {
		userID = claims.UserID
	}

	sessionID := newID("s_")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.deps.Store.CreateSession(ctx, sessionID, claims.TenantID, userID); err != nil {
		h.deps.Log.Error("create session failed", "err", err, "tenantId", claims.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create session failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"sessionId": sessionID, "status": "active"})
}

// ChatWS 升级为长连接；鉴权信息来自 token，sessionId 必须归属该租户。
func (h *Handler) ChatWS(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	owned, err := h.deps.Store.SessionOwned(ctx, sessionID, claims.TenantID)
	cancel()

	if err != nil {
		h.deps.Log.Error("check session owner failed", "err", err, "sessionId", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "check session failed"})
		return
	}
	if !owned {
		c.JSON(http.StatusForbidden, gin.H{"error": "session not found or not owned by tenant"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.deps.Log.Warn("ws upgrade failed", "err", err, "sessionId", sessionID)
		return
	}

	client := ws.NewClient(h.deps.Hub, conn, h.deps.Log, sessionID, claims.TenantID, claims.UserID)
	h.deps.Hub.Register(client)

	h.deps.Log.Info("ws connected",
		"sessionId", sessionID, "tenantId", claims.TenantID, "userId", claims.UserID, "online", h.deps.Hub.Count())

	go client.WritePump()
	client.ReadPump(h.deps.BaseCtx, h.deps.WSReadLimit)
}

// HandleUserMessage 处理一条上行消息：落库 → 取回复 → 落库。
// 当前为 echo 桩，P0 后半段替换为 Agent.ChatStream(gRPC) 逐 token 转发。
func (h *Handler) HandleUserMessage(baseCtx context.Context, client *ws.Client, f ws.Frame) {
	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()

	if f.SessionID != client.SessionID {
		client.Send(ws.Frame{Type: ws.TypeError, Code: "session_mismatch", Message: "sessionId does not match connection"})
		return
	}

	userMsgID := f.MsgID
	if userMsgID == "" {
		userMsgID = newID("c_")
	}

	if err := h.deps.Store.InsertMessage(ctx, store.Message{
		ID:        userMsgID,
		SessionID: client.SessionID,
		TenantID:  client.TenantID,
		Role:      "user",
		Content:   f.Content,
		Intent:    sql.NullString{},
	}); err != nil {
		h.deps.Log.Error("persist user message failed", "err", err, "sessionId", client.SessionID)
		client.Send(ws.Frame{Type: ws.TypeError, Code: "persist_failed", Message: "save message failed"})
		return
	}

	replyID := newID("a_")
	replyText := "网关已收到：" + f.Content + "（Agent 未接入，当前为占位回复）"
	client.Send(ws.Frame{Type: ws.TypeAssistantDelta, MsgID: replyID, Delta: replyText})
	client.Send(ws.Frame{Type: ws.TypeAssistantDone, MsgID: replyID, Citations: []string{}, NeedHuman: false})

	if err := h.deps.Store.InsertMessage(ctx, store.Message{
		ID:            replyID,
		SessionID:     client.SessionID,
		TenantID:      client.TenantID,
		Role:          "assistant",
		Content:       replyText,
		CitationsJSON: store.MarshalCitations(nil),
	}); err != nil {
		h.deps.Log.Error("persist assistant message failed", "err", err, "sessionId", client.SessionID)
	}

	if err := h.deps.Store.TouchSession(ctx, client.SessionID); err != nil {
		h.deps.Log.Warn("touch session failed", "err", err, "sessionId", client.SessionID)
	}
}
