package store

import (
	"context"
	"fmt"
)

// Scored 单路检索的命中项。Score 语义随检索方式不同（向量=余弦相似度，全文=ts_rank）。
type Scored struct {
	ID          int64
	DocID       uint64
	Content     string
	HeadingPath string
	Score       float64
}

// SearchVector 向量召回。强制带 tenant_id 过滤，这是多租户隔离的硬约束。
func (p *PG) SearchVector(ctx context.Context, tenantID uint64, vec []float32, limit int) ([]Scored, error) {
	const q = `SELECT id, doc_id, content, coalesce(heading_path, ''),
		1 - (embedding <=> $1::vector) AS score
		FROM kb_chunks
		WHERE tenant_id = $2 AND embedding IS NOT NULL
		ORDER BY embedding <=> $1::vector
		LIMIT $3`

	rows, err := p.pool.Query(ctx, q, VectorLiteral(vec), tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	return scanScored(rows)
}

// SearchTrigram 基于 pg_trgm 的字面召回，中文不依赖分词器也能工作：
// 命中子串直接给 1.0 分，否则按三元组相似度打分。这是中文场景下 tsv 的替代方案。
func (p *PG) SearchByGrams(ctx context.Context, tenantID uint64, query string, grams []string, limit int) ([]Scored, error) {
	if len(grams) == 0 {
		return nil, nil
	}

	// 中文没有空格分词，tsv('simple') 与 similarity() 都召不回短问句，
	// 这里用「查询二元组在切片中的覆盖率」做字面召回：
	// 命中任一二元组即入选，按覆盖率排序，整句命中直接给 1.0。
	// 代价：ILIKE '%x%' 走顺序扫描，预览工具的数据量可以接受；
	// 正式检索应由 Agent 接中文分词器或真实向量模型解决。
	// 标题命中额外加权 0.5：标题是人工确认过的主题，比正文里偶然出现的词更可信
	// （"什么时候发货" 会同时命中"已发货商品"和标题"发货时效"，标题优先才符合直觉）。
	const q = `SELECT id, doc_id, content, coalesce(heading_path, ''),
		CASE WHEN content ILIKE '%' || $1 || '%' THEN 1.0
		     ELSE LEAST(1.0,
		       (SELECT count(*) FROM unnest($2::text[]) g WHERE content ILIKE '%' || g || '%')::float8 / $3::float8
		       + 0.5 * (SELECT count(*) FROM unnest($2::text[]) g WHERE coalesce(heading_path, '') ILIKE '%' || g || '%')::float8 / $3::float8)
		END AS score
		FROM kb_chunks
		WHERE tenant_id = $4
		  AND (content ILIKE '%' || $1 || '%'
		       OR EXISTS (SELECT 1 FROM unnest($2::text[]) g WHERE content ILIKE '%' || g || '%'))
		ORDER BY score DESC, id
		LIMIT $5`

	rows, err := p.pool.Query(ctx, q, query, grams, len(grams), tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("gram search: %w", err)
	}
	defer rows.Close()

	return scanScored(rows)
}

// SearchFullText 字面召回。当前 tsv 由 simple 配置生成，中文会整段成词，
// 实际召回效果有限，主要作为向量召回的补充信号参与 RRF 融合。
func (p *PG) SearchFullText(ctx context.Context, tenantID uint64, query string, limit int) ([]Scored, error) {
	const q = `SELECT id, doc_id, content, coalesce(heading_path, ''),
		ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
		FROM kb_chunks
		WHERE tenant_id = $2 AND tsv @@ plainto_tsquery('simple', $1)
		ORDER BY score DESC
		LIMIT $3`

	rows, err := p.pool.Query(ctx, q, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}
	defer rows.Close()

	return scanScored(rows)
}
