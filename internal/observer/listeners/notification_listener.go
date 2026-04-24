package listeners

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/observer"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
)

type NotificationListener struct {
	rmq    *queue.RabbitMQ
	logger *slog.Logger
}

func NewNotificationListener(rmq *queue.RabbitMQ, logger *slog.Logger) *NotificationListener {
	return &NotificationListener{
		rmq:    rmq,
		logger: logger,
	}
}

func (l *NotificationListener) OnJobStatusChanged(ctx context.Context, event observer.JobEvent) error {
	errMsg := ""
	if event.Error != nil {
		errMsg = event.Error.Error()
	}

	jobEvent := &domain.JobEvent{
		JobID:        event.JobID,
		UserID:       event.UserID,
		Status:       event.Status,
		FilePath:     event.FilePath,
		ErrorMessage: &errMsg,
		Timestamp:    time.Now(),
	}

	data, err := json.Marshal(jobEvent)
	if err != nil {
		l.logger.Error("failed to marshal event",
			slog.String("job_id", event.JobID),
			slog.Any("error", err),
		)
		return err
	}

	if err := l.rmq.PublishEvent(ctx, data); err != nil {
		l.logger.Error("failed to publish event to queue",
			slog.String("job_id", event.JobID),
			slog.Any("error", err),
		)
		return err
	}

	l.logger.Debug("event published to queue",
		slog.String("job_id", event.JobID),
		slog.String("status", string(event.Status)),
	)

	return nil
}
