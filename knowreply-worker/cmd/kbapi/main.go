package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/config"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/embed"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/retrieval"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

// 检索预览服务：用于验证切分与召回质量（阶段 B）。
// 定位是内部工具，默认只监听 127.0.0.1；正式形态由 Node Agent 承担检索编排。
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}

	addr := os.Getenv("KB_API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6702"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgStore, err := store.NewPG(ctx, store.PGOptions{
		DSN:              cfg.PG.DSN,
		MaxConns:         cfg.PG.MaxConns,
		MinConns:         cfg.PG.MinConns,
		MaxConnLifetime:  cfg.PG.MaxConnLife,
		MaxConnIdleTime:  cfg.PG.MaxConnIdle,
		HealthCheckPerio: cfg.PG.HealthCheckP,
	})
	if err != nil {
		logger.Error("connect pg failed", "err", err)
		os.Exit(1)
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
		logger.Error("init embedder failed", "err", err)
		os.Exit(1)
	}

	searcher := retrieval.NewSearcher(pgStore, embedder, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "embedder": embedder.Describe()})
	})
	mux.HandleFunc("POST /internal/kb/search", func(w http.ResponseWriter, r *http.Request) {
		handleSearch(r, w, searcher, logger)
	})

	srv := &http.Server{Addr: addr, Handler: withCORS(mux), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		logger.Info("kb api listening", "addr", addr, "embedder", embedder.Describe())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("kb api stopped")
}

type searchRequest struct {
	TenantID   uint64 `json:"tenantId"`
	Query      string `json:"query"`
	Limit      int    `json:"limit"`
	VectorTopK int    `json:"vectorTopK"`
	TextTopK   int    `json:"textTopK"`
	GramTopK   int    `json:"gramTopK"`
}

func handleSearch(r *http.Request, w http.ResponseWriter, searcher *retrieval.Searcher, logger *slog.Logger) {
	defer r.Body.Close()

	var req searchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body: " + err.Error()})
		return
	}

	start := time.Now()
	resp, err := searcher.Search(r.Context(), retrieval.Request{
		TenantID:   req.TenantID,
		Query:      req.Query,
		Limit:      req.Limit,
		VectorTopK: req.VectorTopK,
		TextTopK:   req.TextTopK,
		GramTopK:   req.GramTopK,
	})
	if err != nil {
		logger.Warn("search failed", "err", err, "tenantId", req.TenantID)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	logger.Info("search done",
		"tenantId", req.TenantID, "query", req.Query,
		"vectorHits", resp.VectorHits, "textHits", resp.TextHits, "gramHits", resp.GramHits,
		"tookMs", time.Since(start).Milliseconds())

	writeJSON(w, http.StatusOK, map[string]any{
		"query":      resp.Query,
		"vectorHits": resp.VectorHits,
		"textHits":   resp.TextHits,
		"gramHits":   resp.GramHits,
		"embedder":   resp.Embedder,
		"tookMs":     time.Since(start).Milliseconds(),
		"hits":       resp.Hits,
	})
}

// withCORS 允许本地管理页面跨端口调用检索预览接口。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
