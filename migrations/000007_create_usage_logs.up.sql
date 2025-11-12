-- QuotaLane: Create usage_logs table
-- Description: 使用记录表，记录每次 API 调用的详细信息

CREATE TABLE IF NOT EXISTS `usage_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '使用记录ID',
    `user_id` BIGINT UNSIGNED NULL COMMENT '用户ID',
    `api_key_id` BIGINT UNSIGNED NULL COMMENT 'API Key ID',
    `account_id` BIGINT UNSIGNED NULL COMMENT '使用的账户ID',
    `model` VARCHAR(100) NULL COMMENT '使用的模型名称',
    `input_tokens` INT NOT NULL DEFAULT 0 COMMENT '输入Token数',
    `output_tokens` INT NOT NULL DEFAULT 0 COMMENT '输出Token数',
    `cache_creation_tokens` INT NOT NULL DEFAULT 0 COMMENT '缓存创建Token数',
    `cache_read_tokens` INT NOT NULL DEFAULT 0 COMMENT '缓存读取Token数',
    `cost` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '本次调用成本（美元）',
    `latency_ms` INT NULL COMMENT '响应延迟（毫秒）',
    `status` ENUM('success', 'error') NOT NULL DEFAULT 'success' COMMENT '调用状态',
    `error_type` VARCHAR(100) NULL COMMENT '错误类型',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_user_id_created_at` (`user_id`, `created_at`),
    KEY `idx_account_id_created_at` (`account_id`, `created_at`),
    KEY `idx_created_at` (`created_at`),
    KEY `idx_model` (`model`),
    KEY `idx_status` (`status`),
    CONSTRAINT `fk_usage_logs_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_usage_logs_api_key_id` FOREIGN KEY (`api_key_id`) REFERENCES `api_keys` (`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_usage_logs_account_id` FOREIGN KEY (`account_id`) REFERENCES `api_accounts` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='使用记录表';
