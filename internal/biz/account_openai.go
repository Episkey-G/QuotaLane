package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"QuotaLane/internal/data"
	"QuotaLane/pkg/oauth"
)

const (
	// MaxConcurrentHealthCheck 最大并发健康检查数
	MaxConcurrentHealthCheck = 5

	// HealthCheckFailureKeyPrefix Redis 健康检查失败计数器前缀
	HealthCheckFailureKeyPrefix = "health_check_failure:"

	// HealthCheckAlertKeyPrefix Redis 健康检查告警标记前缀
	HealthCheckAlertKeyPrefix = "alert:health_check:"

	// HealthCheckAlertTTL 告警标记 TTL（24 小时）
	HealthCheckAlertTTL = 24 * time.Hour
)

// ErrorRecord 错误记录结构（存储在 last_error 字段）
type ErrorRecord struct {
	Code       int       `json:"code"`
	Message    string    `json:"message"`
	RetryCount int       `json:"retry_count"`
	BaseAPI    string    `json:"base_api,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ValidateOpenAIResponsesAccount 验证 OpenAI Responses 账户
// accountID: 账户 ID
// 返回: 验证成功返回 nil，失败返回错误
func (uc *AccountUsecase) ValidateOpenAIResponsesAccount(ctx context.Context, accountID int64) error {
	// 1. 从 Repo 读取账户信息
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// 验证 Provider 类型
	if account.Provider != data.ProviderOpenAIResponses {
		return fmt.Errorf("account is not OpenAI Responses type: provider=%s", account.Provider)
	}

	// 验证必填字段
	if account.APIKeyEncrypted == "" {
		return fmt.Errorf("account API key is empty")
	}
	if account.BaseAPI == "" {
		return fmt.Errorf("account base API is empty")
	}

	// 2. 解密 API Key
	apiKey, err := uc.crypto.Decrypt(account.APIKeyEncrypted)
	if err != nil {
		uc.logger.Errorw("failed to decrypt API key",
			"account_id", accountID,
			"error", err)
		return fmt.Errorf("failed to decrypt API key: %w", err)
	}

	// 3. 提取代理配置（从 metadata JSON 读取 proxy_url）
	var proxyURL string
	if account.Metadata != nil && *account.Metadata != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(*account.Metadata), &metadata); err != nil {
			uc.logger.Warnw("failed to parse metadata JSON, skipping proxy",
				"account_id", accountID,
				"error", err)
		} else if proxy, ok := metadata["proxy_url"].(string); ok {
			proxyURL = proxy
		}
	}

	// 4. 通过 OAuth Manager 获取 Provider 并验证
	provider := uc.oauthManager.GetProvider(data.ProviderOpenAIResponses)
	if provider == nil {
		return fmt.Errorf("OpenAI Responses provider not registered")
	}

	// 构建 AccountMetadata
	accountMetadata := &oauth.AccountMetadata{
		ProxyURL: proxyURL,
		BaseAPI:  account.BaseAPI,
	}

	// 调用 Provider 验证 API Key
	err = provider.ValidateToken(ctx, apiKey, accountMetadata)

	if err != nil {
		// 验证失败：记录错误、减分、更新状态
		return uc.handleValidationFailure(ctx, account, err)
	}

	// 5. 验证成功：恢复健康分数、更新状态、清除错误记录
	return uc.handleValidationSuccess(ctx, account)
}

// handleValidationSuccess 处理验证成功的情况
func (uc *AccountUsecase) handleValidationSuccess(ctx context.Context, account *data.Account) error {
	// 更新健康分数为 100
	if err := uc.repo.UpdateHealthScore(ctx, account.ID, 100); err != nil {
		uc.logger.Errorw("failed to update health score after success",
			"account_id", account.ID,
			"error", err)
		return err
	}

	// 更新状态为 ACTIVE
	if err := uc.repo.UpdateAccountStatus(ctx, account.ID, data.StatusActive); err != nil {
		uc.logger.Errorw("failed to update status after success",
			"account_id", account.ID,
			"error", err)
		return err
	}

	// 清除连续失败计数和错误记录
	account.ConsecutiveErrors = 0
	account.LastError = nil
	account.LastErrorAt = nil
	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		uc.logger.Warnw("failed to clear error records",
			"account_id", account.ID,
			"error", err)
	}

	// 清除 Redis 失败计数器
	failureKey := fmt.Sprintf("%s%d", HealthCheckFailureKeyPrefix, account.ID)
	if err := uc.rdb.Del(ctx, failureKey).Err(); err != nil {
		uc.logger.Warnw("failed to clear failure counter",
			"account_id", account.ID,
			"error", err)
	}

	// 检查是否需要恢复熔断状态
	if account.HealthScore >= 50 && account.IsCircuitBroken {
		account.IsCircuitBroken = false
		if err := uc.repo.UpdateAccount(ctx, account); err != nil {
			uc.logger.Warnw("failed to recover circuit breaker",
				"account_id", account.ID,
				"error", err)
		} else {
			uc.logger.Infow("circuit breaker recovered",
				"account_id", account.ID,
				"account_name", account.Name,
				"health_score", 100)
		}
	}

	uc.logger.Infow("OpenAI account validation succeeded",
		"account_id", account.ID,
		"account_name", account.Name,
		"health_score", 100)

	return nil
}

// handleValidationFailure 处理验证失败的情况
func (uc *AccountUsecase) handleValidationFailure(ctx context.Context, account *data.Account, validationErr error) error {
	// 减少健康分数 20 分（与 Story 2.2 保持一致）
	newScore := account.HealthScore - 20
	if err := uc.repo.UpdateHealthScore(ctx, account.ID, newScore); err != nil {
		uc.logger.Errorw("failed to update health score after failure",
			"account_id", account.ID,
			"error", err)
		return err
	}

	// 更新状态为 ERROR
	if err := uc.repo.UpdateAccountStatus(ctx, account.ID, data.StatusError); err != nil {
		uc.logger.Errorw("failed to update status after failure",
			"account_id", account.ID,
			"error", err)
		return err
	}

	// 记录错误信息
	errorRecord := ErrorRecord{
		Code:       extractErrorCode(validationErr),
		Message:    validationErr.Error(),
		RetryCount: 3, // OpenAI 服务默认重试 3 次
		BaseAPI:    account.BaseAPI,
		OccurredAt: time.Now(),
	}
	errorJSON, _ := json.Marshal(errorRecord)
	errorStr := string(errorJSON)

	now := time.Now()
	account.LastError = &errorStr
	account.LastErrorAt = &now
	account.ConsecutiveErrors++

	if err := uc.repo.UpdateAccount(ctx, account); err != nil {
		uc.logger.Warnw("failed to update error records",
			"account_id", account.ID,
			"error", err)
	}

	// 增加 Redis 失败计数器
	failureKey := fmt.Sprintf("%s%d", HealthCheckFailureKeyPrefix, account.ID)
	if err := uc.rdb.Incr(ctx, failureKey).Err(); err != nil {
		uc.logger.Warnw("failed to increment failure counter",
			"account_id", account.ID,
			"error", err)
	}

	// 检查是否需要触发熔断
	if newScore < 30 && !account.IsCircuitBroken {
		account.IsCircuitBroken = true
		if err := uc.repo.UpdateAccount(ctx, account); err != nil {
			uc.logger.Errorw("failed to set circuit breaker",
				"account_id", account.ID,
				"error", err)
		}

		// 设置告警标记
		alertKey := fmt.Sprintf("%s%d", HealthCheckAlertKeyPrefix, account.ID)
		alertMessage := fmt.Sprintf("OpenAI Responses 健康分数低于30: account_id=%d, name=%s, score=%d",
			account.ID, account.Name, newScore)
		if err := uc.rdb.Set(ctx, alertKey, alertMessage, HealthCheckAlertTTL).Err(); err != nil {
			uc.logger.Warnw("failed to set alert marker",
				"account_id", account.ID,
				"error", err)
		}

		uc.logger.Errorw("circuit breaker triggered",
			"account_id", account.ID,
			"account_name", account.Name,
			"health_score", newScore,
			"last_error", validationErr.Error())
	}

	uc.logger.Errorw("OpenAI account validation failed",
		"account_id", account.ID,
		"account_name", account.Name,
		"error", validationErr,
		"new_health_score", newScore,
		"consecutive_errors", account.ConsecutiveErrors)

	return validationErr
}

// HealthCheckOpenAIResponsesAccounts 批量健康检查所有 ACTIVE 状态的 OpenAI Responses 账户
// 定时任务调用此方法
func (uc *AccountUsecase) HealthCheckOpenAIResponsesAccounts(ctx context.Context) error {
	startTime := time.Now()

	// 查询所有 ACTIVE 状态的 OpenAI Responses 账户
	accounts, err := uc.repo.ListAccountsByProvider(ctx, data.ProviderOpenAIResponses, data.StatusActive)
	if err != nil {
		uc.logger.Errorw("failed to list OpenAI Responses accounts", "error", err)
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	totalCount := len(accounts)
	if totalCount == 0 {
		uc.logger.Infow("no active OpenAI Responses accounts to check")
		return nil
	}

	uc.logger.Infow("starting OpenAI Responses health check",
		"total_accounts", totalCount)

	// 使用 semaphore 限制并发数为 5
	semaphore := make(chan struct{}, MaxConcurrentHealthCheck)
	results := make(chan error, totalCount)

	// 并发检查所有账户
	for _, account := range accounts {
		semaphore <- struct{}{} // 获取信号量

		go func(acc *data.Account) {
			defer func() { <-semaphore }() // 释放信号量

			// 执行健康检查
			err := uc.ValidateOpenAIResponsesAccount(ctx, acc.ID)
			results <- err
		}(account)
	}

	// 等待所有检查完成并统计结果
	successCount := 0
	failureCount := 0
	for i := 0; i < totalCount; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	duration := time.Since(startTime)

	uc.logger.Infow("OpenAI Responses health check completed",
		"total_accounts", totalCount,
		"success_count", successCount,
		"failure_count", failureCount,
		"duration_ms", duration.Milliseconds())

	return nil
}

// extractErrorCode 从错误消息中提取 HTTP 状态码
func extractErrorCode(err error) int {
	errMsg := err.Error()
	// 简单的状态码提取逻辑
	if errMsg == "" {
		return 0
	}
	// 尝试匹配 "HTTP 401", "HTTP 429" 等模式
	var code int
	if _, scanErr := fmt.Sscanf(errMsg, "invalid API key (HTTP %d)", &code); scanErr == nil {
		return code
	}
	if _, scanErr := fmt.Sscanf(errMsg, "client error (HTTP %d)", &code); scanErr == nil {
		return code
	}
	if _, scanErr := fmt.Sscanf(errMsg, "server error (HTTP %d)", &code); scanErr == nil {
		return code
	}
	return 0
}
