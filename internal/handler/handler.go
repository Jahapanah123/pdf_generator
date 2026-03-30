package handler

import (
	"log/slog"

	"github.com/jahapahanh123/pdf_generator/internal/SSE"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
	"github.com/jahapanah123/pdf_generator/internal/service"
)

type Handlers struct {
	PDF    *PDFHandler
	SSE    *SSEHandler
	Auth   *AuthHandler
	Health *HealthHandler
}

func NewHandlers(
	pdfService service.PDFService,
	broker *SSE.Broker,
	jwtManager *jwtpkg.Manager,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		PDF:    NewPDFHandler(pdfService, logger),
		SSE:    NewSSEHandler(broker, logger),
		Auth:   NewAuthHandler(jwtManager, logger),
		Health: NewHealthHandler(broker),
	}
}
