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
	mu     sync.RWMutex

	// Reconnect
	done       chan struct{}
	notifyConn chan *amqp.Error
	notifyChan chan *amqp.Error
}

func NewRabbitMQ(cfg config.RabbitMQConfig, logger *slog.Logger) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		cfg:    cfg,
		logger: logger,
		done:   make(chan struct{}),
	}

	if err := rmq.connect(); err != nil {
		return nil, err
	}
	if err := rmq.setup(); err != nil {
		return nil, err
	}

	go rmq.reconnectLoop()

	logger.Info("rabbitmq connected",
		slog.String("job_queue", cfg.JobQueue),
		slog.String("event_exchange", cfg.EventExchange),
	)

	return rmq, nil
}

func (r *RabbitMQ) connect() error {
	conn, err := amqp.Dial(r.cfg.URL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	r.conn = conn
	r.ch = ch

	// Register close notifiers
	r.notifyConn = r.conn.NotifyClose(make(chan *amqp.Error, 1))
	r.notifyChan = r.ch.NotifyClose(make(chan *amqp.Error, 1))

	return nil
}

func (r *RabbitMQ) reconnectLoop() {
	for {
		select {
		case <-r.done:
			return
		case connErr := <-r.notifyConn:
			if connErr != nil {
				r.logger.Error("rabbitmq connection lost", slog.Any("error", connErr))
			}
			r.handleReconnect()
		case chanErr := <-r.notifyChan:
			if chanErr != nil {
				r.logger.Error("rabbitmq channel closed", slog.Any("error", chanErr))
			}
			r.handleReconnect()
		}
	}
}

func (r *RabbitMQ) handleReconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()

	backoff := time.Second

	for {
		select {
		case <-r.done:
			return
		default:
		}

		r.logger.Info("attempting rabbitmq reconnect",
			slog.Duration("backoff", backoff),
		)

		time.Sleep(backoff)

		// Clean up old connection
		if r.ch != nil {
			_ = r.ch.Close()
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}

		if err := r.connect(); err != nil {
			r.logger.Error("reconnect failed", slog.Any("error", err))
			backoff = nextBackoff(backoff)
			continue
		}

		if err := r.setup(); err != nil {
			r.logger.Error("re-setup failed", slog.Any("error", err))
			backoff = nextBackoff(backoff)
			continue
		}

		r.logger.Info("rabbitmq reconnected successfully")
		return
	}
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > 30*time.Second {
		return 30 * time.Second
	}
	return next
}

func (r *RabbitMQ) setup() error {
	//  Job direct exchange
	if err := r.ch.ExchangeDeclare(
		r.cfg.JobExchange, "direct", true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare job exchange: %w", err)
	}

	// DLQ (messages that exhausted all retries)
	if _, err := r.ch.QueueDeclare(
		r.cfg.JobDLQ, true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare DLQ: %w", err)
	}
	if err := r.ch.QueueBind(
		r.cfg.JobDLQ, r.cfg.JobDLQ, r.cfg.JobExchange, false, nil,
	); err != nil {
		return fmt.Errorf("bind DLQ: %w", err)
	}

	// Retry queues (retry_1 → main → retry_2 → main → retry_3 → main → usk bad DLQ)
	retryQueues := []struct {
		Name string
		TTL  int
	}{
		{Name: r.retryQueueName(1), TTL: 1000},  // 1 second
		{Name: r.retryQueueName(2), TTL: 5000},  // 5 seconds
		{Name: r.retryQueueName(3), TTL: 10000}, // 10 seconds
	}

	for _, rq := range retryQueues {
		if _, err := r.ch.QueueDeclare(
			rq.Name, true, false, false, false,
			amqp.Table{
				"x-dead-letter-exchange":    r.cfg.JobExchange,
				"x-dead-letter-routing-key": r.cfg.JobQueue, // Expired messages go back to main queue
				"x-message-ttl":             int64(rq.TTL),
			},
		); err != nil {
			return fmt.Errorf("declare retry queue %s: %w", rq.Name, err)
		}
		if err := r.ch.QueueBind(
			rq.Name, rq.Name, r.cfg.JobExchange, false, nil,
		); err != nil {
			return fmt.Errorf("bind retry queue %s: %w", rq.Name, err)
		}
	}

	//  Main job queue - dead letters go to retry_queue_1 first
	if _, err := r.ch.QueueDeclare(
		r.cfg.JobQueue, true, false, false, false,
		amqp.Table{
			"x-dead-letter-exchange":    r.cfg.JobExchange,
			"x-dead-letter-routing-key": r.retryQueueName(1), // First retry
		},
	); err != nil {
		return fmt.Errorf("declare job queue: %w", err)
	}
	if err := r.ch.QueueBind(
		r.cfg.JobQueue, r.cfg.JobQueue, r.cfg.JobExchange, false, nil,
	); err != nil {
		return fmt.Errorf("bind job queue: %w", err)
	}

	//  Fanout exchange for job completion events
	if err := r.ch.ExchangeDeclare(
		r.cfg.EventExchange, "fanout", true, false, false, false, nil,
	); err != nil {
		return fmt.Errorf("declare event exchange: %w", err)
	}

	r.logger.Info("queue topology created",
		slog.String("main_queue", r.cfg.JobQueue),
		slog.String("retry_1", r.retryQueueName(1)),
		slog.String("retry_2", r.retryQueueName(2)),
		slog.String("retry_3", r.retryQueueName(3)),
		slog.String("dlq", r.cfg.JobDLQ),
	)

	return nil
}

func (r *RabbitMQ) retryQueueName(level int) string {
	return fmt.Sprintf("%s_retry_%d", r.cfg.JobQueue, level)
}

func (r *RabbitMQ) RetryQueueName(level int) string {
	return r.retryQueueName(level)
}

func (r *RabbitMQ) PublishJob(ctx context.Context, body []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.JobExchange,
		r.cfg.JobQueue,
		false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (r *RabbitMQ) PublishToRetry(ctx context.Context, level int, body []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if level < 1 || level > 3 {
		return fmt.Errorf("invalid retry level: %d", level)
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.JobExchange,
		r.retryQueueName(level),
		false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (r *RabbitMQ) PublishEvent(ctx context.Context, body []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.EventExchange,
		"",
		false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}

func (r *RabbitMQ) ConsumeJobs(prefetch int) (<-chan amqp.Delivery, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ch.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set QoS: %w", err)
	}

	msgs, err := r.ch.Consume(
		r.cfg.JobQueue, "", false, false, false, false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("consume jobs: %w", err)
	}
	return msgs, nil
}

func (r *RabbitMQ) ConsumeEvents() (<-chan amqp.Delivery, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q, err := r.ch.QueueDeclare(
		"", false, true, true, false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("declare event queue: %w", err)
	}

	if err := r.ch.QueueBind(
		q.Name, "", r.cfg.EventExchange, false, nil,
	); err != nil {
		return nil, fmt.Errorf("bind event queue: %w", err)
	}

	msgs, err := r.ch.Consume(
		q.Name, "", true, true, false, false, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("consume events: %w", err)
	}

	r.logger.Info("event consumer started",
		slog.String("queue", q.Name),
		slog.String("exchange", r.cfg.EventExchange),
	)

	return msgs, nil
}

func (r *RabbitMQ) QueueSize() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q, err := r.ch.QueueInspect(r.cfg.JobQueue)
	if err != nil {
		return 0, fmt.Errorf("inspect queue: %w", err)
	}
	return q.Messages, nil
}

func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	close(r.done)

	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// PublishToDLQ sends a failed message directly to the dead letter queue
func (r *RabbitMQ) PublishToDLQ(ctx context.Context, body []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.ch.PublishWithContext(
		pubCtx,
		r.cfg.JobExchange,
		r.cfg.JobDLQ,
		false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
}
