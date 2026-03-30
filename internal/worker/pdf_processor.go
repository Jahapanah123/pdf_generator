package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/pdf"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
)

type PDFProcessor struct {
	repo      repository.JobRepository
	rmq       *queue.RabbitMQ
	generator *pdf.Generator
	logger    *slog.Logger
}

func NewPDFProcessor(
	repo repository.JobRepository,
	rmq *queue.RabbitMQ,
	generator *pdf.Generator,
	logger *slog.Logger,
) *PDFProcessor {
	return &PDFProcessor{
		repo:      repo,
		rmq:       rmq,
		generator: generator,
		logger:    logger,
	}
}

func (p *PDFProcessor) Process(ctx context.Context, msg *domain.JobMessage) error {
	p.logger.Info("processing job",
		slog.String("job_id", msg.JobID),
		slog.String("template", msg.TemplateName),
		slog.Int("retry", msg.RetryCount),
	)

	// Status: processing
	if err := p.repo.UpdateStatus(ctx, msg.JobID, domain.JobStatusProcessing, nil, nil); err != nil {
		return fmt.Errorf("update status to processing: %w", err)
	}
	p.publishEvent(ctx, msg.JobID, msg.UserID, domain.JobStatusProcessing, nil, nil)

	// Generate PDF
	filePath, err := p.generator.Generate(msg.JobID, msg.TemplateName, msg.Payload)
	if err != nil {
		errMsg := err.Error()
		_ = p.repo.UpdateStatus(ctx, msg.JobID, domain.JobStatusFailed, nil, &errMsg)
		p.publishEvent(ctx, msg.JobID, msg.UserID, domain.JobStatusFailed, nil, &errMsg)
		return fmt.Errorf("generate PDF: %w", err)
	}

	// Status: completed
	if err := p.repo.UpdateStatus(ctx, msg.JobID, domain.JobStatusCompleted, &filePath, nil); err != nil {
		return fmt.Errorf("update status to completed: %w", err)
	}
	p.publishEvent(ctx, msg.JobID, msg.UserID, domain.JobStatusCompleted, &filePath, nil)

	p.logger.Info("job completed",
		slog.String("job_id", msg.JobID),
		slog.String("file_path", filePath),
	)

	return nil
}

func (p *PDFProcessor) publishEvent(ctx context.Context, jobID, userID string, status domain.JobStatus, filePath, errMsg *string) {
	event := &domain.JobEvent{
		JobID:        jobID,
		UserID:       userID,
		Status:       status,
		FilePath:     filePath,
		ErrorMessage: errMsg,
		Timestamp:    time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("marshal event", slog.String("job_id", jobID), slog.Any("error", err))
		return
	}

	if err := p.rmq.PublishEvent(ctx, data); err != nil {
		p.logger.Error("publish event", slog.String("job_id", jobID), slog.Any("error", err))
	}
}

func (p *PDFProcessor) ParseMessage(body []byte) (*domain.JobMessage, error) {
	var msg domain.JobMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &msg, nil
}
