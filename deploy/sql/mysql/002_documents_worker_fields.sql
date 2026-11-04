-- ============================================================================
-- 002：documents 表补充 worker 所需的失败原因与更新时间
-- MySQL 8.0 不支持 ADD COLUMN IF NOT EXISTS，本文件只执行一次。
-- ============================================================================

ALTER TABLE `documents`
  ADD COLUMN `error_msg` VARCHAR(512) DEFAULT NULL COMMENT '最近一次处理失败原因' AFTER `status`,
  ADD COLUMN `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态最后变更时间' AFTER `created_at`;

-- 便于 worker 捞出长时间卡在 processing 的文档做补偿
CREATE INDEX `idx_documents_status_updated` ON `documents` (`status`, `updated_at`);
