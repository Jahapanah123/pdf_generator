package observer

import (
	"context"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

// JobEvent represents a state change in job lifecycle
type JobEvent struct {
	JobID     string
	UserID    string
	Status    domain.JobStatus
	FilePath  *string
	Error     error
	Timestamp time.Time
}

// EventListener handles job events
type EventListener interface {
	OnJobStatusChanged(ctx context.Context, event JobEvent) error
}

// EventPublisher manages and notifies listeners
type EventPublisher struct {
	listeners []EventListener
}

func NewEventPublisher() *EventPublisher {
	return &EventPublisher{
		listeners: make([]EventListener, 0),
	}
}

func (p *EventPublisher) Subscribe(listener EventListener) {
	p.listeners = append(p.listeners, listener)
}

func (p *EventPublisher) Publish(ctx context.Context, event JobEvent) {
	for _, listener := range p.listeners {
		// Non-blocking: each listener in goroutine
		go func(l EventListener) {
			if err := l.OnJobStatusChanged(ctx, event); err != nil {
				// Log error but don't fail the main operation
				// In production, you'd send to error tracking service
			}
		}(listener)
	}
}
