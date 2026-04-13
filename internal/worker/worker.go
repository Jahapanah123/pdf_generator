package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
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

func (p *Pool) Start(ctx context.Context) error {
	msgs, err := p.rmq.ConsumeJobs(p.prefetch)
	if err != nil {
		return err
	}

	p.logger.Info("worker pool starting",
		slog.Int("concurrency", p.concurrency),
		slog.Int("prefetch", p.prefetch),
		slog.Int("max_retries", p.maxRetries),
	)

	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i, msgs)
	}

	return nil
}

func (p *Pool) Stop() {
	p.logger.Info("stopping worker pool...")
	p.wg.Wait()
	p.logger.Info("worker pool stopped")
}

func (p *Pool) worker(ctx context.Context, id int, msgs <-chan amqp.Delivery) {
	defer p.wg.Done()
	p.logger.Info("worker started", slog.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker shutting down", slog.Int("worker_id", id))
			return
		case msg, ok := <-msgs:
			if !ok {
				p.logger.Info("channel closed", slog.Int("worker_id", id))
				return
			}
			p.processMessage(ctx, id, msg)
		}
	}
}

func (p *Pool) processMessage(ctx context.Context, workerID int, msg amqp.Delivery) {
	start := time.Now()

	// Parse message
	jobMsg, err := p.processor.ParseMessage(msg.Body)
	if err != nil {
		p.logger.Error("parse message failed - sending to DLQ",
			slog.Int("worker_id", workerID),
			slog.Any("error", err),
		)
		// Malformed message → Nack without requeue → goes to DLQ
		_ = msg.Nack(false, false)
		return
	}

	// Get current retry count from message headers
	retryCount := getRetryCount(msg)

	p.logger.Info("processing job",
		slog.Int("worker_id", workerID),
		slog.String("job_id", jobMsg.JobID),
		slog.String("template", jobMsg.TemplateName),
		slog.Int("retry_attempt", retryCount),
	)

	// Process the job
	if err := p.processor.Process(ctx, jobMsg); err != nil {
		p.logger.Error("job processing failed",
			slog.Int("worker_id", workerID),
			slog.String("job_id", jobMsg.JobID),
			slog.Int("retry_attempt", retryCount),
			slog.Any("error", err),
			slog.Duration("duration", time.Since(start)),
		)

		// Check if we should retry
		if retryCount < p.maxRetries {
			p.logger.Info("job will be retried via DLX",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("retry_attempt", retryCount),
				slog.Int("max_retries", p.maxRetries),
			)

			// Nack with requeue=false
			// RabbitMQ will route this to retry queue via DLX
			// After TTL expires, it comes back to main queue with x-death header incremented
			_ = msg.Nack(false, false)
		} else {
			// Max retries exceeded
			p.logger.Warn("max retries exceeded - job failed permanently",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("total_attempts", retryCount+1),
			)

			// Update DB status to failed
			errMsg := fmt.Sprintf("max retries (%d) exceeded: %v", p.maxRetries, err)
			_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)

			// Ack to remove from queue (don't send to DLQ as we've handled it)
			_ = msg.Ack(false)
		}
		return
	}

	// Success - acknowledge
	if err := msg.Ack(false); err != nil {
		p.logger.Error("ack failed",
			slog.String("job_id", jobMsg.JobID),
			slog.Any("error", err),
		)
	}

	p.logger.Info("job completed successfully",
		slog.Int("worker_id", workerID),
		slog.String("job_id", jobMsg.JobID),
		slog.Duration("duration", time.Since(start)),
	)
}

// getRetryCount extracts retry count from RabbitMQ's x-death header
// RabbitMQ automatically tracks how many times a message has been dead-lettered
func getRetryCount(msg amqp.Delivery) int {
	if msg.Headers == nil {
		return 0
	}

	// x-death is an array of death records
	deaths, ok := msg.Headers["x-death"].([]interface{})
	if !ok || len(deaths) == 0 {
		return 0
	}

	// Get the first death record (most recent)
	death, ok := deaths[0].(amqp.Table)
	if !ok {
		return 0
	}

	// count field tells us how many times this message was dead-lettered
	count, ok := death["count"].(int64)
	if !ok {
		return 0
	}

	return int(count)
}
