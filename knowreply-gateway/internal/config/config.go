package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 网关的全部运行参数，统一从环境变量读取，默认值对齐 deploy/docker-compose.yml。
type Config struct {
	Env      string
	HTTPAddr string

	MySQL   MySQL
	Redis   Redis
	Auth    Auth
	Agent   Agent
	Storage Storage
	Queue   Queue

	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	ShutdownTimeout   time.Duration
	WSReadLimit       int64
}

type MySQL struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Redis struct {
	Addr     string
	Password string
	CacheDB  int
	QueueDB  int
}

type Auth struct {
	JWTSecret string
	TokenTTL  time.Duration
}

type Agent struct {
	// Agent gRPC 地址，阶段 P0 预留；未接通时网关走本地 echo 兜底。
	Addr string
}

// Storage 上传文件的落盘位置，必须与 worker 的 DOC_STORAGE_ROOT 指向同一目录。
type Storage struct {
	RootDir        string
	MaxUploadBytes int64
}

// Queue 文档入库任务队列，与 worker 的 WORKER_QUEUE / REDIS_DB_QUEUE 保持一致。
type Queue struct {
	Name string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:      getEnv("APP_ENV", "dev"),
		HTTPAddr: getEnv("HTTP_ADDR", ":6701"),
		MySQL: MySQL{
			DSN:             buildDSN(),
			MaxOpenConns:    getEnvInt("MYSQL_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getEnvInt("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("MYSQL_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Redis: Redis{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:16479"),
			Password: getEnv("REDIS_PASSWORD", ""),
			CacheDB:  getEnvInt("REDIS_DB_CACHE", 1),
			QueueDB:  getEnvInt("REDIS_DB_QUEUE", 2),
		},
		Auth: Auth{
			JWTSecret: getEnv("JWT_SECRET", "dev_only_change_me"),
			TokenTTL:  getEnvDuration("JWT_TTL", 24*time.Hour),
		},
		Agent: Agent{
			Addr: getEnv("AGENT_GRPC_ADDR", "127.0.0.1:50051"),
		},
		Storage: Storage{
			RootDir:        getEnv("DOC_STORAGE_ROOT", "/opt/KnowReply/data/docs"),
			MaxUploadBytes: int64(getEnvInt("MAX_UPLOAD_BYTES", 50<<20)),
		},
		Queue: Queue{
			Name: getEnv("DOC_QUEUE", "ingest"),
		},
		ReadHeaderTimeout: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 10*time.Second),
		WriteTimeout:      getEnvDuration("HTTP_WRITE_TIMEOUT", 0),
		ShutdownTimeout:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		WSReadLimit:       int64(getEnvInt("WS_READ_LIMIT", 64*1024)),
	}

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET must not be empty")
	}
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("MYSQL_DSN must not be empty")
	}
	if cfg.Storage.RootDir == "" {
		return nil, fmt.Errorf("DOC_STORAGE_ROOT must not be empty")
	}
	if cfg.Storage.MaxUploadBytes <= 0 {
		return nil, fmt.Errorf("MAX_UPLOAD_BYTES must be positive")
	}
	if cfg.Queue.Name == "" {
		return nil, fmt.Errorf("DOC_QUEUE must not be empty")
	}
	return cfg, nil
}

// buildDSN 优先取完整 MYSQL_DSN，否则用分项环境变量拼装。
// 时区固定 UTC：MySQL 实例系统时区为 UTC，契约上全链路存 UTC，展示层再转本地。
func buildDSN() string {
	if dsn := os.Getenv("MYSQL_DSN"); dsn != "" {
		return dsn
	}
	host := getEnv("MYSQL_HOST", "127.0.0.1")
	port := getEnv("MYSQL_PORT", "13406")
	user := getEnv("MYSQL_USER", "knowreply")
	pass := getEnv("MYSQL_PASSWORD", "knowreply_2026_mysql")
	db := getEnv("MYSQL_DATABASE", "knowreply")

	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=UTC&timeout=3s&readTimeout=5s&writeTimeout=5s",
		user, pass, host, port, db,
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
