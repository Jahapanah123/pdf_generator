package domain

import "errors"

var (
	ErrJobNotFound       = errors.New("job not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInternalServer    = errors.New("internal server error")
	ErrQueueUnavailable  = errors.New("queue service unavailable")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenInvalid      = errors.New("token invalid")
	ErrSSEMaxConnections = errors.New("max SSE connections reached")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type APIError struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

func NewAPIError(code int, message string, errs ...ValidationError) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Errors:  errs,
	}
}
