package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type PDFHandler struct {
	service PDFService
	logger  *slog.Logger
}

func NewPDFHandler(service PDFService, logger *slog.Logger) *PDFHandler {
	return &PDFHandler{
		service: service,
		logger:  logger,
	}
}

func (h *PDFHandler) CreateJob(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req domain.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "invalid request body"))
		return
	}

	resp, err := h.service.CreateJob(c.Request.Context(), userID.(string), &req)
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

	resp, err := h.service.GetJobStatus(c.Request.Context(), userID.(string), jobID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PDFHandler) ListJobs(c *gin.Context) {
	userID, _ := c.Get("user_id")

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	resp, err := h.service.ListJobs(c.Request.Context(), userID.(string), limit, offset)
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

func (h *PDFHandler) DownloadJob(c *gin.Context) {
	userID, _ := c.Get("user_id")
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "job id is required"))
		return
	}

	resp, err := h.service.GetJobStatus(c.Request.Context(), userID.(string), jobID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	if resp.Status != domain.JobStatusCompleted {
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, "job is not completed yet"))
		return
	}

	if resp.FilePath == nil || *resp.FilePath == "" {
		c.JSON(http.StatusNotFound,
			domain.NewAPIError(http.StatusNotFound, "pdf file not found"))
		return
	}

	c.FileAttachment(*resp.FilePath, jobID+".pdf")
}

func (h *PDFHandler) handleError(c *gin.Context, err error) {
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
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(http.StatusBadRequest,
			domain.NewAPIError(http.StatusBadRequest, err.Error()))
	default:
		h.logger.Error("unhandled error", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError,
			domain.NewAPIError(http.StatusInternalServerError, "internal server error"))
	}
}
