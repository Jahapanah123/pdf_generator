package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	cfg    config.RabbitMQConfig
	logger *slog.Logger
	mu     sync.Mutex
}

func NewRabbitMQ(cfg config.RabbitMQConfig, logger *slog.Logger) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		cfg:    cfg,
		logger: logger,
	}

	if err := rmq.connect(); err != nil {
		return nil, err
	}

	if err := rmq.setup(); err != nil {
		return nil, err
	}

	logger.Info("Connected to RabbitMQ",
		slog.String("job_queue", cfg.JobQueue),
		slog.String("event_exchange", cfg.EventExchange),
	)

	return rmq, nil
}

func (r *RabbitMQ) connect() error {
	conn, err := amqp.Dial(r.cfg.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	r.conn = conn
	r.ch = ch
	return nil

}

func (r *RabbitMQ) setup() error {
	// Declare job exchange
	if err := r.ch.ExchangeDeclare(
		r.cfg.JobExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare job exchange: %w", err)
	}

	// DLQ

	if _, err := r.ch.QueueDeclare(
		r.cfg.JobDLQ,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare job DLQ: %w", err)
	}

	if err := r.ch.QueueBind(
		r.cfg.JobDLQ,
		r.cfg.JobDLQ,
		r.cfg.JobExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind job DLQ: %w", err)
	}

	// job queue with DLQ routing
	if _, err := r.ch.QueueDeclare(
		r.cfg.JobQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    r.cfg.JobExchange,
			"x-dead-letter-routing-key": r.cfg.JobDLQ,
		},
	); err != nil {
		return fmt.Errorf("failed to declare job queue: %w", err)
	}

	if err := r.ch.QueueBind(
		r.cfg.JobQueue,
		r.cfg.JobQueue,
		r.cfg.JobExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind job queue: %w", err)
	}

	// fanout event exchange

	if err := r.ch.ExchangeDeclare(
		r.cfg.EventExchange,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare event exchange: %w", err)
	}

	return nil
}

func (r *RabbitMQ) PublishJob(ctx context.Context, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.JobExchange,
		r.cfg.JobQueue,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (r *RabbitMQ) PublishEvent(ctx context.Context, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.EventExchange,
		"",
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Transient,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

// ConsumeJobs returns a channel of deliveries from the job queue

func (r *RabbitMQ) ConsumeJobs(prefetch int) (<-chan amqp.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ch.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := r.ch.Consume(
		r.cfg.JobQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to consume from job queue: %w", err)
	}

	return msgs, nil
}

// ConsumeEvents creates an exclusive auto-delete queue bound to the fanout exchange
// Each API instance gets its own queue so all instances receive every event

func (r *RabbitMQ) ConsumeEvents() (<-chan amqp.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	q, err := r.ch.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to declare event queue: %w", err)
	}

	if err := r.ch.QueueBind(
		q.Name,
		"",
		r.cfg.EventExchange,
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to bind event queue: %w", err)
	}

	msgs, err := r.ch.Consume( // / auto-ack, exclusive
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to consume from event queue: %w", err)
	}

	r.logger.Info("event consumer started",
		slog.String("queue", q.Name),
		slog.String("exchange", r.cfg.EventExchange),
	)

	return msgs, nil
}

func (r *RabbitMQ) QueueSize() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	q, err := r.ch.QueueInspect(r.cfg.JobQueue)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect job queue: %w", err)
	}
	return q.Messages, nil
}

func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
