package worker

import (
	"context"
	"encoding/json"
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

	jobMsg, err := p.processor.ParseMessage(msg.Body)
	if err != nil {
		p.logger.Error("parse message failed",
			slog.Int("worker_id", workerID),
			slog.Any("error", err),
		)
		// Malformed message — nack without requeue, DLX sends to DLQ
		_ = msg.Nack(false, false)
		return
	}

	if err := p.processor.Process(ctx, jobMsg); err != nil {
		p.logger.Error("process failed",
			slog.Int("worker_id", workerID),
			slog.String("job_id", jobMsg.JobID),
			slog.Int("retry_count", jobMsg.RetryCount),
			slog.Any("error", err),
			slog.Duration("duration", time.Since(start)),
		)

		// Ack current message — we handle retry/DLQ manually
		_ = msg.Ack(false)

		// Increment retry in DB
		_ = p.repo.IncrementRetry(ctx, jobMsg.JobID)

		nextRetry := jobMsg.RetryCount + 1

		if nextRetry <= p.maxRetries {
			// Send to delayed retry queue
			jobMsg.RetryCount = nextRetry
			retryBody, marshalErr := json.Marshal(jobMsg)
			if marshalErr != nil {
				p.logger.Error("marshal retry message failed",
					slog.String("job_id", jobMsg.JobID),
					slog.Any("error", marshalErr),
				)
				errMsg := "failed to marshal retry message"
				_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)
				return
			}

			if pubErr := p.rmq.PublishToRetry(ctx, nextRetry, retryBody); pubErr != nil {
				p.logger.Error("publish to retry queue failed",
					slog.String("job_id", jobMsg.JobID),
					slog.Int("retry_level", nextRetry),
					slog.Any("error", pubErr),
				)
				errMsg := "failed to publish to retry queue: " + pubErr.Error()
				_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)
				return
			}

			p.logger.Info("job sent to retry queue",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("retry_level", nextRetry),
				slog.String("retry_queue", p.rmq.RetryQueueName(nextRetry)),
			)
		} else {
			// All retries exhausted — send to DLQ explicitly + mark failed in DB
			errMsg := "all retries exhausted: " + err.Error()
			_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)

			// Publish to DLQ so the message is preserved for inspection
			dlqBody, marshalErr := json.Marshal(jobMsg)
			if marshalErr != nil {
				p.logger.Error("marshal DLQ message failed",
					slog.String("job_id", jobMsg.JobID),
					slog.Any("error", marshalErr),
				)
				return
			}

			if dlqErr := p.rmq.PublishToDLQ(ctx, dlqBody); dlqErr != nil {
				p.logger.Error("publish to DLQ failed",
					slog.String("job_id", jobMsg.JobID),
					slog.Any("error", dlqErr),
				)
			} else {
				p.logger.Warn("job sent to DLQ",
					slog.String("job_id", jobMsg.JobID),
					slog.Int("total_attempts", nextRetry),
				)
			}
		}
		return
	}

	// Success
	if err := msg.Ack(false); err != nil {
		p.logger.Error("ack failed",
			slog.String("job_id", jobMsg.JobID),
			slog.Any("error", err),
		)
	}

	p.logger.Info("job done",
		slog.Int("worker_id", workerID),
		slog.String("job_id", jobMsg.JobID),
		slog.Duration("duration", time.Since(start)),
	)
}
