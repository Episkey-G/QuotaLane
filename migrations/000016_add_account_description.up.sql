-- Add description column to api_accounts table
ALTER TABLE `api_accounts` ADD COLUMN `description` TEXT NULL COMMENT '账户描述' AFTER `name`;
