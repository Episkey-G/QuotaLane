package main

import (
	"context"
	"time"

	"QuotaLane/internal/biz"

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
