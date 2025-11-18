-- QuotaLane: Rollback enhance account_groups
-- Description: 回滚账户组表增强(删除字段和索引)

-- 1. 删除 name 索引
DROP INDEX IF EXISTS `idx_name` ON `account_groups`;

-- 2. 删除 deleted_at 字段
ALTER TABLE `account_groups`
DROP COLUMN IF EXISTS `deleted_at`;

-- 3. 删除 created_at 字段(从 account_group_members)
ALTER TABLE `account_group_members`
DROP COLUMN IF EXISTS `created_at`;
