-- QuotaLane: Create api_accounts table
-- Description: AI 账号池表，存储第三方 AI 服务的账号信息

CREATE TABLE IF NOT EXISTS `api_accounts` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账户ID',
    `name` VARCHAR(100) NOT NULL COMMENT '账户名称',
    `provider` ENUM('claude-official', 'claude-console', 'bedrock', 'ccr', 'droid', 'gemini', 'openai-responses', 'azure-openai') NOT NULL COMMENT 'AI服务提供商',
    `api_key_encrypted` TEXT NULL COMMENT '加密的API Key',
    `oauth_data_encrypted` TEXT NULL COMMENT '加密的OAuth数据（JSON格式）',
    `rpm_limit` INT NOT NULL DEFAULT 0 COMMENT '每分钟请求数限制',
    `tpm_limit` INT NOT NULL DEFAULT 0 COMMENT '每分钟Token数限制',
    `health_score` INT NOT NULL DEFAULT 100 COMMENT '健康分数（0-100）',
    `is_circuit_broken` BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否熔断',
    `status` ENUM('active', 'inactive', 'error') NOT NULL DEFAULT 'active' COMMENT '账户状态',
    `metadata` JSON NULL COMMENT '扩展元数据（代理配置、区域等）',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_provider` (`provider`),
    KEY `idx_status` (`status`),
    KEY `idx_health_score` (`health_score`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI账号池表';
