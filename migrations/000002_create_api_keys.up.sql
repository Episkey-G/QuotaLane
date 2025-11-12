-- QuotaLane: Create api_keys table
-- Description: 虚拟 Token 表，存储用户的 API Key 信息

CREATE TABLE IF NOT EXISTS `api_keys` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'API Key ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `token_hash` VARCHAR(255) NOT NULL COMMENT 'Token哈希值',
    `name` VARCHAR(100) NULL COMMENT 'API Key名称',
    `rate_limit` INT NOT NULL DEFAULT 100 COMMENT '速率限制（RPM）',
    `status` ENUM('active', 'expired', 'revoked') NOT NULL DEFAULT 'active' COMMENT 'Key状态',
    `expires_at` TIMESTAMP NULL COMMENT '过期时间',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `last_used_at` TIMESTAMP NULL COMMENT '最后使用时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_token_hash` (`token_hash`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`),
    CONSTRAINT `fk_api_keys_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='虚拟Token表';
