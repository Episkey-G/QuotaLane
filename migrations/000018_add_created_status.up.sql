-- Add 'created' status to account status ENUM
-- This status is used for newly created accounts before they are validated
ALTER TABLE `api_accounts`
  MODIFY COLUMN `status` ENUM('created','active','inactive','error') NOT NULL DEFAULT 'active' COMMENT '账户状态';
