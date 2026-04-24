package strategy

import (
	"fmt"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type PayloadValidator interface {
	Validate(data map[string]any) error
	TemplateName() string
}

type ValidatorRegistry struct {
	validators map[string]PayloadValidator
}

func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make(map[string]PayloadValidator),
	}
}

func (r *ValidatorRegistry) Register(validator PayloadValidator) {
	r.validators[validator.TemplateName()] = validator
}

func (r *ValidatorRegistry) Validate(templateName string, data map[string]any) error {
	validator, exists := r.validators[templateName]
	if !exists {
		return fmt.Errorf("%w: invalid template %s", domain.ErrInvalidInput, templateName)
	}
	return validator.Validate(data)
}
