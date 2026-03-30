package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/sse"
)

type SSEHandler struct {
	broker *sse.Broker
	logger *slog.Logger
}

func NewSSEHandler(broker *sse.Broker, logger *slog.Logger) *SSEHandler {
	return &SSEHandler{broker: broker, logger: logger}
}

func (h *SSEHandler) Stream(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized,
			domain.NewAPIError(http.StatusUnauthorized, "unauthorized"))
		return
	}

	// SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	client, err := h.broker.Subscribe(userID.(string))
	if err != nil {
		h.logger.Error("SSE subscribe failed", slog.Any("error", err))
		c.JSON(http.StatusServiceUnavailable,
			domain.NewAPIError(http.StatusServiceUnavailable, "too many connections"))
		return
	}
	defer h.broker.Unsubscribe(client)

	// Send connected event
	c.SSEvent("connected", gin.H{"message": "SSE connection established"})
	c.Writer.Flush()

	// Stream loop
	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("marshal SSE event", slog.Any("error", err))
				return true
			}
			c.SSEvent("job_update", string(data))
			c.Writer.Flush()
			return true
		}
	})
}
