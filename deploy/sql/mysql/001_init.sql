-- ============================================================================
-- KnowReply 业务库初始化（MySQL 8.0）
-- 目标库：knowreply（复用实例 lhplat-mysql）
-- 执行：mysql -h127.0.0.1 -P13406 -uknowreply -p knowreply < 001_init.sql
-- 说明：所有查询必须带 tenant_id 过滤，索引均以 tenant_id 打头或覆盖等值条件。
-- ============================================================================

SET NAMES utf8mb4;

-- ---------------------------------------------------------------------------
-- 租户
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `tenants` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(128)    NOT NULL                COMMENT '租户名称',
  `status`     VARCHAR(16)     NOT NULL DEFAULT 'active' COMMENT 'active / suspended',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenants_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='租户';

-- ---------------------------------------------------------------------------
-- 会话
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `sessions` (
  `id`         VARCHAR(64)     NOT NULL                COMMENT '业务会话ID，如 s_123',
  `tenant_id`  BIGINT UNSIGNED NOT NULL,
  `user_id`    VARCHAR(64)     NOT NULL                COMMENT '终端用户标识',
  `status`     VARCHAR(16)     NOT NULL DEFAULT 'active' COMMENT 'active / transferred / closed',
  `intent_top` VARCHAR(64)     DEFAULT NULL            COMMENT '本会话主导意图',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_sessions_tenant_user` (`tenant_id`, `user_id`, `created_at`),
  KEY `idx_sessions_tenant_status` (`tenant_id`, `status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='会话';

-- ---------------------------------------------------------------------------
-- 消息（含 AI 引用片段，用于回溯与评估）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `messages` (
  `id`             VARCHAR(64)     NOT NULL            COMMENT '消息ID，如 c_001 / a_001',
  `session_id`     VARCHAR(64)     NOT NULL,
  `tenant_id`      BIGINT UNSIGNED NOT NULL,
  `role`           VARCHAR(16)     NOT NULL            COMMENT 'user / assistant / system',
  `content`        MEDIUMTEXT      NOT NULL,
  `citations_json` JSON            DEFAULT NULL        COMMENT '命中的 kb_chunks.id 列表',
  `intent`         VARCHAR(64)     DEFAULT NULL,
  `confidence`     DECIMAL(5,4)    DEFAULT NULL        COMMENT '0.0000 ~ 1.0000',
  `created_at`     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_messages_session_created` (`session_id`, `created_at`),
  KEY `idx_messages_tenant_created` (`tenant_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='消息';

-- ---------------------------------------------------------------------------
-- 工单（转人工）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `tickets` (
  `id`         VARCHAR(64)     NOT NULL                COMMENT '工单ID，如 t_001',
  `tenant_id`  BIGINT UNSIGNED NOT NULL,
  `session_id` VARCHAR(64)     NOT NULL,
  `reason`     VARCHAR(64)     NOT NULL                COMMENT 'user_request / high_risk_intent / low_confidence / negative_sentiment / round_limit',
  `status`     VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending / assigned / closed',
  `assignee`   VARCHAR(64)     DEFAULT NULL            COMMENT '人工坐席',
  `created_at` DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_tickets_tenant_status` (`tenant_id`, `status`, `created_at`),
  KEY `idx_tickets_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='工单';

-- ---------------------------------------------------------------------------
-- 文档元数据（正文与向量在 PostgreSQL）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `documents` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id`    BIGINT UNSIGNED NOT NULL,
  `title`        VARCHAR(255)    NOT NULL,
  `source_url`   VARCHAR(512)    DEFAULT NULL,
  `status`       VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending / processing / ready / failed',
  `version`      VARCHAR(32)     NOT NULL DEFAULT 'v1',
  `effective_at` DATE            DEFAULT NULL,
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_documents_tenant_status` (`tenant_id`, `status`),
  KEY `idx_documents_tenant_effective` (`tenant_id`, `effective_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='知识文档元数据';
