package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "QuotaLane/api/v1"
	pkgerrors "QuotaLane/pkg/errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// AccountGroup is the GORM model for account_groups table.
type AccountGroup struct {
	ID          int64      `gorm:"primaryKey;column:id"`
	Name        string     `gorm:"column:name;size:100;not null;index:idx_name"`
	Description string     `gorm:"column:description;type:text"`
	Priority    int32      `gorm:"column:priority;default:0;not null;index:idx_priority"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime;index:idx_created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"` // 软删除字段
}

// TableName specifies the table name for GORM.
func (AccountGroup) TableName() string {
	return "account_groups"
}

// AccountGroupMember is the GORM model for account_group_members table.
type AccountGroupMember struct {
	GroupID   int64     `gorm:"primaryKey;column:group_id"`
	AccountID int64     `gorm:"primaryKey;column:account_id;index:idx_account_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the table name for GORM.
func (AccountGroupMember) TableName() string {
	return "account_group_members"
}

// AccountGroupData represents account group data with member IDs.
// This serves as the domain model used by the biz layer.
type AccountGroupData struct {
	ID          int64
	Name        string
	Description string
	Priority    int32
	AccountIDs  []int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AccountGroupRepo implementation using GORM and Redis.
type AccountGroupRepo struct {
	data *Data
	db   *gorm.DB
	log  *log.Helper
}

// NewAccountGroupRepo creates a new account group repository.
func NewAccountGroupRepo(data *Data, db *gorm.DB, logger log.Logger) *AccountGroupRepo {
	return &AccountGroupRepo{
		data: data,
		db:   db,
		log:  log.NewHelper(log.With(logger, "module", "data/account-group")),
	}
}

// CreateGroup creates a new account group with members in a transaction.
func (r *AccountGroupRepo) CreateGroup(ctx context.Context, name string, description string, priority int32, accountIDs []int64) (int64, error) {
	group := &AccountGroupData{
		Name:        name,
		Description: description,
		Priority:    priority,
		AccountIDs:  accountIDs,
	}
	dbGroup := &AccountGroup{
		Name:        group.Name,
		Description: group.Description,
		Priority:    group.Priority,
	}

	// Start transaction
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Insert group
		if err := tx.Create(dbGroup).Error; err != nil {
			r.log.Errorf("failed to create account group: %v", err)
			return &pkgerrors.DatabaseError{
				Type:        pkgerrors.ErrorTypeUnknown,
				OriginalErr: err,
				Message:     "创建账户组失败",
			}
		}

		// 2. Insert members (if any)
		if len(group.AccountIDs) > 0 {
			members := make([]*AccountGroupMember, len(group.AccountIDs))
			for i, accountID := range group.AccountIDs {
				members[i] = &AccountGroupMember{
					GroupID:   dbGroup.ID,
					AccountID: accountID,
				}
			}
			if err := tx.Create(&members).Error; err != nil {
				r.log.Errorf("failed to create group members: %v", err)
				return &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "创建账户组成员失败"}
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// Cache the new group
	r.cacheGroup(ctx, dbGroup.ID, group)

	// Cache group IDs for each account
	for _, accountID := range group.AccountIDs {
		r.invalidateAccountGroupsCache(ctx, accountID)
	}

	return dbGroup.ID, nil
}

// GetGroup retrieves a group by ID with member account IDs.
func (r *AccountGroupRepo) GetGroup(ctx context.Context, id int64) (*AccountGroupData, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("group:%d", id)
	cached, err := r.data.GetRedisClient().Get(ctx, cacheKey).Result()
	if err == nil {
		var group AccountGroupData
		if err := json.Unmarshal([]byte(cached), &group); err == nil {
			r.log.Debugf("cache hit for group %d", id)
			return &group, nil
		}
	}

	// Query database
	var dbGroup AccountGroup
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&dbGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &pkgerrors.DatabaseError{
				Type:        pkgerrors.ErrorTypeNotFound,
				OriginalErr: err,
				Message:     "账户组不存在",
			}
		}
		r.log.Errorf("failed to get group: %v", err)
		return nil, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询账户组失败"}
	}

	// Query members
	var members []*AccountGroupMember
	if err := r.db.Where("group_id = ?", id).Find(&members).Error; err != nil {
		r.log.Errorf("failed to get group members: %v", err)
		return nil, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询账户组成员失败"}
	}

	accountIDs := make([]int64, len(members))
	for i, m := range members {
		accountIDs[i] = m.AccountID
	}

	group := &AccountGroupData{
		ID:          dbGroup.ID,
		Name:        dbGroup.Name,
		Description: dbGroup.Description,
		Priority:    dbGroup.Priority,
		AccountIDs:  accountIDs,
		CreatedAt:   dbGroup.CreatedAt,
		UpdatedAt:   dbGroup.UpdatedAt,
	}

	// Cache the result
	r.cacheGroup(ctx, id, group)

	return group, nil
}

// ListGroups retrieves a paginated list of groups (without members).
func (r *AccountGroupRepo) ListGroups(ctx context.Context, page, pageSize int32) ([]*AccountGroupData, int64, error) {
	var groups []*AccountGroup
	var total int64

	// Count total
	if err := r.db.Model(&AccountGroup{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		r.log.Errorf("failed to count groups: %v", err)
		return nil, 0, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询账户组总数失败"}
	}

	// Query with pagination and sort by priority DESC
	offset := (page - 1) * pageSize
	if err := r.db.Where("deleted_at IS NULL").
		Order("priority DESC, created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&groups).Error; err != nil {
		r.log.Errorf("failed to list groups: %v", err)
		return nil, 0, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询账户组列表失败"}
	}

	// Convert to data model (without members for list view)
	result := make([]*AccountGroupData, len(groups))
	for i, g := range groups {
		result[i] = &AccountGroupData{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Priority:    g.Priority,
			CreatedAt:   g.CreatedAt,
			UpdatedAt:   g.UpdatedAt,
		}
	}

	return result, total, nil
}

// UpdateGroup updates a group and its members in a transaction.
func (r *AccountGroupRepo) UpdateGroup(ctx context.Context, id int64, name string, description string, priority int32, accountIDs []int64) error {
	group := &AccountGroupData{
		ID:          id,
		Name:        name,
		Description: description,
		Priority:    priority,
		AccountIDs:  accountIDs,
	}
	// First get old members for cache invalidation
	oldGroup, err := r.GetGroup(ctx, group.ID)
	if err != nil {
		return err
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Update group metadata
		updates := map[string]interface{}{
			"name":        group.Name,
			"description": group.Description,
			"priority":    group.Priority,
			"updated_at":  time.Now(),
		}
		if err := tx.Model(&AccountGroup{}).Where("id = ? AND deleted_at IS NULL", group.ID).Updates(updates).Error; err != nil {
			r.log.Errorf("failed to update group: %v", err)
			return &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "更新账户组失败"}
		}

		// 2. Delete old members
		if err := tx.Where("group_id = ?", group.ID).Delete(&AccountGroupMember{}).Error; err != nil {
			r.log.Errorf("failed to delete old members: %v", err)
			return &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "删除旧成员失败"}
		}

		// 3. Insert new members
		if len(group.AccountIDs) > 0 {
			members := make([]*AccountGroupMember, len(group.AccountIDs))
			for i, accountID := range group.AccountIDs {
				members[i] = &AccountGroupMember{
					GroupID:   group.ID,
					AccountID: accountID,
				}
			}
			if err := tx.Create(&members).Error; err != nil {
				r.log.Errorf("failed to create new members: %v", err)
				return &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "创建新成员失败"}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate caches
	r.invalidateGroupCache(ctx, group.ID)
	// Invalidate old and new account group caches
	for _, accountID := range oldGroup.AccountIDs {
		r.invalidateAccountGroupsCache(ctx, accountID)
	}
	for _, accountID := range group.AccountIDs {
		r.invalidateAccountGroupsCache(ctx, accountID)
	}

	return nil
}

// DeleteGroup soft deletes a group (sets deleted_at).
func (r *AccountGroupRepo) DeleteGroup(ctx context.Context, id int64) error {
	// Get group first for cache invalidation
	group, err := r.GetGroup(ctx, id)
	if err != nil {
		return err
	}

	// Soft delete (set deleted_at)
	now := time.Now()
	if err := r.db.Model(&AccountGroup{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error; err != nil {
		r.log.Errorf("failed to delete group: %v", err)
		return &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "删除账户组失败"}
	}

	// Members are automatically deleted by ON DELETE CASCADE foreign key constraint
	// But we manually delete them here for clarity
	if err := r.db.Where("group_id = ?", id).Delete(&AccountGroupMember{}).Error; err != nil {
		r.log.Warnf("failed to delete members (should be handled by cascade): %v", err)
	}

	// Invalidate caches
	r.invalidateGroupCache(ctx, id)
	for _, accountID := range group.AccountIDs {
		r.invalidateAccountGroupsCache(ctx, accountID)
	}

	return nil
}

// GetAccountGroups retrieves all groups that an account belongs to.
func (r *AccountGroupRepo) GetAccountGroups(ctx context.Context, accountID int64) ([]*AccountGroupData, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("account:%d:groups", accountID)
	cached, err := r.data.GetRedisClient().Get(ctx, cacheKey).Result()
	if err == nil {
		var groupIDs []int64
		if err := json.Unmarshal([]byte(cached), &groupIDs); err == nil && len(groupIDs) > 0 {
			// Fetch full group details
			var groups []*AccountGroupData
			for _, gid := range groupIDs {
				group, err := r.GetGroup(ctx, gid)
				if err == nil {
					groups = append(groups, group)
				}
			}
			if len(groups) == len(groupIDs) {
				r.log.Debugf("cache hit for account %d groups", accountID)
				return groups, nil
			}
		}
	}

	// Query database: JOIN account_groups with account_group_members
	var dbGroups []*AccountGroup
	if err := r.db.
		Joins("JOIN account_group_members ON account_groups.id = account_group_members.group_id").
		Where("account_group_members.account_id = ? AND account_groups.deleted_at IS NULL", accountID).
		Order("account_groups.priority DESC").
		Find(&dbGroups).Error; err != nil {
		r.log.Errorf("failed to get account groups: %v", err)
		return nil, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询账户所属组失败"}
	}

	// Convert to data model
	groups := make([]*AccountGroupData, len(dbGroups))
	groupIDs := make([]int64, len(dbGroups))
	for i, g := range dbGroups {
		groups[i] = &AccountGroupData{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Priority:    g.Priority,
			CreatedAt:   g.CreatedAt,
			UpdatedAt:   g.UpdatedAt,
		}
		groupIDs[i] = g.ID
	}

	// Cache group IDs (10 minutes TTL)
	if data, err := json.Marshal(groupIDs); err == nil {
		r.data.GetRedisClient().Set(ctx, cacheKey, data, 10*time.Minute)
	}

	return groups, nil
}

// GetAllGroupedAccountIDs retrieves all account IDs that belong to any group.
func (r *AccountGroupRepo) GetAllGroupedAccountIDs(ctx context.Context) ([]int64, error) {
	var members []*AccountGroupMember
	if err := r.db.
		Select("DISTINCT account_id").
		Joins("JOIN account_groups ON account_group_members.group_id = account_groups.id").
		Where("account_groups.deleted_at IS NULL").
		Find(&members).Error; err != nil {
		r.log.Errorf("failed to get grouped account IDs: %v", err)
		return nil, &pkgerrors.DatabaseError{Type: pkgerrors.ErrorTypeUnknown, OriginalErr: err, Message: "查询已分组账户失败"}
	}

	accountIDs := make([]int64, len(members))
	for i, m := range members {
		accountIDs[i] = m.AccountID
	}

	return accountIDs, nil
}

// cacheGroup caches a group for 10 minutes.
func (r *AccountGroupRepo) cacheGroup(ctx context.Context, id int64, group *AccountGroupData) {
	cacheKey := fmt.Sprintf("group:%d", id)
	data, err := json.Marshal(group)
	if err != nil {
		r.log.Warnf("failed to marshal group for cache: %v", err)
		return
	}

	if err := r.data.GetRedisClient().Set(ctx, cacheKey, data, 10*time.Minute).Err(); err != nil {
		// Redis failure is not critical, just log
		r.log.Warnf("failed to cache group %d: %v", id, err)
	}
}

// invalidateGroupCache removes a group from cache.
func (r *AccountGroupRepo) invalidateGroupCache(ctx context.Context, id int64) {
	cacheKey := fmt.Sprintf("group:%d", id)
	if err := r.data.GetRedisClient().Del(ctx, cacheKey).Err(); err != nil && err != redis.Nil {
		r.log.Warnf("failed to invalidate group cache: %v", err)
	}
}

// invalidateAccountGroupsCache removes account's groups cache.
func (r *AccountGroupRepo) invalidateAccountGroupsCache(ctx context.Context, accountID int64) {
	cacheKey := fmt.Sprintf("account:%d:groups", accountID)
	if err := r.data.GetRedisClient().Del(ctx, cacheKey).Err(); err != nil && err != redis.Nil {
		r.log.Warnf("failed to invalidate account groups cache: %v", err)
	}
}

// AccountGroupToProto converts AccountGroupData to Proto message.
func AccountGroupToProto(group *AccountGroupData) *v1.AccountGroup {
	return &v1.AccountGroup{
		Id:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		Priority:    group.Priority,
		AccountIds:  group.AccountIDs,
		CreatedAt:   timestamppb.New(group.CreatedAt),
		UpdatedAt:   timestamppb.New(group.UpdatedAt),
	}
}
