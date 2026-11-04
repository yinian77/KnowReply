package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/yinian77/KnowReply/knowreply-worker/internal/chunker"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/embed"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/parser"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/queue"
	"github.com/yinian77/KnowReply/knowreply-worker/internal/store"
)

const (
	maxFileBytes  = 50 << 20 // 单文件上限 50MB
	maxChunkCount = 4000     // 单文档切片上限，防止异常文件打爆库
)

// Ingester 文档入库流水线：解析 → 切分 → embedding → 写 pgvector → 更新状态。
type Ingester struct {
	docs    *store.MySQL
	kb      *store.PG
	emb     embed.Embedder
	split   chunker.Splitter
	log     *slog.Logger
	client  *http.Client
	rootDir string
}

type IngesterOptions struct {
	Docs     *store.MySQL
	KB       *store.PG
	Embedder embed.Embedder
	Splitter chunker.Splitter
	Log      *slog.Logger
	// RootDir 非空时，本地源文件必须位于该目录内，防止 source_url 被构造成任意文件读取。
	RootDir string
}

func NewIngester(opts IngesterOptions) *Ingester {
	return &Ingester{
		docs:    opts.Docs,
		kb:      opts.KB,
		emb:     opts.Embedder,
		split:   opts.Splitter,
		log:     opts.Log,
		client:  &http.Client{Timeout: 60 * time.Second},
		rootDir: opts.RootDir,
	}
}

// ProcessTask 是 asynq 的处理器入口。返回 asynq.SkipRetry 包裹的错误表示不可重试。
func (i *Ingester) ProcessTask(ctx context.Context, t *asynq.Task) error {
	p, err := queue.ParseIngestPayload(t)
	if err != nil {
		return fmt.Errorf("%w: %s", asynq.SkipRetry, err)
	}

	log := i.log.With("tenantId", p.TenantID, "docId", p.DocID)

	doc, err := i.docs.GetDocument(ctx, p.TenantID, p.DocID)
	if errors.Is(err, store.ErrDocumentNotFound) {
		return fmt.Errorf("%w: document %d not found for tenant %d", asynq.SkipRetry, p.DocID, p.TenantID)
	}
	if err != nil {
		return err
	}

	if err := i.docs.MarkProcessing(ctx, doc.ID); err != nil {
		return err
	}

	if err := i.run(ctx, log, doc); err != nil {
		log.Error("ingest failed", "err", err)
		if mErr := i.docs.MarkFailed(context.WithoutCancel(ctx), doc.ID, err.Error()); mErr != nil {
			log.Error("mark failed status error", "err", mErr)
		}
		return err
	}

	if err := i.docs.MarkReady(ctx, doc.ID); err != nil {
		return err
	}
	log.Info("ingest finished")
	return nil
}

func (i *Ingester) run(ctx context.Context, log *slog.Logger, doc *store.Document) error {
	path, cleanup, err := i.resolveSource(ctx, doc)
	if err != nil {
		return err
	}
	defer cleanup()

	parsed, err := parser.Parse(path)
	if err != nil {
		return fmt.Errorf("%w: %s", asynq.SkipRetry, err)
	}
	log.Info("parsed document",
		"ext", parsed.Ext, "runes", len([]rune(parsed.Text)), "markdown", parsed.Markdown)

	chunks := i.split.Split(parsed.Text, parsed.Markdown)
	if len(chunks) == 0 {
		return fmt.Errorf("%w: no chunks produced", asynq.SkipRetry)
	}
	if len(chunks) > maxChunkCount {
		chunks = chunks[:maxChunkCount]
		log.Warn("chunk count truncated", "limit", maxChunkCount)
	}

	texts := make([]string, len(chunks))
	for idx, c := range chunks {
		texts[idx] = c.Content
	}

	vectors, err := i.emb.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed chunks: %w", err)
	}
	if len(vectors) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: chunks=%d vectors=%d", len(chunks), len(vectors))
	}

	var effectiveAt *time.Time
	if doc.EffectiveAt.Valid {
		v := doc.EffectiveAt.Time
		effectiveAt = &v
	}

	rows := make([]store.ChunkRow, len(chunks))
	for idx, c := range chunks {
		rows[idx] = store.ChunkRow{
			TenantID:    doc.TenantID,
			DocID:       doc.ID,
			Content:     c.Content,
			HeadingPath: c.HeadingPath,
			EffectiveAt: effectiveAt,
			Embedding:   vectors[idx],
		}
	}

	written, err := i.kb.ReplaceChunks(ctx, doc.TenantID, doc.ID, rows)
	if err != nil {
		return err
	}
	log.Info("chunks written", "count", written, "dim", i.emb.Dim(), "embedder", i.emb.Describe())
	return nil
}

// resolveSource 支持本地路径、file:// 与 http(s):// 三种来源。
// 返回的 cleanup 用于清理临时下载文件。
func (i *Ingester) resolveSource(ctx context.Context, doc *store.Document) (string, func(), error) {
	noop := func() {}

	if !doc.SourceURL.Valid || strings.TrimSpace(doc.SourceURL.String) == "" {
		return "", noop, fmt.Errorf("%w: document %d has empty source_url", asynq.SkipRetry, doc.ID)
	}
	src := strings.TrimSpace(doc.SourceURL.String)

	u, err := url.Parse(src)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return i.download(ctx, src)
	}

	path := src
	if err == nil && u.Scheme == "file" {
		path = u.Path
	}

	abs, err := i.safePath(path)
	if err != nil {
		return "", noop, fmt.Errorf("%w: %s", asynq.SkipRetry, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", noop, fmt.Errorf("stat source file: %w", err)
	}
	return abs, noop, nil
}

// safePath 限制本地文件只能落在配置的根目录内。
func (i *Ingester) safePath(p string) (string, error) {
	if !filepath.IsAbs(p) {
		if i.rootDir == "" {
			return "", fmt.Errorf("relative source path %q requires DOC_STORAGE_ROOT", p)
		}
		p = filepath.Join(i.rootDir, p)
	}

	clean := filepath.Clean(p)
	if i.rootDir == "" {
		return clean, nil
	}
	root, err := filepath.Abs(i.rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("source path %q escapes storage root", p)
	}
	return clean, nil
}

func (i *Ingester) download(ctx context.Context, src string) (string, func(), error) {
	noop := func() {}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return "", noop, fmt.Errorf("%w: build download request: %s", asynq.SkipRetry, err)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return "", noop, fmt.Errorf("download document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", noop, fmt.Errorf("download document: status %d", resp.StatusCode)
	}

	ext := filepath.Ext(src)
	if e := filepath.Ext(resp.Request.URL.Path); e != "" {
		ext = e
	}

	f, err := os.CreateTemp("", "knowreply-doc-*"+ext)
	if err != nil {
		return "", noop, fmt.Errorf("create temp file: %w", err)
	}

	written, err := io.Copy(f, io.LimitReader(resp.Body, maxFileBytes+1))
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", noop, fmt.Errorf("save downloaded document: %w", err)
	}
	if written > maxFileBytes {
		_ = os.Remove(f.Name())
		return "", noop, fmt.Errorf("%w: document exceeds %d bytes", asynq.SkipRetry, maxFileBytes)
	}

	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}
