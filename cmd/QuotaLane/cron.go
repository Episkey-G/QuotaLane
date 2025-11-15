package main

import (
	"context"
	"time"

	"QuotaLane/internal/biz"
	"QuotaLane/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
)

// StartTokenRefreshCron 启动 Token 刷新定时任务
// 执行频率：每 6 小时执行一次
// 刷新策略：刷新 24 小时内过期的 Token
func StartTokenRefreshCron(task *biz.OAuthRefreshTask, logger log.Logger) *cron.Cron {
	helper := log.NewHelper(logger)

	c := cron.New(cron.WithSeconds())

	// 每 6 小时执行一次（在整点执行：0:00, 6:00, 12:00, 18:00）
	// Cron 表达式：0 0 */6 * * * （秒 分 时 日 月 周）
	_, err := c.AddFunc("0 0 */6 * * *", func() {
		helper.Info("Starting OAuth token refresh task...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := task.RefreshExpiringTokens(ctx); err != nil {
			helper.Errorw("OAuth token refresh task failed", "error", err)
		} else {
			helper.Info("OAuth token refresh task completed successfully")
		}
	})

	if err != nil {
		helper.Errorw("failed to register token refresh cron job", "error", err)
		return nil
	}

	c.Start()
	helper.Info("Token refresh cron job started: runs every 6 hours (0:00, 6:00, 12:00, 18:00)")

	return c
}

// StartConcurrencyCleanupCron 启动并发槽位清理定时任务
// 执行频率：每分钟执行一次
// 清理策略：清理 > 10 分钟的超时请求
func StartConcurrencyCleanupCron(rateLimiter *biz.RateLimiterUseCase, accountRepo data.AccountRepo, logger log.Logger) *cron.Cron {
	helper := log.NewHelper(logger)

	c := cron.New(cron.WithSeconds())

	// 每分钟执行一次
	// Cron 表达式：0 * * * * * （每分钟的第0秒执行）
	_, err := c.AddFunc("0 * * * * *", func() {
		helper.Debug("Starting concurrency cleanup task...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Get all active account IDs
		accounts, _, err := accountRepo.ListAccounts(ctx, &data.AccountFilter{
			Status:   data.StatusActive,
			Page:     1,
			PageSize: 1000, // Process up to 1000 accounts per run
		})
		if err != nil {
			helper.Errorw("Failed to list accounts for concurrency cleanup", "error", err)
			return
		}

		// Extract account IDs
		accountIDs := make([]int64, 0, len(accounts))
		for _, account := range accounts {
			accountIDs = append(accountIDs, account.ID)
		}

		if len(accountIDs) == 0 {
			helper.Debug("No active accounts to clean up")
			return
		}

		// Clean up expired concurrency for all accounts
		cleanedCount, err := rateLimiter.CleanupExpiredConcurrencyForAllAccounts(ctx, accountIDs)
		if err != nil {
			helper.Errorw("Concurrency cleanup task failed", "error", err)
		} else {
			helper.Debugw("Concurrency cleanup task completed",
				"total_accounts", len(accountIDs),
				"cleaned", cleanedCount)
		}
	})

	if err != nil {
		helper.Errorw("failed to register concurrency cleanup cron job", "error", err)
		return nil
	}

	c.Start()
	helper.Info("Concurrency cleanup cron job started: runs every minute")

	return c
}
