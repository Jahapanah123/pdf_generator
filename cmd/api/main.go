package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jahapanah123/pdf_generator/internal/config"
	"github.com/jahapanah123/pdf_generator/internal/handler"
	"github.com/jahapanah123/pdf_generator/internal/pkg/db"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository/postgres"
	"github.com/jahapanah123/pdf_generator/internal/router"
	"github.com/jahapanah123/pdf_generator/internal/service"
	"github.com/jahapanah123/pdf_generator/internal/sse"
	"github.com/jahapanah123/pdf_generator/internal/strategy"
	"github.com/jahapanah123/pdf_generator/internal/strategy/validators"
)

func main() {
	logger := newLogger()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPostgresPool(ctx, cfg.DB, logger)
	if err != nil {
		logger.Error("connect postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	rmq, err := queue.NewRabbitMQ(cfg.RabbitMQ, logger)
	if err != nil {
		logger.Error("connect rabbitmq", slog.Any("error", err))
		os.Exit(1)
	}
	defer rmq.Close()

	jobRepo := postgres.NewJobRepository(pool)

	validatorRegistry := strategy.NewValidatorRegistry()
	validatorRegistry.Register(validators.InvoiceValidator{})
	validatorRegistry.Register(validators.ReportValidator{})

	pdfService := service.NewPDFService(
		jobRepo,
		rmq,
		validatorRegistry,
		logger,
		cfg.Worker.MaxRetries,
	)

	jwtManager := jwtpkg.NewManager(cfg.JWT)

	broker := sse.NewBroker(
		rmq,
		logger,
		cfg.SSE.MaxConnections,
		cfg.SSE.ClientBuffer,
	)

	if err := broker.Start(ctx); err != nil {
		logger.Error("start SSE broker", slog.Any("error", err))
		os.Exit(1)
	}

	handlers := handler.NewHandlers(
		pdfService,
		broker,
		jwtManager,
		logger,
	)

	r := router.Setup(handlers, jwtManager, cfg, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	startHTTPServer(srv, logger)
	waitForShutdown(cancel, srv, logger)
}

func newLogger() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	slog.SetDefault(logger)
	return logger
}

func startHTTPServer(srv *http.Server, logger *slog.Logger) {
	go func() {
		logger.Info("HTTP server starting", slog.String("addr", srv.Addr))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()
}

func waitForShutdown(
	cancel context.CancelFunc,
	srv *http.Server,
	logger *slog.Logger,
) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	<-quit

	logger.Info("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", slog.Any("error", err))
	}

	logger.Info("server stopped")
}
