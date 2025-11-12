-- QuotaLane: Create users table
-- Description: 用户基本信息表，存储用户账号、角色、状态、配额等信息

CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    `email` VARCHAR(255) NOT NULL COMMENT '用户邮箱',
    `password_hash` TEXT NOT NULL COMMENT '密码哈希值',
    `role` ENUM('admin', 'user') NOT NULL DEFAULT 'user' COMMENT '用户角色',
    `status` ENUM('active', 'inactive', 'banned') NOT NULL DEFAULT 'active' COMMENT '账户状态',
    `current_plan_id` BIGINT UNSIGNED NULL COMMENT '当前套餐ID',
    `quota_limit` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '配额限制（美元）',
    `quota_used` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '已使用配额（美元）',
    `invite_code` VARCHAR(50) NULL COMMENT '注册使用的邀请码',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_email` (`email`),
    KEY `idx_status` (`status`),
    KEY `idx_current_plan_id` (`current_plan_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户基本信息表';
