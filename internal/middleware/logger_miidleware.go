package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		status := c.Writer.Status()
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("body_size", c.Writer.Size()),
		}

		if rid, exists := c.Get(RequestIDKey); exists {
			attrs = append(attrs, slog.String("request_id", rid.(string)))
		}
		if uid, exists := c.Get("user_id"); exists {
			attrs = append(attrs, slog.String("user_id", uid.(string)))
		}

		switch {
		case status >= 500:
			logger.LogAttrs(c.Request.Context(), slog.LevelError, "request", attrs...)
		case status >= 400:
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "request", attrs...)
		default:
			logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "request", attrs...)
		}
	}
}
