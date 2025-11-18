-- QuotaLane: Enhance account_groups table
-- Description: 添加软删除字段、唯一索引和审计字段

-- 1. 添加 deleted_at 字段到 account_groups 表(软删除)
ALTER TABLE `account_groups`
ADD COLUMN `deleted_at` TIMESTAMP NULL COMMENT '软删除时间' AFTER `updated_at`;

-- 2. 添加普通索引(MySQL 不支持部分唯一索引,唯一性由应用层保证)
-- 注意: 在 Biz 层需要检查 name 唯一性(WHERE deleted_at IS NULL)
CREATE INDEX `idx_name` ON `account_groups`(`name`);

-- 3. (可选)添加 created_at 字段到 account_group_members 表(审计用途)
ALTER TABLE `account_group_members`
ADD COLUMN `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '成员加入时间' AFTER `account_id`;
