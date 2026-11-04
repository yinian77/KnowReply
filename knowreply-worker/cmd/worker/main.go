package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/chunker"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/config"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/embed"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/task"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}

	if err := run(cfg, logger); err != nil {
		logger.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", cfg.MySQL.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	mysqlStore := store.NewMySQL(db)
	if err := mysqlStore.Ping(ctx); err != nil {
		return err
	}

	pgStore, err := store.NewPG(ctx, store.PGOptions{
		DSN:              cfg.PG.DSN,
		MaxConns:         cfg.PG.MaxConns,
		MinConns:         cfg.PG.MinConns,
		MaxConnLifetime:  cfg.PG.MaxConnLife,
		MaxConnIdleTime:  cfg.PG.MaxConnIdle,
		HealthCheckPerio: cfg.PG.HealthCheckP,
	})
	if err != nil {
		return err
	}
	defer pgStore.Close()

	embedder, err := embed.New(embed.Options{
		Provider: cfg.Embedding.Provider,
		BaseURL:  cfg.Embedding.BaseURL,
		APIKey:   cfg.Embedding.APIKey,
		Model:    cfg.Embedding.Model,
		Dim:      cfg.Embedding.Dim,
		Batch:    cfg.Embedding.Batch,
		Timeout:  cfg.Embedding.Timeout,
	})
	if err != nil {
		return err
	}

	ingester := task.NewIngester(task.IngesterOptions{
		Docs:     mysqlStore,
		KB:       pgStore,
		Embedder: embedder,
		Splitter: chunker.Splitter{MinRunes: cfg.Chunk.MinRunes, MaxRunes: cfg.Chunk.MaxRunes},
		Log:      logger,
		RootDir:  os.Getenv("DOC_STORAGE_ROOT"),
	})

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.QueueDB},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      map[string]int{cfg.QueueName: 1},
			LogLevel:    asynq.InfoLevel,
			Logger:      newAsynqLogger(logger),
		},
	)

	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeDocIngest, ingester)
	mux.Handle(queue.TypeDocDelete, task.NewDeleter(pgStore, logger))

	if err := srv.Start(mux); err != nil {
		return err
	}
	logger.Info("worker started",
		"queue", cfg.QueueName,
		"concurrency", cfg.Concurrency,
		"redis", cfg.Redis.Addr,
		"queueDb", cfg.Redis.QueueDB,
		"embedder", embedder.Describe(),
		"chunk", []int{cfg.Chunk.MinRunes, cfg.Chunk.MaxRunes},
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	logger.Info("signal received, draining", "signal", sig.String())

	// Shutdown 会等待在途任务处理完再退出
	srv.Shutdown()
	logger.Info("worker stopped")
	return nil
}
