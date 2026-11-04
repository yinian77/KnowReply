package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Store 承载网关侧的持久化操作。约定：所有查询必须带 tenant_id 过滤。
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

type Session struct {
	ID        string
	TenantID  uint64
	UserID    string
	Status    string
	IntentTop sql.NullString
	CreatedAt time.Time
}

type Message struct {
	ID            string
	SessionID     string
	TenantID      uint64
	Role          string
	Content       string
	CitationsJSON []byte
	Intent        sql.NullString
	Confidence    sql.NullFloat64
}

type Document struct {
	ID          uint64     `json:"id"`
	TenantID    uint64     `json:"tenantId"`
	Title       string     `json:"title"`
	SourceURL   *string    `json:"sourceUrl"`
	SourceSize  int64      `json:"sourceSize"`
	Status      string     `json:"status"`
	ChunkCount  int        `json:"chunkCount"`
	ErrorMsg    *string    `json:"errorMsg"`
	Version     string     `json:"version"`
	EffectiveAt *string    `json:"effectiveAt"`
	CreatedAt   *time.Time `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}

// DocStats 知识库汇总：给管理台顶部概览用，避免前端按当前页聚合。
type DocStats struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Ready      int64 `json:"ready"`
	Failed     int64 `json:"failed"`
	Offline    int64 `json:"offline"`
	Chunks     int64 `json:"chunks"`
	Bytes      int64 `json:"bytes"`
}

func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, sessionID string, tenantID uint64, userID string) error {
	const q = `INSERT INTO sessions (id, tenant_id, user_id, status) VALUES (?, ?, ?, 'active')`
	if _, err := s.db.ExecContext(ctx, q, sessionID, tenantID, userID); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// SessionOwned 校验会话存在且属于该租户，防止跨租户劫持 sessionID。
func (s *Store) SessionOwned(ctx context.Context, sessionID string, tenantID uint64) (bool, error) {
	const q = `SELECT 1 FROM sessions WHERE id = ? AND tenant_id = ? LIMIT 1`
	var one int
	err := s.db.QueryRowContext(ctx, q, sessionID, tenantID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query session: %w", err)
	}
	return true, nil
}

func (s *Store) InsertMessage(ctx context.Context, m Message) error {
	const q = `INSERT INTO messages
		(id, session_id, tenant_id, role, content, citations_json, intent, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	var citations any
	if len(m.CitationsJSON) > 0 {
		citations = string(m.CitationsJSON)
	}
	if _, err := s.db.ExecContext(ctx, q,
		m.ID, m.SessionID, m.TenantID, m.Role, m.Content, citations, m.Intent, m.Confidence,
	); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (s *Store) TouchSession(ctx context.Context, sessionID string) error {
	const q = `UPDATE sessions SET updated_at = CURRENT_TIMESTAMP(3) WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, q, sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// ErrDocumentNotFound 文档不存在或不属于该租户。
var ErrDocumentNotFound = errors.New("document not found")

// DocumentMeta 上传/下线场景需要的文档最小信息。
type DocumentMeta struct {
	ID       uint64
	TenantID uint64
	Title    string
	Status   string
}

// CreateDocument 落一条待处理文档，返回自增 docId。
func (s *Store) CreateDocument(ctx context.Context, tenantID uint64, title, sourceURL, version string, sourceSize int64) (uint64, error) {
	const q = `INSERT INTO documents (tenant_id, title, source_url, source_size, status, version)
		VALUES (?, ?, ?, ?, 'pending', ?)`

	res, err := s.db.ExecContext(ctx, q, tenantID, title, sourceURL, sourceSize, version)
	if err != nil {
		return 0, fmt.Errorf("insert document: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get document id: %w", err)
	}
	return uint64(id), nil
}

func (s *Store) GetDocument(ctx context.Context, tenantID, docID uint64) (*DocumentMeta, error) {
	const q = `SELECT id, tenant_id, title, status FROM documents WHERE id = ? AND tenant_id = ?`

	var d DocumentMeta
	err := s.db.QueryRowContext(ctx, q, docID, tenantID).Scan(&d.ID, &d.TenantID, &d.Title, &d.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}
	return &d, nil
}

func (s *Store) SetDocumentStatus(ctx context.Context, tenantID, docID uint64, status string) error {
	const q = `UPDATE documents SET status = ?, updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND tenant_id = ?`
	if _, err := s.db.ExecContext(ctx, q, status, docID, tenantID); err != nil {
		return fmt.Errorf("update document status: %w", err)
	}
	return nil
}

func (s *Store) ListDocuments(ctx context.Context, tenantID uint64, status string, limit, offset int) ([]Document, error) {
	q := `SELECT id, tenant_id, title, source_url, source_size, status, chunk_count,
			error_msg, version, effective_at, created_at, updated_at
		FROM documents WHERE tenant_id = ?`
	args := []any{tenantID}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	docs := make([]Document, 0, limit)
	for rows.Next() {
		var (
			d           Document
			sourceURL   sql.NullString
			errorMsg    sql.NullString
			effectiveAt sql.NullTime
			createdAt   sql.NullTime
			updatedAt   sql.NullTime
		)
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.Title, &sourceURL, &d.SourceSize, &d.Status, &d.ChunkCount,
			&errorMsg, &d.Version, &effectiveAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		if sourceURL.Valid {
			d.SourceURL = &sourceURL.String
		}
		if errorMsg.Valid {
			d.ErrorMsg = &errorMsg.String
		}
		if effectiveAt.Valid {
			s := effectiveAt.Time.Format("2006-01-02")
			d.EffectiveAt = &s
		}
		if createdAt.Valid {
			t := createdAt.Time
			d.CreatedAt = &t
		}
		if updatedAt.Valid {
			t := updatedAt.Time
			d.UpdatedAt = &t
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// DocumentStats 汇总当前租户的文档与切片总量，供管理台顶部概览。
func (s *Store) DocumentStats(ctx context.Context, tenantID uint64) (*DocStats, error) {
	const q = `SELECT COUNT(*),
			COALESCE(SUM(status = 'pending'), 0),
			COALESCE(SUM(status = 'processing'), 0),
			COALESCE(SUM(status = 'ready'), 0),
			COALESCE(SUM(status = 'failed'), 0),
			COALESCE(SUM(status = 'offline'), 0),
			COALESCE(SUM(chunk_count), 0),
			COALESCE(SUM(source_size), 0)
		FROM documents WHERE tenant_id = ?`

	var st DocStats
	err := s.db.QueryRowContext(ctx, q, tenantID).Scan(
		&st.Total, &st.Pending, &st.Processing, &st.Ready, &st.Failed, &st.Offline, &st.Chunks, &st.Bytes,
	)
	if err != nil {
		return nil, fmt.Errorf("query document stats: %w", err)
	}
	return &st, nil
}

// MarshalCitations 将引用 ID 列表序列化为 messages.citations_json。
func MarshalCitations(ids []string) []byte {
	if len(ids) == 0 {
		return nil
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return nil
	}
	return b
}
