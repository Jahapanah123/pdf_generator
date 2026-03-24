package domain

import (
	"encoding/json"
	"time"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

type Job struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	TemplateName string          `json:"template_name"`
	Payload      json.RawMessage `json:"payload"`
	Status       JobStatus       `json:"status"`
	FilePath     *string         `json:"file_path,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	RetryCount   int             `json:"retry_count"`
	MaxRetries   int             `json:"max_retries"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

type CreateJobRequest struct {
	TemplateName string          `json:"template_name" validate:"required"`
	Payload      json.RawMessage `json:"payload" validate:"required"`
}

type JobResponse struct {
	ID      string    `json:"id"`
	Status  JobStatus `json:"status"`
	Message string    `json:"message"`
}

type JobStatusResponse struct {
	ID           string     `json:"id"`
	Status       JobStatus  `json:"status"`
	FilePath     *string    `json:"file_path,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type JobMessage struct {
	JobID        string          `json:"job_id"`
	UserID       string          `json:"user_id"`
	TemplateName string          `json:"template_name"`
	Payload      json.RawMessage `json:"payload"`
	RetryCount   int             `json:"retry_count"`
	MaxRetries   int             `json:"max_retries"`
}
