-- QuotaLane: Remove oauth_expires_at column from api_accounts table
-- Description: 回滚 OAuth Token 过期时间字段

DROP INDEX IF EXISTS `idx_oauth_expires_at` ON `api_accounts`;

ALTER TABLE `api_accounts`
    DROP COLUMN IF EXISTS `oauth_expires_at`;
