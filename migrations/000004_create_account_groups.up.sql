-- QuotaLane: Create account_groups table
-- Description: 账户组表，用于对 API 账户进行分组管理

CREATE TABLE IF NOT EXISTS `account_groups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账户组ID',
    `name` VARCHAR(100) NOT NULL COMMENT '组名称',
    `description` TEXT NULL COMMENT '组描述',
    `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级（数字越大优先级越高）',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_priority` (`priority`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户组表';
