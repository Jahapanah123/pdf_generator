package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jahapanah123/pdf-generator/internal/domain"
	"github.com/jahapanah123/pdf-generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf-generator/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Pool struct {
	rmq         *queue.RabbitMQ
	processor   *PDFProcessor
	repo        repository.JobRepository
	logger      *slog.Logger
	concurrency int
	prefetch    int
	maxRetries  int
	wg          sync.WaitGroup
}

func NewPool(
	rmq *queue.RabbitMQ,
	processor *PDFProcessor,
	repo repository.JobRepository,
	logger *slog.Logger,
	concurrency int,
	prefetch int,
	maxRetries int,
) *Pool {
	return &Pool{
		rmq:         rmq,
		processor:   processor,
		repo:        repo,
		logger:      logger,
		concurrency: concurrency,
		prefetch:    prefetch,
		maxRetries:  maxRetries,
	}
}

func (p *Pool) processMessage(ctx context.Context, workerID int, msg amqp.Delivery) {
	start := time.Now()

	jobMsg, err := p.processor.ParseMessage(msg.Body)
	if err != nil {
		p.logger.Error("parse message failed - sending to DLQ",
			slog.Int("worker_id", workerID),
			slog.Any("error", err),
		)
		_ = msg.Nack(false, false)
		return
	}

	retryCount := getRetryCount(msg)

	p.logger.Info("processing job",
		slog.Int("worker_id", workerID),
		slog.String("job_id", jobMsg.JobID),
		slog.Int("retry_attempt", retryCount),
	)

	if err := p.processor.Process(ctx, jobMsg); err != nil {
		p.logger.Error("job processing failed",
			slog.Int("worker_id", workerID),
			slog.String("job_id", jobMsg.JobID),
			slog.Int("retry_attempt", retryCount),
			slog.Any("error", err),
			slog.Duration("duration", time.Since(start)),
		)

		if retryCount < p.maxRetries {
			p.logger.Info("job will be retried via DLX",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("retry_attempt", retryCount),
				slog.Int("max_retries", p.maxRetries),
			)
			_ = msg.Nack(false, false)
		} else {
			p.logger.Warn("max retries exceeded - job failed permanently",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("total_attempts", retryCount+1),
			)
			errMsg := fmt.Sprintf("max retries (%d) exceeded: %v", p.maxRetries, err)
			_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)
			_ = msg.Ack(false)
		}
		return
	}

	if err := msg.Ack(false); err != nil {
		p.logger.Error("ack failed",
			slog.String("job_id", jobMsg.JobID),
			slog.Any("error", err),
		)
		return
	}

	p.logger.Info("job completed successfully",
		slog.Int("worker_id", workerID),
		slog.String("job_id", jobMsg.JobID),
		slog.Duration("duration", time.Since(start)),
	)
}
