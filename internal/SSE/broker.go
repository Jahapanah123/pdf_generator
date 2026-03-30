package sse

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Client represents a single SSE connection
type Client struct {
	UserID string
	Events chan *domain.JobEvent
	done   chan struct{}
}

// Broker manages all SSE connections and routes RabbitMQ events to clients
type Broker struct {
	rmq            *queue.RabbitMQ
	logger         *slog.Logger
	maxConnections int
	clientBuffer   int

	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
	active  atomic.Int64
}

func NewBroker(rmq *queue.RabbitMQ, logger *slog.Logger, maxConns, clientBuffer int) *Broker {
	return &Broker{
		rmq:            rmq,
		logger:         logger,
		maxConnections: maxConns,
		clientBuffer:   clientBuffer,
		clients:        make(map[string]map[*Client]struct{}),
	}
}

func (b *Broker) Start(ctx context.Context) error {
	msgs, err := b.rmq.ConsumeEvents()
	if err != nil {
		return err
	}

	go b.listenEvents(ctx, msgs)

	b.logger.Info("SSE broker started, consuming from fanout exchange")
	return nil
}

func (b *Broker) listenEvents(ctx context.Context, msgs <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("SSE broker event listener stopped")
			return
		case msg, ok := <-msgs:
			if !ok {
				b.logger.Warn("event channel closed")
				return
			}

			var event domain.JobEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				b.logger.Error("unmarshal event", slog.Any("error", err))
				continue
			}

			b.broadcast(&event)
		}
	}
}

func (b *Broker) broadcast(event *domain.JobEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	userClients, ok := b.clients[event.UserID]
	if !ok || len(userClients) == 0 {
		return
	}

	for client := range userClients {
		select {
		case <-client.done:
			// Client already disconnected, skip
			continue
		default:
		}

		select {
		case <-client.done:
			continue
		case client.Events <- event:
		default:
			b.logger.Warn("SSE client buffer full, dropping event",
				slog.String("user_id", event.UserID),
				slog.String("job_id", event.JobID),
			)
		}
	}
}

func (b *Broker) Subscribe(userID string) (*Client, error) {
	if int(b.active.Load()) >= b.maxConnections {
		return nil, domain.ErrSSEMaxConnections
	}

	client := &Client{
		UserID: userID,
		Events: make(chan *domain.JobEvent, b.clientBuffer),
		done:   make(chan struct{}),
	}

	b.mu.Lock()
	if _, ok := b.clients[userID]; !ok {
		b.clients[userID] = make(map[*Client]struct{})
	}
	b.clients[userID][client] = struct{}{}
	b.mu.Unlock()

	b.active.Add(1)

	b.logger.Info("SSE client connected",
		slog.String("user_id", userID),
		slog.Int64("active", b.active.Load()),
	)

	return client, nil
}

func (b *Broker) Unsubscribe(client *Client) {
	// Signal done first — prevents broadcast from writing to Events
	close(client.done)

	b.mu.Lock()
	if userClients, ok := b.clients[client.UserID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(b.clients, client.UserID)
		}
	}
	b.mu.Unlock()

	// Safe to close Events now — no writer can reach it after done is closed + map removal
	close(client.Events)
	b.active.Add(-1)

	b.logger.Info("SSE client disconnected",
		slog.String("user_id", client.UserID),
		slog.Int64("active", b.active.Load()),
	)
}

func (b *Broker) ActiveConnections() int64 {
	return b.active.Load()
}
