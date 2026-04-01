package pdf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"
)

type Generator struct {
	outputDir string
}

func NewGenerator(outputDir string) (*Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Generator{outputDir: outputDir}, nil
}

type PDFData struct {
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Author   string            `json:"author"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Items    []PDFItem         `json:"items,omitempty"`
}

type PDFItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Validate checks that the payload has minimum required data
func (d *PDFData) Validate() error {
	if d.Title == "" {
		return fmt.Errorf("title is required")
	}
	if d.Content == "" && len(d.Items) == 0 {
		return fmt.Errorf("content or items required, PDF would be empty")
	}
	return nil
}

func (g *Generator) Generate(jobID, templateName string, payload json.RawMessage) (string, error) {
	var data PDFData
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}

	if err := data.Validate(); err != nil {
		return "", fmt.Errorf("invalid PDF data: %w", err)
	}

	p := gofpdf.New("P", "mm", "A4", "")
	p.SetTitle(data.Title, false)
	if data.Author != "" {
		p.SetAuthor(data.Author, false)
	}
	p.AddPage()

	// Title
	p.SetFont("Arial", "B", 24)
	p.CellFormat(190, 15, data.Title, "", 1, "C", false, 0, "")
	p.Ln(10)

	switch templateName {
	case "invoice":
		g.renderInvoice(p, data)
	case "report":
		g.renderReport(p, data)
	default:
		g.renderDefault(p, data)
	}

	// Footer
	p.SetY(-30)
	p.SetFont("Arial", "I", 8)
	p.CellFormat(190, 10, fmt.Sprintf("Job ID: %s", jobID), "", 0, "C", false, 0, "")

	filename := fmt.Sprintf("%s.pdf", jobID)
	filePath := filepath.Join(g.outputDir, filename)

	if err := p.OutputFileAndClose(filePath); err != nil {
		return "", fmt.Errorf("write PDF: %w", err)
	}

	return filePath, nil
}

func (g *Generator) renderInvoice(p *gofpdf.Fpdf, data PDFData) {
	// Author line — only if present
	if data.Author != "" {
		p.SetFont("Arial", "", 12)
		p.CellFormat(190, 8, fmt.Sprintf("Billed by: %s", data.Author), "", 1, "L", false, 0, "")
		p.Ln(5)
	}

	// Items table — only if items exist
	if len(data.Items) > 0 {
		// Table header
		p.SetFont("Arial", "B", 11)
		p.SetFillColor(200, 200, 200)
		p.CellFormat(120, 8, "Item", "1", 0, "L", true, 0, "")
		p.CellFormat(70, 8, "Amount", "1", 1, "R", true, 0, "")

		// Table rows
		p.SetFont("Arial", "", 10)
		for _, item := range data.Items {
			if item.Label == "" && item.Value == "" {
				continue // skip empty rows
			}
			p.CellFormat(120, 7, item.Label, "1", 0, "L", false, 0, "")
			p.CellFormat(70, 7, item.Value, "1", 1, "R", false, 0, "")
		}
		p.Ln(5)
	}

	// Content / notes — only if present
	if data.Content != "" {
		p.Ln(5)
		p.SetFont("Arial", "B", 11)
		p.CellFormat(190, 8, "Notes:", "", 1, "L", false, 0, "")
		p.SetFont("Arial", "", 10)
		p.MultiCell(190, 6, data.Content, "", "L", false)
	}
}

func (g *Generator) renderReport(p *gofpdf.Fpdf, data PDFData) {
	// Author
	if data.Author != "" {
		p.SetFont("Arial", "I", 10)
		p.CellFormat(190, 8, fmt.Sprintf("Prepared by: %s", data.Author), "", 1, "L", false, 0, "")
		p.Ln(5)
	}

	// Content
	if data.Content != "" {
		p.SetFont("Arial", "", 11)
		p.MultiCell(190, 6, data.Content, "", "L", false)
	}

	// Key findings — only if items exist
	if len(data.Items) > 0 {
		p.Ln(10)
		p.SetFont("Arial", "B", 12)
		p.CellFormat(190, 8, "Key Findings", "", 1, "L", false, 0, "")
		p.Ln(3)

		p.SetFont("Arial", "", 10)
		count := 0
		for _, item := range data.Items {
			if item.Label == "" && item.Value == "" {
				continue
			}
			count++
			p.CellFormat(190, 7,
				fmt.Sprintf("%d. %s: %s", count, item.Label, item.Value),
				"", 1, "L", false, 0, "")
		}
	}
}

func (g *Generator) renderDefault(p *gofpdf.Fpdf, data PDFData) {
	if data.Author != "" {
		p.SetFont("Arial", "", 12)
		p.CellFormat(190, 8, fmt.Sprintf("By: %s", data.Author), "", 1, "L", false, 0, "")
		p.Ln(5)
	}

	if data.Content != "" {
		p.SetFont("Arial", "", 11)
		p.MultiCell(190, 6, data.Content, "", "L", false)
	}

	if len(data.Items) > 0 {
		p.Ln(10)
		p.SetFont("Arial", "B", 11)
		p.CellFormat(190, 8, "Details:", "", 1, "L", false, 0, "")
		p.Ln(3)

		p.SetFont("Arial", "", 10)
		for _, item := range data.Items {
			if item.Label == "" && item.Value == "" {
				continue
			}
			p.CellFormat(190, 7,
				fmt.Sprintf("• %s: %s", item.Label, item.Value),
				"", 1, "L", false, 0, "")
		}
	}
}
