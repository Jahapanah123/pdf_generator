package validators

import (
	"fmt"

	"github.com/jahapanah123/pdf_generator/internal/domain"
)

type InvoiceValidator struct{}

func (v InvoiceValidator) TemplateName() string {
	return "invoice"
}

func (v InvoiceValidator) Validate(data map[string]any) error {
	required := []string{"invoice_number", "date", "due_date", "from", "to", "items", "total"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("%w: missing required field '%s'", domain.ErrInvalidInput, field)
		}
	}

	from, ok := data["from"].(map[string]any)
	if !ok || from["name"] == nil || from["name"] == "" {
		return fmt.Errorf("%w: from.name is required", domain.ErrInvalidInput)
	}

	to, ok := data["to"].(map[string]any)
	if !ok || to["name"] == nil || to["name"] == "" {
		return fmt.Errorf("%w: to.name is required", domain.ErrInvalidInput)
	}

	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		return fmt.Errorf("%w: at least one item is required", domain.ErrInvalidInput)
	}

	return nil
}
