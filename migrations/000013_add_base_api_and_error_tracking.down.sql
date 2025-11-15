-- QuotaLane: Remove base_api and error tracking columns from api_accounts table

DROP INDEX `idx_last_error_at` ON `api_accounts`;

ALTER TABLE `api_accounts`
    DROP COLUMN `consecutive_errors`;

ALTER TABLE `api_accounts`
    DROP COLUMN `last_error_at`;

ALTER TABLE `api_accounts`
    DROP COLUMN `last_error`;

ALTER TABLE `api_accounts`
    DROP COLUMN `base_api`;
