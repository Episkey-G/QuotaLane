-- QuotaLane: Create account_audit_logs table
-- Description: 账户审计日志表，记录账户的所有关键操作

CREATE TABLE IF NOT EXISTS `account_audit_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审计日志ID',
    `account_id` BIGINT UNSIGNED NOT NULL COMMENT '账户ID',
    `action_type` ENUM('CREATED', 'UPDATED', 'DELETED', 'TOKEN_REFRESHED', 'HEALTH_CHECK', 'CIRCUIT_BROKEN', 'CIRCUIT_RECOVERED') NOT NULL COMMENT '操作类型',
    `details` JSON NULL COMMENT '操作详情（JSON格式）',
    `operator_id` BIGINT UNSIGNED NULL COMMENT '操作者ID（用户ID）',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_account_id` (`account_id`),
    KEY `idx_action_type` (`action_type`),
    KEY `idx_created_at` (`created_at`),
    KEY `idx_operator_id` (`operator_id`),
    CONSTRAINT `fk_account_audit_logs_account_id` FOREIGN KEY (`account_id`) REFERENCES `api_accounts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户审计日志表';
