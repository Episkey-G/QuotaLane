-- Rollback: Remove JSON virtual column index for metadata tags
-- Story: 2-7 Account Metadata and Extended Configuration

-- Drop index first (must drop index before dropping column)
DROP INDEX `idx_tags_virtual` ON `api_accounts`;

-- Drop virtual column
ALTER TABLE `api_accounts`
    DROP COLUMN `tags_virtual`;
