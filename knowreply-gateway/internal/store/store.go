package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
	Status      string     `json:"status"`
	Version     string     `json:"version"`
	EffectiveAt *string    `json:"effectiveAt"`
	CreatedAt   *time.Time `json:"createdAt"`
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

func (s *Store) ListDocuments(ctx context.Context, tenantID uint64, status string, limit, offset int) ([]Document, error) {
	q := `SELECT id, tenant_id, title, source_url, status, version, effective_at, created_at
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
			effectiveAt sql.NullTime
			createdAt   sql.NullTime
		)
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Title, &sourceURL, &d.Status, &d.Version, &effectiveAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		if sourceURL.Valid {
			d.SourceURL = &sourceURL.String
		}
		if effectiveAt.Valid {
			s := effectiveAt.Time.Format("2006-01-02")
			d.EffectiveAt = &s
		}
		if createdAt.Valid {
			t := createdAt.Time
			d.CreatedAt = &t
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
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
