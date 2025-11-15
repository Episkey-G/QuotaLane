-- Rollback: 恢复到迁移 000014 的 provider ENUM 值
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
