package factory

import (
	"log/slog"

	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository"
	"github.com/jahapanah123/pdf_generator/internal/service"
	"github.com/jahapanah123/pdf_generator/internal/strategy"
	"github.com/jahapanah123/pdf_generator/internal/strategy/validators"
)

// ServiceFactory creates services with all dependencies
type ServiceFactory struct {
	repo              repository.JobRepository
	rmq               *queue.RabbitMQ
	validatorRegistry *strategy.ValidatorRegistry
	logger            *slog.Logger
	maxRetries        int
}

func NewServiceFactory(
	repo repository.JobRepository,
	rmq *queue.RabbitMQ,
	logger *slog.Logger,
	maxRetries int,
) *ServiceFactory {
	// Initialize validator registry with strategies
	validatorRegistry := strategy.NewValidatorRegistry()
	validatorRegistry.Register(validators.InvoiceValidator{})
	validatorRegistry.Register(validators.ReportValidator{})

	return &ServiceFactory{
		repo:              repo,
		rmq:               rmq,
		validatorRegistry: validatorRegistry,
		logger:            logger,
		maxRetries:        maxRetries,
	}
}

func (f *ServiceFactory) CreatePDFService() *service.PDFService {
	return service.NewPDFService(
		f.repo,
		f.rmq,
		f.validatorRegistry,
		f.logger,
		f.maxRetries,
	)
}
