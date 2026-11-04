package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/yinian77/KnowReply/knowreply-gateway/internal/auth"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/config"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/handler"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/store"
	"github.com/yinian77/KnowReply/knowreply-gateway/internal/ws"
)

// Server 组装依赖并持有 HTTP 服务、MySQL 连接池与 Redis 客户端。
type Server struct {
	cfg *config.Config
	log *slog.Logger

	mysql   *sql.DB
	redis   *redis.Client
	queue   *queue.Client
	hub     *ws.Hub
	http    *http.Server
	baseCtx context.Context
	cancel  context.CancelFunc
}

func New(cfg *config.Config, log *slog.Logger) (*Server, error) {
	db, err := sql.Open("mysql", cfg.MySQL.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.CacheDB,
	})
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	if err := os.MkdirAll(cfg.Storage.RootDir, 0o755); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("create storage root %s: %w", cfg.Storage.RootDir, err)
	}

	baseCtx, cancel := context.WithCancel(context.Background())
	st := store.New(db)
	hub := ws.NewHub(log, nil)
	queueCli := queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.QueueDB, cfg.Queue.Name)

	h := handler.New(handler.Deps{
		Store:       st,
		Signer:      auth.NewSigner(cfg.Auth.JWTSecret, cfg.Auth.TokenTTL),
		Hub:         hub,
		Log:         log,
		WSReadLimit: cfg.WSReadLimit,
		Storage:     cfg.Storage,
		Queue:       queueCli,
		BaseCtx:     baseCtx,
		Ready: func(ctx context.Context) error {
			if err := db.PingContext(ctx); err != nil {
				return fmt.Errorf("mysql: %w", err)
			}
			if err := rdb.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redis: %w", err)
			}
			return nil
		},
	})
	hub.SetMessageHandler(h.HandleUserMessage)

	return &Server{
		cfg:     cfg,
		log:     log,
		mysql:   db,
		redis:   rdb,
		queue:   queueCli,
		hub:     hub,
		http:    &http.Server{Addr: cfg.HTTPAddr, Handler: buildRouter(cfg, h), ReadHeaderTimeout: cfg.ReadHeaderTimeout, WriteTimeout: cfg.WriteTimeout},
		baseCtx: baseCtx,
		cancel:  cancel,
	}, nil
}

func buildRouter(cfg *config.Config, h *handler.Handler) http.Handler {
	if cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())
	if cfg.Env != "prod" {
		r.Use(corsMiddleware())
	}

	r.GET("/healthz", h.Health)
	r.GET("/readyz", h.Ready)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/token", h.IssueToken)

	authed := v1.Group("", h.RequireAuth())
	authed.POST("/chat/sessions", h.CreateSession)
	authed.GET("/kb/documents", h.ListDocuments)
	authed.POST("/kb/documents", h.UploadDocument)
	authed.DELETE("/kb/documents/:id", h.DeleteDocument)

	r.GET("/ws/chat", h.RequireAuth(), h.ChatWS)

	return r
}

// Start 阻塞运行 HTTP 服务，返回时说明服务已停止。
func (s *Server) Start() error {
	s.log.Info("http listening", "addr", s.cfg.HTTPAddr, "env", s.cfg.Env, "agent", s.cfg.Agent.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// Shutdown 先停止接收新连接，再断开长连接与存储客户端。
func (s *Server) Shutdown(ctx context.Context) error {
	var errs []error

	if err := s.http.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown http: %w", err))
	}
	s.cancel()

	if err := s.queue.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close queue client: %w", err))
	}
	if err := s.redis.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close redis: %w", err))
	}
	if err := s.mysql.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close mysql: %w", err))
	}

	return errors.Join(errs...)
}
