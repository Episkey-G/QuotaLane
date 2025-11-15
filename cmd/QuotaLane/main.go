// Package main is the entry point of QuotaLane service.
// It initializes the Kratos application with gRPC and HTTP servers.
package main

import (
	"context"
	"flag"
	"os"
	"time"

	"QuotaLane/internal/biz"
	"QuotaLane/internal/conf"
	"QuotaLane/internal/data"
	zapLogger "QuotaLane/pkg/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/robfig/cron/v3"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs/config.yaml", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()

	// Load configuration using Viper with environment variable and CLI flag support
	bc, err := conf.NewBootstrap(flagconf)
	if err != nil {
		// Use fallback logger before Zap is initialized
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize Zap logger from configuration
	zapLog, err := zapLogger.NewZapLogger(bc.Log)
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	defer func() {
		_ = zapLog.Sync() // Ignore sync errors on shutdown
	}()

	// Create Kratos adapter for Zap logger
	logger := zapLogger.NewKratosAdapter(zapLog)

	// Add context fields to logger
	logger = log.With(logger,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	// Log startup configuration using enhanced LogHelper
	zapLogger.NewLogHelper(logger).Startup(
		"QuotaLane service starting",
		"log.level", bc.Log.Level,
		"log.format", bc.Log.Format,
	)

	appComponents, cleanup, err := wireApp(bc.Server, bc.Data, bc.Auth, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// Initialize and start cron scheduler for OAuth token refresh and concurrency cleanup
	cronScheduler := setupCronJobs(appComponents.AccountUC, appComponents.OAuthRefreshTask, appComponents.RateLimiter, appComponents.AccountRepo, logger)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	zapLogger.NewLogHelper(logger).Startup("Cron scheduler started for OAuth token refresh and concurrency cleanup")

	// start and wait for stop signal
	if err := appComponents.App.Run(); err != nil {
		panic(err)
	}
}

// setupCronJobs configures and returns the cron scheduler.
// The scheduler runs AutoRefreshTokens every 5 minutes and concurrency cleanup every minute.
func setupCronJobs(accountUC *biz.AccountUsecase, oauthRefreshTask *biz.OAuthRefreshTask, rateLimiter *biz.RateLimiterUseCase, accountRepo data.AccountRepo, logger log.Logger) *cron.Cron {
	helper := zapLogger.NewLogHelper(logger)

	// Create cron scheduler with seconds support for unified OAuth refresh
	c := cron.New(cron.WithSeconds())

	// Add UNIFIED OAuth token refresh job (every 6 hours: 0:00, 6:00, 12:00, 18:00)
	// Refreshes all OAuth accounts (Claude, Codex) with tokens expiring within 2 hours
	// 优化：避免频繁刷新短期 token（如 Claude 8h），只在真正快过期时刷新
	// Cron format with seconds: "0 0 */6 * * *" (sec min hour day month dow)
	_, err := c.AddFunc("0 0 */6 * * *", func() {
		defer func() {
			if r := recover(); r != nil {
				helper.Errorf("panic in unified OAuth token refresh cron job: %v", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		helper.Info("Starting unified OAuth token refresh task...")
		if err := oauthRefreshTask.RefreshExpiringTokens(ctx); err != nil {
			helper.Errorw("Unified OAuth token refresh task failed", "error", err)
		} else {
			helper.Info("Unified OAuth token refresh task completed successfully")
		}
	})

	if err != nil {
		helper.Fatalf("failed to add unified OAuth refresh cron job: %v", err)
	}

	// Add OAuth token refresh job (every 5 minutes)
	// Cron format with seconds: "0 */5 * * * *" = at minute 0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55
	_, err = c.AddFunc("0 */5 * * * *", func() {
		defer func() {
			if r := recover(); r != nil {
				helper.Errorf("panic in OAuth token refresh cron job: %v", r)
			}
		}()

		ctx := context.Background()
		helper.Info("Starting OAuth token refresh cron job")

		if err := accountUC.AutoRefreshTokens(ctx); err != nil {
			helper.Errorf("OAuth token refresh cron job failed: %v", err)
		} else {
			helper.Info("OAuth token refresh cron job completed successfully")
		}
	})

	if err != nil {
		helper.Fatalf("failed to add OAuth refresh cron job: %v", err)
	}

	// Add OpenAI Responses health check job (every 10 minutes, offset from OAuth refresh)
	// Cron format: "0 2-59/10 * * * *" = at minute 2, 12, 22, 32, 42, 52
	// This avoids conflict with OAuth refresh (0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55)
	_, err = c.AddFunc("0 2-59/10 * * * *", func() {
		defer func() {
			if r := recover(); r != nil {
				helper.Errorf("panic in OpenAI health check cron job: %v", r)
			}
		}()

		ctx := context.Background()
		helper.Info("Starting OpenAI Responses health check cron job")

		if err := accountUC.HealthCheckOpenAIResponsesAccounts(ctx); err != nil {
			helper.Errorf("OpenAI health check cron job failed: %v", err)
		} else {
			helper.Info("OpenAI health check cron job completed successfully")
		}
	})

	if err != nil {
		helper.Fatalf("failed to add OpenAI health check cron job: %v", err)
	}

	// Add concurrency cleanup job (every minute)
	// Cron format: "0 * * * * *" = every minute at second 0
	// Cleans up expired concurrency slots (> 10 minutes old)
	_, err = c.AddFunc("0 * * * * *", func() {
		defer func() {
			if r := recover(); r != nil {
				helper.Errorf("panic in concurrency cleanup cron job: %v", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		helper.Debug("Starting concurrency cleanup cron job")

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
			helper.Errorw("Concurrency cleanup cron job failed", "error", err)
		} else {
			helper.Debugw("Concurrency cleanup cron job completed",
				"total_accounts", len(accountIDs),
				"cleaned", cleanedCount)
		}
	})

	if err != nil {
		helper.Fatalf("failed to add concurrency cleanup cron job: %v", err)
	}

	return c
}
