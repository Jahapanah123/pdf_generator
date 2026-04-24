package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jahapanah123/pdf_generator/internal/config"
	"github.com/jahapanah123/pdf_generator/internal/pkg/db"
	"github.com/jahapanah123/pdf_generator/internal/pkg/pdf"
	"github.com/jahapanah123/pdf_generator/internal/pkg/queue"
	"github.com/jahapanah123/pdf_generator/internal/repository/postgres"
	"github.com/jahapanah123/pdf_generator/internal/worker"
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

	generator, err := pdf.NewGenerator(cfg.PDF.OutPutDir)
	if err != nil {
		logger.Error("create PDF generator", slog.Any("error", err))
		os.Exit(1)
	}

	jobRepo := postgres.NewJobRepository(pool)
	processor := worker.NewPDFProcessor(jobRepo, rmq, generator, logger)

	workerPool := worker.NewPool(
		rmq, processor, jobRepo, logger,
		cfg.Worker.Concurrency, cfg.RabbitMQ.Prefetch, cfg.Worker.MaxRetries,
	)

	if err := workerPool.Start(ctx); err != nil {
		logger.Error("start worker pool", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("worker pool running",
		slog.Int("concurrency", cfg.Worker.Concurrency),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down workers...")
	cancel()
	workerPool.Stop()
	logger.Info("workers stopped")
}
