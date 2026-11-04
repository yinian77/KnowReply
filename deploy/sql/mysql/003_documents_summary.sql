-- ============================================================================
-- 003：documents 补充汇总展示所需字段
-- chunk_count 由 worker 在入库成功后回写，source_size 由网关在上传时记录。
-- MySQL 8.0 不支持 ADD COLUMN IF NOT EXISTS，本文件只执行一次。
-- ============================================================================

ALTER TABLE `documents`
  ADD COLUMN `source_size` BIGINT UNSIGNED NOT NULL DEFAULT 0
    COMMENT '源文件字节数' AFTER `source_url`,
  ADD COLUMN `chunk_count` INT UNSIGNED NOT NULL DEFAULT 0
    COMMENT '写入 pgvector 的切片数' AFTER `status`;
