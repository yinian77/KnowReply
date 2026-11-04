-- ============================================================================
-- 002：为中文模糊召回补 pg_trgm 表达式索引
-- tsv + simple 配置对中文几乎无效（整段成词），检索预览改用 ILIKE + 三元组相似度，
-- 需要 gin_trgm_ops 索引支撑，否则每次检索都全表扫。
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_kb_chunks_content_trgm
  ON kb_chunks USING gin (content gin_trgm_ops);
