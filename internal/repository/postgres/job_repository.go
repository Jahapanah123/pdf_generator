package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"github.com/jahapanah123/pdf_generator/internal/repository"
)

type jobRepository struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) repository.JobRepository {
	return &jobRepository{pool: pool}
}

func (r *jobRepository) Create(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (id, user_id, template_name, payload, status, max_retries, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		job.ID, job.UserID, job.TemplateName, job.Payload,
		job.Status, job.MaxRetries, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (r *jobRepository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	query := `
		SELECT id, user_id, template_name, payload, status, file_path, error_message,
		       retry_count, max_retries, created_at, updated_at, completed_at
		FROM jobs WHERE id = $1`

	job := &domain.Job{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.UserID, &job.TemplateName, &job.Payload,
		&job.Status, &job.FilePath, &job.ErrorMessage,
		&job.RetryCount, &job.MaxRetries,
		&job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrJobNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return job, nil
}

func (r *jobRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Job, error) {
	query := `
		SELECT id, user_id, template_name, payload, status, file_path, error_message,
		       retry_count, max_retries, created_at, updated_at, completed_at
		FROM jobs WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job := &domain.Job{}
		if err := rows.Scan(
			&job.ID, &job.UserID, &job.TemplateName, &job.Payload,
			&job.Status, &job.FilePath, &job.ErrorMessage,
			&job.RetryCount, &job.MaxRetries,
			&job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id string, status domain.JobStatus, filePath *string, errMsg *string) error {
	now := time.Now()
	var completedAt *time.Time
	if status == domain.JobStatusCompleted || status == domain.JobStatusFailed {
		completedAt = &now
	}

	query := `
		UPDATE jobs
		SET status = $2, file_path = $3, error_message = $4, updated_at = $5, completed_at = $6
		WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, status, filePath, errMsg, now, completedAt)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrJobNotFound
	}
	return nil
}

func (r *jobRepository) IncrementRetry(ctx context.Context, id string) error {
	query := `UPDATE jobs SET retry_count = retry_count + 1, updated_at = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("increment retry: %w", err)
	}
	return nil
}
