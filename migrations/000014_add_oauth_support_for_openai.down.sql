-- Rollback: 移除 OpenAI OAuth 支持字段
-- Date: 2025-11-15
-- Story: 2.3 - Codex CLI OAuth 授权流程

-- 1. 删除索引
DROP INDEX IF EXISTS `idx_provider_status` ON `api_accounts`;
DROP INDEX IF EXISTS `idx_token_expires_at` ON `api_accounts`;

-- 2. 恢复 provider 枚举（移除 codex-cli）
ALTER TABLE `api_accounts`
    MODIFY COLUMN `provider` ENUM(
        'claude',
        'gemini',
        'openai-responses',
        'bedrock',
        'azure-openai',
        'droid',
        'ccr'
    ) NOT NULL COMMENT '账户类型';

-- 3. 删除 OAuth 相关字段
ALTER TABLE `api_accounts`
    DROP COLUMN `organizations`,
    DROP COLUMN `id_token_encrypted`,
    DROP COLUMN `token_expires_at`,
    DROP COLUMN `refresh_token_encrypted`,
    DROP COLUMN `access_token_encrypted`;
