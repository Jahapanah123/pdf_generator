package validators

import (
	"fmt"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type ReportValidator struct{}

func (v ReportValidator) TemplateName() string {
	return "report"
}

func (v ReportValidator) Validate(data map[string]any) error {
	required := []string{"title", "author", "date", "sections"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("%w: missing required field '%s'", domain.ErrInvalidInput, field)
		}
	}

	sections, ok := data["sections"].([]any)
	if !ok || len(sections) == 0 {
		return fmt.Errorf("%w: at least one section is required", domain.ErrInvalidInput)
	}

	return nil
}
