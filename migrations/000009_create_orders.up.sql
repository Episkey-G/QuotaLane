-- QuotaLane: Create orders table
-- Description: 订单记录表，存储用户的订单信息

CREATE TABLE IF NOT EXISTS `orders` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '订单ID',
    `order_no` VARCHAR(50) NOT NULL COMMENT '订单编号',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `plan_id` BIGINT UNSIGNED NOT NULL COMMENT '套餐ID',
    `original_amount` DECIMAL(10,2) NOT NULL COMMENT '原始金额（美元）',
    `discount_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '折扣金额（美元）',
    `final_amount` DECIMAL(10,2) NOT NULL COMMENT '最终金额（美元）',
    `status` ENUM('pending', 'paid', 'cancelled', 'refunded') NOT NULL DEFAULT 'pending' COMMENT '订单状态',
    `paid_at` TIMESTAMP NULL COMMENT '支付时间',
    `starts_at` TIMESTAMP NULL COMMENT '套餐开始时间',
    `expires_at` TIMESTAMP NULL COMMENT '套餐过期时间',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_plan_id` (`plan_id`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`),
    CONSTRAINT `fk_orders_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_orders_plan_id` FOREIGN KEY (`plan_id`) REFERENCES `plans` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='订单记录表';
