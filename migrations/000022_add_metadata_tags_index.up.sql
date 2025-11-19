-- Migration: Add JSON virtual column index for tags query optimization
-- Purpose: Optimize metadata tags filtering in ListAccounts
-- MySQL Version: 8.0.17+ (required for JSON virtual column indexing)
-- Story: 2-7 Account Metadata and Extended Configuration

-- Check and add virtual column if it doesn't exist
-- Note: MySQL doesn't support ADD COLUMN IF NOT EXISTS before 8.0.29
-- We use a stored procedure to check existence first
DROP PROCEDURE IF EXISTS add_tags_virtual_column;

DELIMITER //
CREATE PROCEDURE add_tags_virtual_column()
BEGIN
    DECLARE col_exists INT DEFAULT 0;

    -- Check if column exists
    SELECT COUNT(*) INTO col_exists
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'api_accounts'
        AND COLUMN_NAME = 'tags_virtual';

    -- Add column if it doesn't exist
    IF col_exists = 0 THEN
        ALTER TABLE `api_accounts`
            ADD COLUMN `tags_virtual` VARCHAR(500)
            AS (JSON_UNQUOTE(JSON_EXTRACT(`metadata`, '$.tags'))) VIRTUAL;
    END IF;

    -- Check if index exists
    SET @index_exists = 0;
    SELECT COUNT(*) INTO @index_exists
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'api_accounts'
        AND INDEX_NAME = 'idx_tags_virtual';

    -- Add index if it doesn't exist
    IF @index_exists = 0 THEN
        CREATE INDEX `idx_tags_virtual` ON `api_accounts` (`tags_virtual`(255));
    END IF;
END//

DELIMITER ;

CALL add_tags_virtual_column();
DROP PROCEDURE IF EXISTS add_tags_virtual_column;

-- Usage examples:
-- Query accounts with specific tag:
--   SELECT * FROM api_accounts
--   WHERE JSON_CONTAINS(metadata->'$.tags', '["production"]')
--   AND deleted_at IS NULL;
--
-- The virtual column index will accelerate these queries significantly
