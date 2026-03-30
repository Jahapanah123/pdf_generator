package worker

import (
	"context"
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
	wg          sync.WaitGroup
}

func NewPool(
	rmq *queue.RabbitMQ,
	processor *PDFProcessor,
	repo repository.JobRepository,
	logger *slog.Logger,
	concurrency int,
	prefetch int,
) *Pool {
	return &Pool{
		rmq:         rmq,
		processor:   processor,
		repo:        repo,
		logger:      logger,
		concurrency: concurrency,
		prefetch:    prefetch,
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
		_ = msg.Nack(false, false) // malformed → DLQ
		return
	}

	if err := p.processor.Process(ctx, jobMsg); err != nil {
		p.logger.Error("process failed",
			slog.Int("worker_id", workerID),
			slog.String("job_id", jobMsg.JobID),
			slog.Any("error", err),
			slog.Duration("duration", time.Since(start)),
		)

		_ = p.repo.IncrementRetry(ctx, jobMsg.JobID)

		if jobMsg.RetryCount < jobMsg.MaxRetries-1 {
			p.logger.Info("requeuing",
				slog.String("job_id", jobMsg.JobID),
				slog.Int("retry", jobMsg.RetryCount+1),
			)
			_ = msg.Nack(false, true) // requeue
		} else {
			p.logger.Warn("max retries, sending to DLQ",
				slog.String("job_id", jobMsg.JobID),
			)
			errMsg := "max retries exceeded: " + err.Error()
			_ = p.repo.UpdateStatus(ctx, jobMsg.JobID, domain.JobStatusFailed, nil, &errMsg)
			_ = msg.Nack(false, false) // DLQ
		}
		return
	}

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
