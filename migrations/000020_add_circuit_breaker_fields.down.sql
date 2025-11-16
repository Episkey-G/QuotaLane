-- QuotaLane: Rollback circuit breaker fields from api_accounts
-- Description: 回滚 Story 2.5 添加的熔断机制相关字段

-- 删除复合索引
ALTER TABLE `api_accounts`
DROP INDEX `idx_circuit_breaker`;

-- 删除熔断时间戳字段
ALTER TABLE `api_accounts`
DROP COLUMN `circuit_broken_at`;

-- 删除乐观锁版本号字段
ALTER TABLE `api_accounts`
DROP COLUMN `version`;
