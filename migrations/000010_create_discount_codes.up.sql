-- QuotaLane: Create discount_codes table
-- Description: 折扣码表，存储促销折扣码信息

CREATE TABLE IF NOT EXISTS `discount_codes` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '折扣码ID',
    `code` VARCHAR(50) NOT NULL COMMENT '折扣码',
    `discount_type` ENUM('percentage', 'fixed') NOT NULL COMMENT '折扣类型（百分比或固定金额）',
    `discount_value` DECIMAL(10,2) NOT NULL COMMENT '折扣值（百分比或金额）',
    `max_uses` INT NOT NULL DEFAULT 0 COMMENT '最大使用次数（0表示无限制）',
    `current_uses` INT NOT NULL DEFAULT 0 COMMENT '当前使用次数',
    `starts_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '生效时间',
    `expires_at` TIMESTAMP NULL COMMENT '过期时间',
    `status` ENUM('active', 'inactive', 'expired') NOT NULL DEFAULT 'active' COMMENT '折扣码状态',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_status` (`status`),
    KEY `idx_expires_at` (`expires_at`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='折扣码表';
