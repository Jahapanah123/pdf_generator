package repository

import (
	"context"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type JobRepository interface {
	Create(ctx context.Context, job *domain.Job) error
	GetByID(ctx context.Context, id string) (*domain.Job, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Job, error)
	UpdateStatus(ctx context.Context, id string, status domain.JobStatus, filePath *string, errMsg *string) error
	IncrementRetry(ctx context.Context, id string) error
}
