package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"QuotaLane/internal/data"
	pkgoauth "QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2/errors"
)

const (
	// MaxConcurrentRefresh 最大并发刷新数
	MaxConcurrentRefresh = 5

	// RefreshFailureKeyPrefix Redis 失败计数器前缀
	RefreshFailureKeyPrefix = "refresh_failure:"

	// RefreshFailureTTL 失败计数器 TTL（30 分钟）
	RefreshFailureTTL = 30 * time.Minute

	// MaxConsecutiveFailures 最大连续失败次数
	MaxConsecutiveFailures = 3

	// AlertKeyPrefix Redis 告警标记前缀
	AlertKeyPrefix = "alert:"

	// AlertTTL 告警标记 TTL（24 小时）
	AlertTTL = 24 * time.Hour
)

// OAuthData represents the decrypted OAuth data structure.
type OAuthData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RefreshClaudeToken 刷新指定账户的 Claude OAuth Token
// accountID: 账户 ID
// 返回错误如果刷新失败
func (uc *AccountUsecase) RefreshClaudeToken(ctx context.Context, accountID int64) error {
	// 1. 从数据库读取账户信息
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// 验证账户类型
	if account.Provider != data.ProviderClaudeOfficial && account.Provider != data.ProviderClaudeConsole {
		return fmt.Errorf("account %d is not a Claude account (provider: %s)", accountID, account.Provider)
	}

	// 2. 解密 OAuth 数据
	if account.OAuthDataEncrypted == "" {
		return fmt.Errorf("account %d has no OAuth data", accountID)
	}

	decrypted, err := uc.crypto.Decrypt(account.OAuthDataEncrypted)
	if err != nil {
		uc.logger.Errorf("failed to decrypt OAuth data for account %d: %v", accountID, err)
		return fmt.Errorf("failed to decrypt OAuth data")
	}

	var oauthData OAuthData
	if err := json.Unmarshal([]byte(decrypted), &oauthData); err != nil {
		uc.logger.Errorf("failed to parse OAuth data for account %d: %v", accountID, err)
		return fmt.Errorf("failed to parse OAuth data")
	}

	// 3. 提取 refresh_token
	refreshToken := oauthData.RefreshToken
	if refreshToken == "" {
		return fmt.Errorf("account %d has no refresh_token", accountID)
	}

	// 4. 解析 metadata 并转换为 OAuth metadata 格式
	var oauthMeta *pkgoauth.AccountMetadata
	if account.Metadata != nil && *account.Metadata != "" {
		// 使用 metadata 包解析
		meta, err := data.ParseMetadata(account.Metadata)
		if err != nil {
			uc.logger.Warnf("failed to parse account metadata for account %d: %v", accountID, err)
		} else if !meta.IsEmpty() {
			// 转换为 OAuth metadata 格式
			oauthMeta = &pkgoauth.AccountMetadata{
				ProxyURL: meta.ProxyURL,
			}
			// 如果代理未启用，清空 proxy_url
			if !meta.ProxyEnabled {
				oauthMeta.ProxyURL = ""
			}
		}
	}

	// 5. 调用统一 OAuth Manager 刷新 Token
	tokenResp, err := uc.oauthManager.RefreshToken(ctx, account.Provider, refreshToken, oauthMeta)
	if err != nil {
		uc.logger.Errorf("OAuth refresh failed for account %d: %v", accountID, err)

		// 处理刷新失败
		if err := uc.handleRefreshFailure(ctx, accountID, err); err != nil {
			uc.logger.Warnf("failed to handle refresh failure: %v", err)
		}

		return fmt.Errorf("OAuth refresh failed: %w", err)
	}

	// 6. 构建新的 OAuth 数据
	newExpiresAt := time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	newOAuthData := OAuthData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    newExpiresAt,
	}

	// 7. 加密新的 OAuth 数据
	newJSON, err := json.Marshal(newOAuthData)
	if err != nil {
		return fmt.Errorf("failed to marshal OAuth data: %w", err)
	}

	encrypted, err := uc.crypto.Encrypt(string(newJSON))
	if err != nil {
		uc.logger.Errorf("failed to encrypt OAuth data for account %d: %v", accountID, err)
		return fmt.Errorf("failed to encrypt OAuth data")
	}

	// 8. 更新数据库
	if err := uc.repo.UpdateOAuthData(ctx, accountID, encrypted, newExpiresAt); err != nil {
		return fmt.Errorf("failed to update OAuth data: %w", err)
	}

	// 9. 刷新成功，重置健康分数并清除失败计数器
	if err := uc.repo.UpdateHealthScore(ctx, accountID, 100); err != nil {
		uc.logger.Warnf("failed to reset health score for account %d: %v", accountID, err)
	}

	// 清除失败计数器
	if uc.rdb != nil {
		failureKey := fmt.Sprintf("%s%d", RefreshFailureKeyPrefix, accountID)
		if err := uc.rdb.Del(ctx, failureKey).Err(); err != nil {
			uc.logger.Warnf("failed to delete failure counter for account %d: %v", accountID, err)
		}
	}

	uc.logger.Infow("OAuth token refreshed successfully",
		"account_id", accountID,
		"name", account.Name,
		"expires_at", newExpiresAt)

	return nil
}

// handleRefreshFailure 处理 Token 刷新失败
func (uc *AccountUsecase) handleRefreshFailure(ctx context.Context, accountID int64, refreshErr error) error {
	// 更新健康分数减 20 分
	account, err := uc.repo.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	newScore := account.HealthScore - 20
	if err := uc.repo.UpdateHealthScore(ctx, accountID, newScore); err != nil {
		return fmt.Errorf("failed to update health score: %w", err)
	}

	// 使用 Redis 跟踪失败次数
	if uc.rdb == nil {
		uc.logger.Warn("Redis client is nil, cannot track failure count")
		return nil
	}

	failureKey := fmt.Sprintf("%s%d", RefreshFailureKeyPrefix, accountID)

	// INCR 失败计数器
	failureCount, err := uc.rdb.Incr(ctx, failureKey).Result()
	if err != nil {
		return fmt.Errorf("failed to increment failure counter: %w", err)
	}

	// 设置 TTL（30 分钟）
	if err := uc.rdb.Expire(ctx, failureKey, RefreshFailureTTL).Err(); err != nil {
		uc.logger.Warnf("failed to set TTL for failure counter: %v", err)
	}

	uc.logger.Warnw("refresh failure tracked",
		"account_id", accountID,
		"failure_count", failureCount,
		"error", refreshErr)

	// 检查是否连续失败 3 次
	if failureCount >= MaxConsecutiveFailures {
		// 标记账户为 ERROR 状态
		if err := uc.repo.UpdateAccountStatus(ctx, accountID, data.StatusError); err != nil {
			return fmt.Errorf("failed to update account status: %w", err)
		}

		// 记录 ERROR 级别日志
		uc.logger.Errorw("account marked as ERROR due to consecutive failures",
			"account_id", accountID,
			"name", account.Name,
			"failure_count", failureCount,
			"last_error", refreshErr)

		// 设置告警标记
		alertKey := fmt.Sprintf("%s%d", AlertKeyPrefix, accountID)
		alertMsg := fmt.Sprintf("Account %d (%s) marked as ERROR: %d consecutive refresh failures. Last error: %v",
			accountID, account.Name, failureCount, refreshErr)

		if err := uc.rdb.Set(ctx, alertKey, alertMsg, AlertTTL).Err(); err != nil {
			uc.logger.Warnf("failed to set alert marker: %v", err)
		}

		// TODO: 发送 Webhook 告警通知（预留接口，后续 Story 实现）
		// if uc.webhook != nil {
		// 	uc.webhook.SendAlert(ctx, accountID, alertMsg)
		// }
	}

	return nil
}

// AutoRefreshTokens 自动刷新即将过期的 Claude 账户 Token（定时任务调用）
// 查询 oauth_expires_at 在未来 10 分钟内的账户并触发刷新
func (uc *AccountUsecase) AutoRefreshTokens(ctx context.Context) error {
	startTime := time.Now()

	// 查询即将过期的账户（未来 10 分钟内）
	threshold := time.Now().UTC().Add(10 * time.Minute)
	accounts, err := uc.repo.ListExpiringAccounts(ctx, threshold)
	if err != nil {
		return fmt.Errorf("failed to list expiring accounts: %w", err)
	}

	if len(accounts) == 0 {
		uc.logger.Info("no expiring accounts found")
		return nil
	}

	uc.logger.Infow("starting auto refresh",
		"account_count", len(accounts),
		"threshold", threshold)

	// 使用 goroutine 并发刷新（限制并发数为 5）
	var (
		wg           sync.WaitGroup
		successCount int32
		failureCount int32
		sem          = make(chan struct{}, MaxConcurrentRefresh)
		mu           sync.Mutex
	)

	for _, account := range accounts {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(acc *data.Account) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			// 刷新 Token
			if err := uc.RefreshClaudeToken(ctx, acc.ID); err != nil {
				uc.logger.Errorf("failed to refresh account %d (%s): %v", acc.ID, acc.Name, err)
				mu.Lock()
				failureCount++
				mu.Unlock()
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(account)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	elapsed := time.Since(startTime)

	uc.logger.Infow("auto refresh completed",
		"total_accounts", len(accounts),
		"success_count", successCount,
		"failure_count", failureCount,
		"elapsed", elapsed)

	// 如果所有账户都刷新失败，返回错误
	if failureCount > 0 && successCount == 0 {
		return errors.InternalServer("AUTO_REFRESH_ALL_FAILED", "all account token refresh attempts failed")
	}

	return nil
}
