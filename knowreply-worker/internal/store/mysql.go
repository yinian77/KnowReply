package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrDocumentNotFound 文档不存在或不属于该租户，属于不可重试错误。
var ErrDocumentNotFound = errors.New("document not found")

// Document documents 表的处理视图。
type Document struct {
	ID          uint64
	TenantID    uint64
	Title       string
	SourceURL   sql.NullString
	Status      string
	Version     string
	Category    sql.NullString
	EffectiveAt sql.NullTime
}

type MySQL struct {
	db *sql.DB
}

func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

func (m *MySQL) Ping(ctx context.Context) error {
	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	return nil
}

// GetDocument 读取文档元数据，强制带 tenant_id 过滤。
func (m *MySQL) GetDocument(ctx context.Context, tenantID, docID uint64) (*Document, error) {
	const q = `SELECT id, tenant_id, title, source_url, status, version, effective_at
		FROM documents WHERE id = ? AND tenant_id = ?`

	var (
		d  Document
		sa sql.NullString
	)
	err := m.db.QueryRowContext(ctx, q, docID, tenantID).
		Scan(&d.ID, &d.TenantID, &d.Title, &sa, &d.Status, &d.Version, &d.EffectiveAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}
	d.SourceURL = sa
	return &d, nil
}

// CreateDocument 落一条待处理文档（status=pending），返回自增 docId。
func (m *MySQL) CreateDocument(ctx context.Context, tenantID uint64, title, sourceURL string) (uint64, error) {
	const q = `INSERT INTO documents (tenant_id, title, source_url, status, version)
		VALUES (?, ?, ?, 'pending', 'v1')`

	res, err := m.db.ExecContext(ctx, q, tenantID, title, sourceURL)
	if err != nil {
		return 0, fmt.Errorf("insert document: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get document id: %w", err)
	}
	return uint64(id), nil
}

func (m *MySQL) MarkProcessing(ctx context.Context, docID uint64) error {
	return m.updateStatus(ctx, docID, "processing", nil)
}

// MarkReady 标记入库完成，同时回写切片数，供管理台汇总展示。
func (m *MySQL) MarkReady(ctx context.Context, docID uint64, chunkCount int) error {
	const q = `UPDATE documents
		SET status = 'ready', error_msg = NULL, chunk_count = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?`
	if _, err := m.db.ExecContext(ctx, q, chunkCount, docID); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	return nil
}

func (m *MySQL) MarkFailed(ctx context.Context, docID uint64, reason string) error {
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return m.updateStatus(ctx, docID, "failed", &reason)
}

func (m *MySQL) updateStatus(ctx context.Context, docID uint64, status string, errMsg *string) error {
	const q = `UPDATE documents SET status = ?, error_msg = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?`
	if _, err := m.db.ExecContext(ctx, q, status, errMsg, docID); err != nil {
		return fmt.Errorf("update document status to %s: %w", status, err)
	}
	return nil
}

// ListStale 找出长时间停在 processing 的文档，供补偿任务重投。
func (m *MySQL) ListStale(ctx context.Context, olderThan time.Duration, limit int) ([]Document, error) {
	const q = `SELECT id, tenant_id, title, status FROM documents
		WHERE status = 'processing' AND updated_at < DATE_SUB(NOW(3), INTERVAL ? SECOND)
		ORDER BY updated_at LIMIT ?`

	rows, err := m.db.QueryContext(ctx, q, int(olderThan.Seconds()), limit)
	if err != nil {
		return nil, fmt.Errorf("query stale documents: %w", err)
	}
	defer rows.Close()

	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Title, &d.Status); err != nil {
			return nil, fmt.Errorf("scan stale document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
