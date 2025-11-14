-- QuotaLane: Add oauth_expires_at column to api_accounts table
-- Description: 添加 OAuth Token 过期时间字段，用于高效查询即将过期的账户

ALTER TABLE `api_accounts`
    ADD COLUMN `oauth_expires_at` DATETIME NULL COMMENT 'OAuth Token过期时间'
    AFTER `oauth_data_encrypted`;

CREATE INDEX `idx_oauth_expires_at` ON `api_accounts`(`oauth_expires_at`);
