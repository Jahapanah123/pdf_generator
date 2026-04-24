package validators

import (
	"fmt"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type CoverLetterValidator struct{}

func (v CoverLetterValidator) TemplateName() string {
	return "coverletter"
}

func (v CoverLetterValidator) Validate(data map[string]any) error {
	personalInfo, ok := data["personal_info"].(map[string]any)
	if !ok {
		return fmt.Errorf("%w: personal_info is required", domain.ErrInvalidInput)
	}

	if personalInfo["full_name"] == nil || personalInfo["full_name"] == "" {
		return fmt.Errorf("%w: personal_info.full_name is required", domain.ErrInvalidInput)
	}

	if personalInfo["email"] == nil || personalInfo["email"] == "" {
		return fmt.Errorf("%w: personal_info.email is required", domain.ErrInvalidInput)
	}

	required := []string{"company_name", "date", "job_title", "salutation", "opening_paragraph", "closing"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("%w: missing required field '%s'", domain.ErrInvalidInput, field)
		}
	}

	bodyParagraphs, ok := data["body_paragraphs"].([]any)
	if !ok || len(bodyParagraphs) == 0 {
		return fmt.Errorf("%w: at least one body paragraph is required", domain.ErrInvalidInput)
	}

	return nil
}
