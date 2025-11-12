-- QuotaLane: Create invite_codes table
-- Description: 邀请码表，用于用户注册邀请系统

CREATE TABLE IF NOT EXISTS `invite_codes` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '邀请码ID',
    `code` VARCHAR(50) NOT NULL COMMENT '邀请码',
    `created_by` BIGINT UNSIGNED NULL COMMENT '创建者ID（用户ID）',
    `max_uses` INT NOT NULL DEFAULT 1 COMMENT '最大使用次数',
    `current_uses` INT NOT NULL DEFAULT 0 COMMENT '当前使用次数',
    `expires_at` TIMESTAMP NULL COMMENT '过期时间',
    `status` ENUM('active', 'inactive', 'expired') NOT NULL DEFAULT 'active' COMMENT '邀请码状态',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_created_by` (`created_by`),
    KEY `idx_status` (`status`),
    KEY `idx_expires_at` (`expires_at`),
    KEY `idx_created_at` (`created_at`),
    CONSTRAINT `fk_invite_codes_created_by` FOREIGN KEY (`created_by`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邀请码表';
