package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
	"github.com/jahapanah123/pdf_generator/internal/strategy"
)

const maxQueueSize = 10000

type PDFService struct {
	repo              repository.JobRepository
	rmq               *queue.RabbitMQ // ✅ Direct dependency, no feeder
	validatorRegistry *strategy.ValidatorRegistry
	logger            *slog.Logger
	maxRetries        int
}

func NewPDFService(
	repo repository.JobRepository,
	rmq *queue.RabbitMQ,
	validatorRegistry *strategy.ValidatorRegistry,
	logger *slog.Logger,
	maxRetries int,
) *PDFService {
	return &PDFService{
		repo:              repo,
		rmq:               rmq,
		validatorRegistry: validatorRegistry,
		logger:            logger,
		maxRetries:        maxRetries,
	}
}

func (s *PDFService) CreateJob(ctx context.Context, userID string, req *domain.CreateJobRequest) (*domain.JobResponse, error) {
	// Backpressure check
	size, err := s.rmq.QueueSize()
	if err != nil {
		s.logger.Warn("queue size check failed, proceeding", slog.Any("error", err))
	} else if size >= maxQueueSize {
		return nil, fmt.Errorf("%w: queue is full (%d messages)", domain.ErrQueueUnavailable, size)
	}

	// Validate payload
	var data map[string]any
	if err := json.Unmarshal(req.Payload, &data); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON payload", domain.ErrInvalidInput)
	}

	if err := s.validatorRegistry.Validate(req.TemplateName, data); err != nil {
		return nil, err
	}

	now := time.Now()
	jobID := uuid.New().String()

	job := &domain.Job{
		ID:           jobID,
		UserID:       userID,
		TemplateName: req.TemplateName,
		Payload:      req.Payload,
		Status:       domain.JobStatusPending,
		MaxRetries:   s.maxRetries,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Persist to DB
	if err := s.repo.Create(ctx, job); err != nil {
		s.logger.Error("failed to create job in DB",
			slog.String("job_id", jobID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Publish to RabbitMQ directly (NO feeder)
	jobMessage := &domain.JobMessage{
		JobID:        jobID,
		UserID:       userID,
		TemplateName: req.TemplateName,
		Payload:      req.Payload,
		RetryCount:   0,
		MaxRetries:   s.maxRetries,
	}

	body, err := json.Marshal(jobMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job message: %w", err)
	}

	if err := s.rmq.PublishJob(ctx, body); err != nil {
		errMsg := "failed to queue job"
		_ = s.repo.UpdateStatus(ctx, jobID, domain.JobStatusFailed, nil, &errMsg)
		return nil, fmt.Errorf("%w: %v", domain.ErrQueueUnavailable, err)
	}

	s.logger.Info("job created",
		slog.String("job_id", jobID),
		slog.String("user_id", userID),
		slog.String("template", req.TemplateName),
	)

	return &domain.JobResponse{
		ID:      jobID,
		Status:  domain.JobStatusPending,
		Message: "PDF generation job queued for processing",
	}, nil
}

func (s *PDFService) GetJobStatus(ctx context.Context, userID, jobID string) (*domain.JobStatusResponse, error) {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if job.UserID != userID {
		return nil, domain.ErrForbidden
	}

	return &domain.JobStatusResponse{
		ID:           job.ID,
		UserID:       job.UserID,
		Status:       job.Status,
		FilePath:     job.FilePath,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt,
		CompletedAt:  job.CompletedAt,
	}, nil
}

func (s *PDFService) ListJobs(ctx context.Context, userID string, limit, offset int) ([]*domain.JobStatusResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	jobs, err := s.repo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	responses := make([]*domain.JobStatusResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = &domain.JobStatusResponse{
			ID:           job.ID,
			UserID:       job.UserID,
			Status:       job.Status,
			FilePath:     job.FilePath,
			ErrorMessage: job.ErrorMessage,
			CreatedAt:    job.CreatedAt,
			CompletedAt:  job.CompletedAt,
		}
	}

	return responses, nil
}
