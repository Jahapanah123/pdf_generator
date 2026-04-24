package listeners

import (
	"context"
	"log/slog"

	"github.com/jahapanah123/pdf_generator/internal/observer"
	"github.com/jahapanah123/pdf_generator/internal/repository"
)

type DatabaseListener struct {
	repo   repository.JobRepository
	logger *slog.Logger
}

func NewDatabaseListener(repo repository.JobRepository, logger *slog.Logger) *DatabaseListener {
	return &DatabaseListener{
		repo:   repo,
		logger: logger,
	}
}

func (l *DatabaseListener) OnJobStatusChanged(ctx context.Context, event observer.JobEvent) error {
	errMsg := ""
	if event.Error != nil {
		errMsg = event.Error.Error()
	}

	err := l.repo.UpdateStatus(ctx, event.JobID, event.Status, event.FilePath, &errMsg)
	if err != nil {
		l.logger.Error("failed to update job status in DB",
			slog.String("job_id", event.JobID),
			slog.Any("error", err),
		)
		return err
	}

	l.logger.Debug("job status updated in DB",
		slog.String("job_id", event.JobID),
		slog.String("status", string(event.Status)),
	)

	return nil
}
