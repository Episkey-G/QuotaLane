-- QuotaLane: Add circuit breaker fields to api_accounts
-- Description: 为账户表添加乐观锁版本号和熔断时间戳字段,支持 Story 2.5 健康分数和熔断机制

-- 添加乐观锁版本号字段
ALTER TABLE `api_accounts`
ADD COLUMN `version` INT NOT NULL DEFAULT 1 COMMENT '乐观锁版本号,用于并发控制' AFTER `metadata`;

-- 添加熔断时间戳字段
ALTER TABLE `api_accounts`
ADD COLUMN `circuit_broken_at` TIMESTAMP NULL DEFAULT NULL COMMENT '熔断触发时间' AFTER `is_circuit_broken`;

-- 为现有数据初始化 version = 1 (已由 DEFAULT 1 处理)
-- 索引优化:为 is_circuit_broken 和 circuit_broken_at 添加复合索引,用于半开状态查询
ALTER TABLE `api_accounts`
ADD INDEX `idx_circuit_breaker` (`is_circuit_broken`, `circuit_broken_at`);
