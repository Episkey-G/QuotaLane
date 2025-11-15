-- QuotaLane: Add base_api and error tracking columns to api_accounts table
-- Description: 为 OpenAI Responses 添加 base_api 字段，并添加错误跟踪字段

ALTER TABLE `api_accounts`
    ADD COLUMN `base_api` VARCHAR(255) NULL COMMENT 'API基础地址（OpenAI Responses等）'
    AFTER `api_key_encrypted`;

ALTER TABLE `api_accounts`
    ADD COLUMN `last_error` TEXT NULL COMMENT '最后一次错误信息（JSON格式）'
    AFTER `metadata`;

ALTER TABLE `api_accounts`
    ADD COLUMN `last_error_at` DATETIME NULL COMMENT '最后一次错误发生时间'
    AFTER `last_error`;

ALTER TABLE `api_accounts`
    ADD COLUMN `consecutive_errors` INT NOT NULL DEFAULT 0 COMMENT '连续失败次数'
    AFTER `last_error_at`;

CREATE INDEX `idx_last_error_at` ON `api_accounts`(`last_error_at`);
