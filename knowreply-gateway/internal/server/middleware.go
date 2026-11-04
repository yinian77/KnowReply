package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogger 结构化访问日志，WS 升级请求也会记录一次。
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		slog.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latencyMs", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}
