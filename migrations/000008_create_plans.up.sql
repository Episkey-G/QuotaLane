-- QuotaLane: Create plans table
-- Description: 套餐定义表，存储订阅套餐的信息

CREATE TABLE IF NOT EXISTS `plans` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '套餐ID',
    `name` VARCHAR(100) NOT NULL COMMENT '套餐名称',
    `description` TEXT NULL COMMENT '套餐描述',
    `price` DECIMAL(10,2) NOT NULL COMMENT '套餐价格（美元）',
    `dollar_limit` DECIMAL(10,2) NOT NULL COMMENT '美元额度限制',
    `duration_days` INT NOT NULL COMMENT '有效期（天数）',
    `rpm_limit` INT NOT NULL DEFAULT 0 COMMENT '每分钟请求数限制（0表示无限制）',
    `features` JSON NULL COMMENT '套餐特性（JSON格式）',
    `badge` VARCHAR(50) NULL COMMENT '套餐徽章',
    `status` ENUM('active', 'inactive') NOT NULL DEFAULT 'active' COMMENT '套餐状态',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='套餐定义表';
