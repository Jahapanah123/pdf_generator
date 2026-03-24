package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	RabbitMQ RabbitMQConfig
	JWT      JWTConfig
	Worker   WorkerConfig
	PDF      PDFConfig
}

type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

type RabbitMQConfig struct {
	URL       string
	QueueName string
	DLQName   string
	Exchange  string
}

type JWTConfig struct {
	AccessTokenSecret  string
	RefreshTokenSecret string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

type WorkerConfig struct {
	Concurrency int
	MaxRetries  int
}

type PDFConfig struct {
	OutPutDir string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},

		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			DBName:   getEnv("DB_NAME", "pdf_service"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
			MaxConns: int32(getIntEnv("DB_MAX_CONNS", 10)),
			MinConns: int32(getIntEnv("DB_MIN_CONNS", 2)),
		},
		RabbitMQ: RabbitMQConfig{
			URL:       getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			QueueName: getEnv("RABBITMQ_QUEUE", "pdf_jobs"),
			DLQName:   getEnv("RABBITMQ_DLQ", "pdf_jobs_dlq"),
			Exchange:  getEnv("RABBITMQ_EXCHANGE", "pdf_exchange"),
		},
		JWT: JWTConfig{
			AccessTokenSecret:  getEnv("JWT_ACCESS_SECRET", ""),
			RefreshTokenSecret: getEnv("JWT_REFRESH_SECRET", ""),
			AccessTokenExpiry:  getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Worker: WorkerConfig{
			Concurrency: getIntEnv("WORKER_CONCURRENCY", 5),
			MaxRetries:  getIntEnv("WORKER_MAX_RETRIES", 3),
		},
		PDF: PDFConfig{
			OutPutDir: getEnv("PDF_OUTPUT_DIR", "./output"),
		},
	}

	if cfg.JWT.AccessTokenSecret == "" {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET is required")
	}

	if cfg.JWT.RefreshTokenSecret == "" {
		return nil, fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	return cfg, nil
}

func (db DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		db.User, db.Password, db.Host, db.Port, db.DBName, db.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
