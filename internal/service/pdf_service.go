package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
	"github.com/jahapanah123/pdf_generator/internal/validator"
)

const maxQueueSize = 10000 // backpressure threshold

	type pdfService struct {
		repo       repository.JobRepository
		rmq        *queue.RabbitMQ
		validator  *validator.Validator
		logger     *slog.Logger
		maxRetries int
	}

	func NewPDFService(
		repo repository.JobRepository,
		rmq *queue.RabbitMQ,
		v *validator.Validator,
		logger *slog.Logger,
		maxRetries int,
	) PDFService {
		return &pdfService{
			repo:       repo,
			rmq:        rmq,
			validator:  v,
			logger:     logger,
			maxRetries: maxRetries,
		}
	}

func (s *pdfService) CreateJob(ctx context.Context, userID string, req *domain.CreateJobRequest) (*domain.JobResponse, error) {
	// Validate
	if errs := s.validator.ValidateCreateJobRequest(req); errs != nil {
		return nil, &validationErr{errors: errs}
	}

	// Backpressure check
	size, err := s.rmq.QueueSize()
	if err != nil {
		s.logger.Warn("queue size check failed, proceeding", slog.Any("error", err))
	} else if size >= maxQueueSize {
		return nil, fmt.Errorf("%w: queue is full (%d messages)", domain.ErrQueueUnavailable, size)
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
		s.logger.Error("create job failed", slog.String("job_id", jobID), slog.Any("error", err))
		return nil, fmt.Errorf("create job: %w", err)
	}

	// Publish to RabbitMQ job queue
	msg := &domain.JobMessage{
		JobID:        jobID,
		UserID:       userID,
		TemplateName: req.TemplateName,
		Payload:      req.Payload,
		RetryCount:   0,
		MaxRetries:   s.maxRetries,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal job message: %w", err)
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

func (s *pdfService) GetJobStatus(ctx context.Context, userID, jobID string) (*domain.JobStatusResponse, error) {
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

func (s *pdfService) ListJobs(ctx context.Context, userID string, limit, offset int) ([]*domain.JobStatusResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	jobs, err := s.repo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
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

// validation error

type validationErr struct {
	errors []domain.ValidationError
}

func (e *validationErr) Error() string                              { return "validation failed" }
func (e *validationErr) ValidationErrors() []domain.ValidationError { return e.errors }

func IsValidationError(err error) ([]domain.ValidationError, bool) {
	var ve *validationErr
	if errors.As(err, &ve) {
		return ve.ValidationErrors(), true
	}
	return nil, false
}
