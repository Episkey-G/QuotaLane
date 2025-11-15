-- Migration: 修复 provider ENUM 值，恢复完整的 provider 类型
-- Date: 2025-11-15
-- Issue: 迁移 000014 错误地将 'claude-official' 改为 'claude'，导致 OAuth 账户创建失败

-- 修复 provider 枚举值，确保包含所有支持的 provider 类型
ALTER TABLE `api_accounts`
    MODIFY COLUMN `provider` ENUM(
        'claude-official',
        'claude-console',
        'bedrock',
        'ccr',
        'droid',
        'gemini',
        'openai-responses',
        'codex-cli',
        'azure-openai'
    ) NOT NULL COMMENT 'AI服务提供商';
