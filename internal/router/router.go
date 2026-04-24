package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/config"
	"github.com/jahapanah123/pdf_generator/internal/handler"
	"github.com/jahapanah123/pdf_generator/internal/middleware"
	jwtpkg "github.com/jahapanah123/pdf_generator/internal/pkg/jwt"
	"golang.org/x/time/rate"
)

func Setup(
	handlers *handler.Handlers,
	jwtManager *jwtpkg.Manager,
	cfg *config.Config,
	logger *slog.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	rateLimiter := middleware.NewRateLimiter(
		rate.Limit(cfg.Rate.RPS),
		cfg.Rate.Burst,
	)

	// Global middleware
	r.Use(
		middleware.RequestID(),
		middleware.CORS(),
		middleware.Logger(logger),
		middleware.Recovery(logger),
	)

	// Public
	r.GET("/health", handlers.Health.Health)
	r.GET("/ready", handlers.Health.Ready)

	// Auth
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", handlers.Auth.Login)
		auth.POST("/refresh", handlers.Auth.Refresh)
	}

	// Protected
	api := r.Group("/api/v1")
	api.Use(
		middleware.JWTAuth(jwtManager),
		middleware.RateLimit(rateLimiter),
	)
	{
		api.POST("/jobs", handlers.PDF.CreateJob)
		api.GET("/jobs", handlers.PDF.ListJobs)
		api.GET("/jobs/:id", handlers.PDF.GetJobStatus)
		api.GET("/jobs/stream", handlers.SSE.Stream)
	}

	return r
}
