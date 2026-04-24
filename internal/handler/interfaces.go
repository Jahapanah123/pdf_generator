package handler

import (
	"context"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

// PDFService is defined by CONSUMER (handler), not producer
// This follows Dependency Inversion Principle
type PDFService interface {
	CreateJob(ctx context.Context, userID string, req *domain.CreateJobRequest) (*domain.JobResponse, error)
	GetJobStatus(ctx context.Context, userID, jobID string) (*domain.JobStatusResponse, error)
	ListJobs(ctx context.Context, userID string, limit, offset int) ([]*domain.JobStatusResponse, error)
}
