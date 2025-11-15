-- Migration: 为 OpenAI 账户添加 OAuth 2.0 支持
-- Date: 2025-11-15
-- Story: 2.3 - Codex CLI OAuth 授权流程

-- 1. 添加 OAuth token 相关字段
ALTER TABLE `api_accounts`
    ADD COLUMN `access_token_encrypted` VARCHAR(1024) NULL COMMENT 'OAuth access token（加密存储）' AFTER `api_key_encrypted`,
    ADD COLUMN `refresh_token_encrypted` VARCHAR(1024) NULL COMMENT 'OAuth refresh token（加密存储）' AFTER `access_token_encrypted`,
    ADD COLUMN `token_expires_at` DATETIME NULL COMMENT 'Access token 过期时间' AFTER `refresh_token_encrypted`,
    ADD COLUMN `id_token_encrypted` VARCHAR(2048) NULL COMMENT 'OpenAI ID token（加密存储，可选）' AFTER `token_expires_at`,
    ADD COLUMN `organizations` TEXT NULL COMMENT '关联的组织列表（JSON 格式）' AFTER `id_token_encrypted`;

-- 2. 扩展 provider 枚举，添加 codex-cli 类型
-- 注意：MySQL 不支持直接修改 ENUM，需要先检查当前值
ALTER TABLE `api_accounts`
    MODIFY COLUMN `provider` ENUM(
        'claude',
        'gemini',
        'openai-responses',
        'codex-cli',
        'bedrock',
        'azure-openai',
        'droid',
        'ccr'
    ) NOT NULL COMMENT '账户类型';

-- 3. 为 token_expires_at 添加索引，优化定时任务查询
CREATE INDEX `idx_token_expires_at` ON `api_accounts`(`token_expires_at`);

-- 4. 为 provider 和 status 的组合查询添加复合索引
-- 优化 ListAccountsByProvider 查询性能
CREATE INDEX `idx_provider_status` ON `api_accounts`(`provider`, `status`);
