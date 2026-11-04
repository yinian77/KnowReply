package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yinian77/KnowReply/knowreply-gateway/internal/auth"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/store"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/ws"
)

const ctxClaimsKey = "claims"

// Deps 网关 HTTP / WS 处理所需的依赖集合。
type Deps struct {
	Store       *store.Store
	Signer      *auth.Signer
	Hub         *ws.Hub
	Log         *slog.Logger
	WSReadLimit int64
	// BaseCtx 随进程生命周期取消，用于脱离 HTTP 请求上下文的 WS 后台读写。
	BaseCtx context.Context
	// Ready 就绪检查：MySQL + Redis 连通性，由 server 注入。
	Ready func(ctx context.Context) error
}

type Handler struct {
	deps Deps
}

func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}

// RequireAuth 校验 Bearer token；WS 场景允许从 query 参数取 token。
func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := h.deps.Signer.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Next()
	}
}

func claimsFrom(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ctxClaimsKey)
	if !ok {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(b)
}
