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
	"github.com/jahapanah123/pdf_generator/internal/factory"
	"github.com/jahapanah123/pdf_generator/internal/handler"
	"github.com/jahapanah123/pdf_generator/internal/pkg/db"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository/postgres"
	"github.com/jahapanah123/pdf_generator/internal/router"
	"github.com/jahapanah123/pdf_generator/internal/sse"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Infrastructure
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

	// Factory Pattern: Create repository
	jobRepo := postgres.NewJobRepository(pool)

	// Factory Pattern: Create service with all dependencies
	serviceFactory := factory.NewServiceFactory(jobRepo, rmq, logger, cfg.Worker.MaxRetries)
	pdfService := serviceFactory.CreatePDFService()

	// Other dependencies
	jwtManager := jwtpkg.NewManager(cfg.JWT)
	broker := sse.NewBroker(rmq, logger, cfg.SSE.MaxConnections, cfg.SSE.ClientBuffer)
	if err := broker.Start(ctx); err != nil {
		logger.Error("start SSE broker", slog.Any("error", err))
		os.Exit(1)
	}

	// Handlers (depend on local interfaces - DIP!)
	handlers := handler.NewHandlers(pdfService, broker, jwtManager, logger)

	// Router
	r := router.Setup(handlers, jwtManager, cfg, logger)

	// HTTP Server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("HTTP server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", slog.Any("error", err))
	}

	cancel()
	logger.Info("server stopped")
}
