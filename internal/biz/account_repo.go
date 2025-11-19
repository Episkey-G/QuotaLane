package biz

import (
	"context"
	"time"

	"QuotaLane/internal/data"
)

// AccountRepo defines the account repository interface.
// Following Kratos v2 DDD architecture, interfaces are defined in biz layer.
// Implementation is in data layer (data.AccountRepo).
type AccountRepo interface {
	CreateAccount(ctx context.Context, account *data.Account) error
	GetAccount(ctx context.Context, id int64) (*data.Account, error)
	ListAccounts(ctx context.Context, filter *data.AccountFilter) ([]*data.Account, int32, error)
	UpdateAccount(ctx context.Context, account *data.Account) error
	DeleteAccount(ctx context.Context, id int64) error
	ListExpiringAccounts(ctx context.Context, expiryThreshold time.Time) ([]*data.Account, error)
	ListAccountsByProvider(ctx context.Context, provider data.AccountProvider, status data.AccountStatus) ([]*data.Account, error)
	ListCodexCLIAccountsNeedingRefresh(ctx context.Context) ([]*data.Account, error)
	UpdateOAuthData(ctx context.Context, accountID int64, oauthData string, expiresAt time.Time) error
	UpdateHealthScore(ctx context.Context, accountID int64, score int) error
	UpdateAccountStatus(ctx context.Context, accountID int64, status data.AccountStatus) error
	// Story 2-7: Tag-based account filtering
	ListAccountsByTags(ctx context.Context, tags []string, limit, offset int) ([]*data.Account, error)
}
