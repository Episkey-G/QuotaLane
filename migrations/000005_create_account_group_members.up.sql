-- QuotaLane: Create account_group_members table
-- Description: 账户组成员关系表，建立账户和组的多对多关系

CREATE TABLE IF NOT EXISTS `account_group_members` (
    `group_id` BIGINT UNSIGNED NOT NULL COMMENT '账户组ID',
    `account_id` BIGINT UNSIGNED NOT NULL COMMENT '账户ID',
    PRIMARY KEY (`group_id`, `account_id`),
    KEY `idx_account_id` (`account_id`),
    CONSTRAINT `fk_account_group_members_group_id` FOREIGN KEY (`group_id`) REFERENCES `account_groups` (`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_account_group_members_account_id` FOREIGN KEY (`account_id`) REFERENCES `api_accounts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户组成员关系表';
