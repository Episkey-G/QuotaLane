-- Revert status ENUM back to original values
-- Note: This will fail if any rows have status='created'
ALTER TABLE `api_accounts`
  MODIFY COLUMN `status` ENUM('active','inactive','error') NOT NULL DEFAULT 'active' COMMENT '账户状态';
