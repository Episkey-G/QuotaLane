package biz

import (
	"context"
	"time"

	"QuotaLane/internal/data"

	"github.com/go-kratos/kratos/v2/log"
)

// AccountGroup is a type alias for data.AccountGroupData for convenience.
type AccountGroup = data.AccountGroupData

// AccountGroupRepo defines the account group repository interface.
// Following Kratos v2 DDD architecture, interfaces are defined in biz layer.
// Implementation is in data layer (data.AccountGroupRepo).
// Uses data layer models to avoid circular dependency.
type AccountGroupRepo interface {
	CreateGroup(ctx context.Context, name string, description string, priority int32, accountIDs []int64) (int64, error)
	GetGroup(ctx context.Context, id int64) (*data.AccountGroupData, error)
	ListGroups(ctx context.Context, page, pageSize int32) ([]*data.AccountGroupData, int64, error)
	UpdateGroup(ctx context.Context, id int64, name string, description string, priority int32, accountIDs []int64) error
	DeleteGroup(ctx context.Context, id int64) error
	GetAccountGroups(ctx context.Context, accountID int64) ([]*data.AccountGroupData, error)
	GetAllGroupedAccountIDs(ctx context.Context) ([]int64, error)
}

// AccountGroupUseCase handles account group business logic.
type AccountGroupUseCase struct {
	repo        AccountGroupRepo
	accountRepo AccountRepo
	log         *log.Helper
}

// NewAccountGroupUseCase creates a new account group use case.
func NewAccountGroupUseCase(
	repo AccountGroupRepo,
	accountRepo AccountRepo,
	logger log.Logger,
) *AccountGroupUseCase {
	return &AccountGroupUseCase{
		repo:        repo,
		accountRepo: accountRepo,
		log:         log.NewHelper(log.With(logger, "module", "biz/account-group")),
	}
}

// CreateAccountGroup creates a new account group.
func (uc *AccountGroupUseCase) CreateAccountGroup(
	ctx context.Context,
	name string,
	description string,
	priority int32,
	accountIDs []int64,
) (*AccountGroup, error) {
	// Validate name uniqueness (MySQL doesn't support partial UNIQUE index)
	// We need to check manually for soft-deleted groups
	existing, _, err := uc.repo.ListGroups(ctx, 1, 1000) // Check all groups
	if err != nil {
		return nil, err
	}
	for _, g := range existing {
		if g.Name == name {
			return nil, NewValidationError("账户组名称已存在")
		}
	}

	// Validate account IDs exist
	if len(accountIDs) > 0 {
		for _, accountID := range accountIDs {
			_, err := uc.accountRepo.GetAccount(ctx, accountID)
			if err != nil {
				uc.log.Warnf("invalid account ID %d: %v", accountID, err)
				return nil, NewValidationError("账户 ID 无效或不存在")
			}
		}
	}

	groupID, err := uc.repo.CreateGroup(ctx, name, description, priority, accountIDs)
	if err != nil {
		return nil, err
	}

	group := &AccountGroup{
		ID:          groupID,
		Name:        name,
		Description: description,
		Priority:    priority,
		AccountIDs:  accountIDs,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	uc.log.Infof("created account group: id=%d, name=%s, priority=%d, members=%d",
		groupID, name, priority, len(accountIDs))

	return group, nil
}

// GetAccountGroup retrieves a group by ID with full details.
func (uc *AccountGroupUseCase) GetAccountGroup(ctx context.Context, id int64) (*AccountGroup, error) {
	group, err := uc.repo.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}

	return group, nil
}

// ListAccountGroups retrieves a paginated list of groups.
func (uc *AccountGroupUseCase) ListAccountGroups(ctx context.Context, page, pageSize int32) ([]*AccountGroup, int64, error) {
	return uc.repo.ListGroups(ctx, page, pageSize)
}

// UpdateAccountGroup updates an existing group.
func (uc *AccountGroupUseCase) UpdateAccountGroup(
	ctx context.Context,
	id int64,
	name string,
	description string,
	priority int32,
	accountIDs []int64,
) error {
	// Verify group exists
	existing, err := uc.repo.GetGroup(ctx, id)
	if err != nil {
		return err
	}

	// Validate name uniqueness if changed
	if name != existing.Name {
		allGroups, _, err := uc.repo.ListGroups(ctx, 1, 1000)
		if err != nil {
			return err
		}
		for _, g := range allGroups {
			if g.ID != id && g.Name == name {
				return NewValidationError("账户组名称已存在")
			}
		}
	}

	// Validate new account IDs
	if len(accountIDs) > 0 {
		for _, accountID := range accountIDs {
			_, err := uc.accountRepo.GetAccount(ctx, accountID)
			if err != nil {
				uc.log.Warnf("invalid account ID %d: %v", accountID, err)
				return NewValidationError("账户 ID 无效或不存在")
			}
		}
	}

	if err := uc.repo.UpdateGroup(ctx, id, name, description, priority, accountIDs); err != nil {
		return err
	}

	uc.log.Infof("updated account group: id=%d, name=%s, priority=%d, members=%d",
		id, name, priority, len(accountIDs))

	return nil
}

// DeleteAccountGroup soft deletes a group.
func (uc *AccountGroupUseCase) DeleteAccountGroup(ctx context.Context, id int64) error {
	// Verify group exists
	group, err := uc.repo.GetGroup(ctx, id)
	if err != nil {
		return err
	}

	if err := uc.repo.DeleteGroup(ctx, id); err != nil {
		return err
	}

	uc.log.Infof("deleted account group: id=%d, name=%s", id, group.Name)

	return nil
}

// GetAccountsByGroup retrieves all accounts in a group.
func (uc *AccountGroupUseCase) GetAccountsByGroup(ctx context.Context, groupID int64) ([]*Account, error) {
	group, err := uc.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	accounts := make([]*Account, 0, len(group.AccountIDs))
	for _, accountID := range group.AccountIDs {
		account, err := uc.accountRepo.GetAccount(ctx, accountID)
		if err != nil {
			uc.log.Warnf("failed to get account %d: %v", accountID, err)
			continue // Skip missing accounts (might be deleted)
		}

		// Convert data.Account to biz.Account
		bizAccount := &Account{
			ID:              account.ID,
			Name:            account.Name,
			Provider:        string(account.Provider),
			Status:          string(account.Status),
			HealthScore:     account.HealthScore,
			IsCircuitBroken: account.IsCircuitBroken,
		}
		accounts = append(accounts, bizAccount)
	}

	return accounts, nil
}

// GetDefaultGroup returns a virtual default group containing all ungrouped accounts.
func (uc *AccountGroupUseCase) GetDefaultGroup(ctx context.Context) (*AccountGroup, error) {
	// Get all account IDs (simplified: list all active accounts)
	allAccounts, _, err := uc.accountRepo.ListAccounts(ctx, nil)
	if err != nil {
		return nil, err
	}

	allAccountIDs := make([]int64, len(allAccounts))
	for i, acc := range allAccounts {
		allAccountIDs[i] = acc.ID
	}

	// Get all grouped account IDs
	groupedIDs, err := uc.repo.GetAllGroupedAccountIDs(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate ungrouped accounts (set difference)
	groupedSet := make(map[int64]bool)
	for _, id := range groupedIDs {
		groupedSet[id] = true
	}

	ungroupedIDs := make([]int64, 0)
	for _, id := range allAccountIDs {
		if !groupedSet[id] {
			ungroupedIDs = append(ungroupedIDs, id)
		}
	}

	// Return virtual default group
	return &AccountGroup{
		ID:          0, // Virtual ID
		Name:        "default",
		Description: "未分组账户（默认组）",
		Priority:    -1, // Lowest priority
		AccountIDs:  ungroupedIDs,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// Account is a simplified account model for group members.
type Account struct {
	ID              int64
	Name            string
	Provider        string
	Status          string
	HealthScore     int
	IsCircuitBroken bool
}

// NewValidationError creates a validation error.
func NewValidationError(message string) error {
	// TODO: Use proper error types from pkg/errors
	return &ValidationError{Message: message}
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
