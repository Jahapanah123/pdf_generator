package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
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
	p.logger.Info("stopping worker pool")
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
				p.logger.Info("message channel closed", slog.Int("worker_id", id))
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
			_ = msg.Nack(false, false)
			return
		}

		errMsg := fmt.Sprintf("max retries (%d) exceeded: %v", p.maxRetries, err)

		_ = p.repo.UpdateStatus(
			ctx,
			jobMsg.JobID,
			domain.JobStatusFailed,
			nil,
			&errMsg,
		)

		_ = msg.Ack(false)
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

func getRetryCount(msg amqp.Delivery) int {
	if msg.Headers == nil {
		return 0
	}

	deaths, ok := msg.Headers["x-death"].([]interface{})
	if !ok || len(deaths) == 0 {
		return 0
	}

	death, ok := deaths[0].(amqp.Table)
	if !ok {
		return 0
	}

	count, ok := death["count"].(int64)
	if !ok {
		return 0
	}

	return int(count)
}
