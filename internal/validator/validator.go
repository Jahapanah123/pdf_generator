package validator

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type Validator struct {
	v *validator.Validate
}

func New() *Validator {
	return &Validator{v: validator.New()}
}

func (val *Validator) ValidateCreateJobRequest(req *domain.CreateJobRequest) []domain.ValidationError {
	if err := val.v.Struct(req); err != nil {
		var errs []domain.ValidationError
		for _, e := range err.(validator.ValidationErrors) {
			errs = append(errs, domain.ValidationError{
				Field:   e.Field(),
				Message: formatError(e),
			})
		}
		return errs
	}
	return nil
}

func formatError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s is invalid", e.Field())
	}
}
