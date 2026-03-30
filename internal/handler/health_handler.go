package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/sse"
)

type HealthHandler struct {
	broker *sse.Broker
}

func NewHealthHandler(broker *sse.Broker) *HealthHandler {
	return &HealthHandler{broker: broker}
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":          "healthy",
		"service":         "pdf-generator",
		"sse_connections": h.broker.ActiveConnections(),
	})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
