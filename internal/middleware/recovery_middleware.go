package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						msg := strings.ToLower(se.Error())
						if strings.Contains(msg, "broken pipe") ||
							strings.Contains(msg, "connection reset") {
							brokenPipe = true
						}
					}
				}

				logger.Error("panic recovered",
					slog.Any("error", err),
					slog.String("path", c.Request.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)

				if brokenPipe {
					c.Abort()
					return
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError,
					domain.NewAPIError(http.StatusInternalServerError, "internal server error"),
				)
			}
		}()
		c.Next()
	}
}
