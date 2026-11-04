package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type tokenRequest struct {
	TenantID uint64 `json:"tenantId" binding:"required"`
	UserID   string `json:"userId" binding:"required"`
}

// IssueToken 签发访问令牌。
// 注意：当前为联调桩实现，直接信任请求体里的 tenantId；
// 正式环境必须改为校验账号密码 / 短信验证码后再签发。
func (h *Handler) IssueToken(c *gin.Context) {
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId and userId are required"})
		return
	}

	token, exp, err := h.deps.Signer.Sign(req.TenantID, req.UserID)
	if err != nil {
		h.deps.Log.Error("sign token failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sign token failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": exp.UTC().Format(time.RFC3339),
		"expiresIn": int64(time.Until(exp).Seconds()),
	})
}
