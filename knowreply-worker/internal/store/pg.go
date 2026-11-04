package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PG 知识库存储（PostgreSQL + pgvector）。
type PG struct {
	pool *pgxpool.Pool
}

type PGOptions struct {
	DSN              string
	MaxConns         int32
	MinConns         int32
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration
	HealthCheckPerio time.Duration
}

func NewPG(ctx context.Context, opts PGOptions) (*PG, error) {
	cfg, err := pgxpool.ParseConfig(opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse pg dsn: %w", err)
	}
	cfg.MaxConns = opts.MaxConns
	cfg.MinConns = opts.MinConns
	cfg.MaxConnLifetime = opts.MaxConnLifetime
	cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	cfg.HealthCheckPeriod = opts.HealthCheckPerio

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pg: %w", err)
	}
	return &PG{pool: pool}, nil
}

func (p *PG) Close() { p.pool.Close() }

func (p *PG) Ping(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping pg: %w", err)
	}
	return nil
}

// ChunkRow 待写入的知识切片。
type ChunkRow struct {
	TenantID    uint64
	DocID       uint64
	Content     string
	HeadingPath string
	Category    string
	Region      string
	EffectiveAt *time.Time
	Embedding   []float32
}

// ReplaceChunks 在同一事务里「先删后插」，保证重跑幂等，不会产生重复切片。
func (p *PG) ReplaceChunks(ctx context.Context, tenantID, docID uint64, rows []ChunkRow) (int, error) {
	if len(rows) == 0 {
		return 0, fmt.Errorf("no chunks to insert")
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM kb_chunks WHERE tenant_id = $1 AND doc_id = $2`, tenantID, docID); err != nil {
		return 0, fmt.Errorf("delete old chunks: %w", err)
	}

	const insertSQL = `INSERT INTO kb_chunks
		(tenant_id, doc_id, content, heading_path, category, region, effective_at, embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::vector)`

	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(insertSQL,
			r.TenantID, r.DocID, r.Content,
			nullString(r.HeadingPath), nullString(r.Category), nullString(r.Region),
			r.EffectiveAt, VectorLiteral(r.Embedding),
		)
	}

	results := tx.SendBatch(ctx, batch)
	for i := range rows {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return 0, fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}
	if err := results.Close(); err != nil {
		return 0, fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit chunks: %w", err)
	}
	return len(rows), nil
}

// DeleteByDoc 文档下线时删除切片（列保留审计，向量必须清掉避免被检索到）。
func (p *PG) DeleteByDoc(ctx context.Context, tenantID, docID uint64) (int64, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM kb_chunks WHERE tenant_id = $1 AND doc_id = $2`, tenantID, docID)
	if err != nil {
		return 0, fmt.Errorf("delete chunks: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (p *PG) CountByDoc(ctx context.Context, tenantID, docID uint64) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx,
		`SELECT count(*) FROM kb_chunks WHERE tenant_id = $1 AND doc_id = $2`, tenantID, docID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count chunks: %w", err)
	}
	return n, nil
}

// VectorLiteral 把 float32 切片转成 pgvector 文本字面量 "[0.1,0.2,...]"。
func VectorLiteral(v []float32) string {
	buf := make([]byte, 0, len(v)*10+2)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendFloat(buf, float64(f), 'f', -1, 32)
	}
	buf = append(buf, ']')
	return string(buf)
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
