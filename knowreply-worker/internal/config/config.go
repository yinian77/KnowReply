package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config worker 运行参数，全部来自环境变量，默认值对齐本机已部署的 MySQL / Redis / PostgreSQL。
type Config struct {
	Redis Redis
	MySQL MySQL
	PG    PG

	Embedding Embedding
	Chunk     Chunk

	Concurrency     int
	QueueName       string
	ShutdownTimeout time.Duration
}

type Redis struct {
	Addr     string
	Password string
	QueueDB  int
}

type MySQL struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type PG struct {
	DSN          string
	MaxConns     int32
	MinConns     int32
	MaxConnLife  time.Duration
	MaxConnIdle  time.Duration
	HealthCheckP time.Duration
}

type Embedding struct {
	Provider string // mock | openai
	BaseURL  string
	APIKey   string
	Model    string
	Dim      int
	Batch    int
	Timeout  time.Duration
}

type Chunk struct {
	MinRunes int
	MaxRunes int
}

func Load() (*Config, error) {
	cfg := &Config{
		Redis: Redis{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:16479"),
			Password: getEnv("REDIS_PASSWORD", ""),
			QueueDB:  getEnvInt("REDIS_DB_QUEUE", 2),
		},
		MySQL: MySQL{
			DSN:             buildMySQLDSN(),
			MaxOpenConns:    getEnvInt("MYSQL_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("MYSQL_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("MYSQL_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		PG: PG{
			DSN:          getEnv("PG_DSN", "postgres://knowreply:knowreply_2026_pg@127.0.0.1:15432/knowreply?sslmode=disable"),
			MaxConns:     int32(getEnvInt("PG_MAX_CONNS", 8)),
			MinConns:     int32(getEnvInt("PG_MIN_CONNS", 1)),
			MaxConnLife:  getEnvDuration("PG_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdle:  getEnvDuration("PG_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckP: getEnvDuration("PG_HEALTH_CHECK_PERIOD", time.Minute),
		},
		Embedding: Embedding{
			Provider: getEnv("EMBEDDING_PROVIDER", "mock"),
			BaseURL:  getEnv("EMBEDDING_BASE_URL", "https://api.openai.com/v1"),
			APIKey:   getEnv("EMBEDDING_API_KEY", ""),
			Model:    getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
			Dim:      getEnvInt("EMBEDDING_DIM", 1536),
			Batch:    getEnvInt("EMBEDDING_BATCH_SIZE", 16),
			Timeout:  getEnvDuration("EMBEDDING_TIMEOUT", 30*time.Second),
		},
		Chunk: Chunk{
			MinRunes: getEnvInt("CHUNK_MIN_RUNES", 300),
			MaxRunes: getEnvInt("CHUNK_MAX_RUNES", 600),
		},
		Concurrency:     getEnvInt("WORKER_CONCURRENCY", 4),
		QueueName:       getEnv("WORKER_QUEUE", "ingest"),
		ShutdownTimeout: getEnvDuration("WORKER_SHUTDOWN_TIMEOUT", 20*time.Second),
	}

	if cfg.Redis.Password == "" {
		return nil, fmt.Errorf("REDIS_PASSWORD must not be empty")
	}
	if cfg.Chunk.MinRunes <= 0 || cfg.Chunk.MaxRunes < cfg.Chunk.MinRunes {
		return nil, fmt.Errorf("invalid chunk size: min=%d max=%d", cfg.Chunk.MinRunes, cfg.Chunk.MaxRunes)
	}
	if cfg.Embedding.Provider == "openai" && cfg.Embedding.APIKey == "" {
		return nil, fmt.Errorf("EMBEDDING_API_KEY is required when EMBEDDING_PROVIDER=openai")
	}
	return cfg, nil
}

// buildMySQLDSN 优先取 MYSQL_DSN；时区固定 UTC，与网关保持一致。
func buildMySQLDSN() string {
	if dsn := os.Getenv("MYSQL_DSN"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=UTC&timeout=3s&readTimeout=5s&writeTimeout=5s",
		getEnv("MYSQL_USER", "knowreply"),
		getEnv("MYSQL_PASSWORD", "knowreply_2026_mysql"),
		getEnv("MYSQL_HOST", "127.0.0.1"),
		getEnv("MYSQL_PORT", "13406"),
		getEnv("MYSQL_DATABASE", "knowreply"),
	)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return fallback
}
