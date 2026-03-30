package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/service"
)

type PDFHandler struct {
	svc    service.PDFService
	logger *slog.Logger
}

func NewPDFHandler(svc service.PDFService, logger *slog.Logger) *PDFHandler {
	return &PDFHandler{svc: svc, logger: logger}
}

func (h *PDFHandler) CreateJob(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req domain.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "invalid request body"))
		return
	}

	resp, err := h.svc.CreateJob(c.Request.Context(), userID.(string), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

func (h *PDFHandler) GetJobStatus(c *gin.Context) {
	userID, _ := c.Get("user_id")
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "job id is required"))
		return
	}

	resp, err := h.svc.GetJobStatus(c.Request.Context(), userID.(string), jobID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PDFHandler) ListJobs(c *gin.Context) {
	userID, _ := c.Get("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.svc.ListJobs(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":   resp,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *PDFHandler) handleError(c *gin.Context, err error) {
	if valErrs, ok := service.IsValidationError(err); ok {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "validation failed", valErrs...))
		return
	}

	switch {
	case errors.Is(err, domain.ErrJobNotFound):
		c.JSON(http.StatusNotFound,
			domain.NewAPIError(http.StatusNotFound, "job not found"))
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden,
			domain.NewAPIError(http.StatusForbidden, "access denied"))
	case errors.Is(err, domain.ErrQueueUnavailable):
		c.JSON(http.StatusServiceUnavailable,
			domain.NewAPIError(http.StatusServiceUnavailable, "service temporarily unavailable"))
	default:
		h.logger.Error("unhandled error", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError,
			domain.NewAPIError(http.StatusInternalServerError, "internal server error"))
	}
}
