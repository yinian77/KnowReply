package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// ListDocuments 知识文档列表，按租户隔离。
func (h *Handler) ListDocuments(c *gin.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
		return
	}

	limit := defaultPageLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = min(n, maxPageLimit)
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	docs, err := h.deps.Store.ListDocuments(ctx, claims.TenantID, c.Query("status"), limit, offset)
	if err != nil {
		h.deps.Log.Error("list documents failed", "err", err, "tenantId", claims.TenantID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list documents failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": docs, "limit": limit, "offset": offset})
}
