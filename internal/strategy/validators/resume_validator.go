package validators

import (
	"fmt"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type ResumeValidator struct{}

func (v ResumeValidator) TemplateName() string {
	return "resume"
}

func (v ResumeValidator) Validate(data map[string]any) error {
	personalInfo, ok := data["personal_info"].(map[string]any)
	if !ok {
		return fmt.Errorf("%w: personal_info is required and must be an object", domain.ErrInvalidInput)
	}

	if personalInfo["full_name"] == nil || personalInfo["full_name"] == "" {
		return fmt.Errorf("%w: personal_info.full_name is required", domain.ErrInvalidInput)
	}

	if personalInfo["email"] == nil || personalInfo["email"] == "" {
		return fmt.Errorf("%w: personal_info.email is required", domain.ErrInvalidInput)
	}

	return nil
}
