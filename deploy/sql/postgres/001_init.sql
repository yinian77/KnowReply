-- ============================================================================
-- KnowReply 知识库（PostgreSQL 16 + pgvector）
-- 目标库：knowreply（容器 knowreply-postgres）
-- 执行：docker exec -i knowreply-postgres psql -U knowreply -d knowreply -f -
-- 说明：检索必须强制带 tenant_id 过滤，这是多租户隔离的硬约束。
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS kb_chunks (
  id           BIGSERIAL PRIMARY KEY,
  tenant_id    BIGINT      NOT NULL,
  doc_id       BIGINT      NOT NULL,
  content      TEXT        NOT NULL,
  heading_path TEXT,
  category     TEXT,
  region       TEXT,
  effective_at DATE,
  embedding    vector(1536),
  tsv          tsvector,
  created_at   TIMESTAMPTZ DEFAULT now()
);

-- 向量检索：余弦距离
CREATE INDEX IF NOT EXISTS idx_kb_chunks_embedding
  ON kb_chunks USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

-- 全文检索
CREATE INDEX IF NOT EXISTS idx_kb_chunks_tsv
  ON kb_chunks USING gin (tsv);

-- 租户 + 分类过滤
CREATE INDEX IF NOT EXISTS idx_kb_chunks_tenant_category
  ON kb_chunks (tenant_id, category);

-- 按文档批量删除/重建切片
CREATE INDEX IF NOT EXISTS idx_kb_chunks_tenant_doc
  ON kb_chunks (tenant_id, doc_id);

-- tsv 自动维护：使用 simple 配置（未装 zhparser/pg_jieba，中文按字符切分，
-- 语义召回主要由向量负责，tsv 仅作字面补充）
CREATE OR REPLACE FUNCTION kb_chunks_tsv_trigger() RETURNS trigger AS $$
BEGIN
  NEW.tsv := to_tsvector('simple', coalesce(NEW.content, ''));
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_kb_chunks_tsv ON kb_chunks;
CREATE TRIGGER trg_kb_chunks_tsv
  BEFORE INSERT OR UPDATE OF content ON kb_chunks
  FOR EACH ROW EXECUTE FUNCTION kb_chunks_tsv_trigger();

COMMENT ON TABLE  kb_chunks              IS '知识切片（文档切分后的检索单元）';
COMMENT ON COLUMN kb_chunks.heading_path IS '标题路径，拼接到 content 前作为上下文前缀';
COMMENT ON COLUMN kb_chunks.effective_at IS '生效日期，过期文档下线而非删除';
COMMENT ON COLUMN kb_chunks.embedding    IS '向量，维度需与 embedding 模型一致（1536）';
