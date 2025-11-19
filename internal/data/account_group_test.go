package data

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"QuotaLane/pkg/errors"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupGroupTestDB creates a test database connection with sqlmock
func setupGroupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	cleanup := func() {
		sqlDB.Close()
	}

	return gormDB, mock, cleanup
}

// setupGroupTestRedis creates a test Redis client with miniredis
func setupGroupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis, func()) {
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, mr, cleanup
}

// setupAccountGroupRepo creates a test AccountGroupRepo instance
func setupAccountGroupRepo(t *testing.T) (*AccountGroupRepo, sqlmock.Sqlmock, *miniredis.Miniredis, func()) {
	gormDB, mock, dbCleanup := setupGroupTestDB(t)
	redisClient, mr, redisCleanup := setupGroupTestRedis(t)

	data := &Data{
		redisClient: redisClient,
		cache:       nil, // not used in these tests
	}

	repo := NewAccountGroupRepo(data, gormDB, log.DefaultLogger)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return repo, mock, mr, cleanup
}

// TestCreateGroup tests creating an account group with members
func TestCreateGroup(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create group with members", func(t *testing.T) {
		mr.FlushAll() // Clear Redis

		// Mock transaction begin
		mock.ExpectBegin()

		// Mock INSERT for account_groups (includes deleted_at as NULL)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `account_groups`")).
			WithArgs("test-group", "Test description", int32(100), sqlmock.AnyArg(), sqlmock.AnyArg(), nil).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Mock INSERT for account_group_members
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `account_group_members`")).
			WithArgs(int64(1), int64(10), sqlmock.AnyArg(), int64(1), int64(20), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 2))

		// Mock transaction commit
		mock.ExpectCommit()

		groupID, err := repo.CreateGroup(ctx, "test-group", "Test description", 100, []int64{10, 20})

		assert.NoError(t, err)
		assert.Equal(t, int64(1), groupID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("create group without members", func(t *testing.T) {
		mr.FlushAll()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `account_groups`")).
			WithArgs("test-group-2", "Empty group", int32(50), sqlmock.AnyArg(), sqlmock.AnyArg(), nil).
			WillReturnResult(sqlmock.NewResult(2, 1))
		mock.ExpectCommit()

		groupID, err := repo.CreateGroup(ctx, "test-group-2", "Empty group", 50, []int64{})

		assert.NoError(t, err)
		assert.Equal(t, int64(2), groupID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("create group fails on insert", func(t *testing.T) {
		mr.FlushAll()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `account_groups`")).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		groupID, err := repo.CreateGroup(ctx, "fail-group", "Fail", 10, []int64{})

		assert.Error(t, err)
		assert.Equal(t, int64(0), groupID)
		assert.IsType(t, &errors.DatabaseError{}, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestGetGroup tests retrieving a group by ID
func TestGetGroup(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get group from database", func(t *testing.T) {
		mr.FlushAll() // Ensure cache miss

		groupID := int64(1)
		now := time.Now()

		// Mock SELECT account_groups
		rows := sqlmock.NewRows([]string{"id", "name", "description", "priority", "created_at", "updated_at", "deleted_at"}).
			AddRow(groupID, "production", "Production accounts", int32(200), now, now, nil)

		// GORM's First() adds ORDER BY and LIMIT as parameters
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_groups` WHERE id = ? AND deleted_at IS NULL ORDER BY `account_groups`.`id` LIMIT ?")).
			WithArgs(groupID, 1).
			WillReturnRows(rows)

		// Mock SELECT account_group_members
		memberRows := sqlmock.NewRows([]string{"group_id", "account_id", "created_at"}).
			AddRow(groupID, int64(10), now).
			AddRow(groupID, int64(20), now)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_group_members` WHERE group_id = ?")).
			WithArgs(groupID).
			WillReturnRows(memberRows)

		group, err := repo.GetGroup(ctx, groupID)

		assert.NoError(t, err)
		assert.NotNil(t, group)
		assert.Equal(t, groupID, group.ID)
		assert.Equal(t, "production", group.Name)
		assert.Equal(t, int32(200), group.Priority)
		assert.Len(t, group.AccountIDs, 2)
		assert.Contains(t, group.AccountIDs, int64(10))
		assert.Contains(t, group.AccountIDs, int64(20))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("get group not found", func(t *testing.T) {
		mr.FlushAll()

		// GORM's First() adds ORDER BY and LIMIT as parameters
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_groups` WHERE id = ? AND deleted_at IS NULL ORDER BY `account_groups`.`id` LIMIT ?")).
			WithArgs(int64(999), 1).
			WillReturnError(gorm.ErrRecordNotFound)

		group, err := repo.GetGroup(ctx, 999)

		assert.Error(t, err)
		assert.Nil(t, group)
		assert.IsType(t, &errors.DatabaseError{}, err)
		dbErr := err.(*errors.DatabaseError)
		assert.Equal(t, errors.ErrorTypeNotFound, dbErr.Type)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestListGroups tests listing groups with pagination
func TestListGroups(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("list groups with pagination", func(t *testing.T) {
		mr.FlushAll()

		now := time.Now()

		// Mock COUNT query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(int64(5))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `account_groups` WHERE deleted_at IS NULL")).
			WillReturnRows(countRows)

		// Mock SELECT query with pagination
		// Note: GORM only includes LIMIT when offset is 0, no OFFSET clause
		groupRows := sqlmock.NewRows([]string{"id", "name", "description", "priority", "created_at", "updated_at", "deleted_at"}).
			AddRow(int64(1), "group1", "desc1", int32(100), now, now, nil).
			AddRow(int64(2), "group2", "desc2", int32(50), now, now, nil)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_groups` WHERE deleted_at IS NULL ORDER BY priority DESC, created_at DESC LIMIT ?")).
			WithArgs(2).
			WillReturnRows(groupRows)

		groups, total, err := repo.ListGroups(ctx, 1, 2)

		assert.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, groups, 2)
		assert.Equal(t, "group1", groups[0].Name)
		assert.Equal(t, int32(100), groups[0].Priority)
		assert.Equal(t, "group2", groups[1].Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestUpdateGroup tests updating a group
func TestUpdateGroup(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("update group successfully", func(t *testing.T) {
		mr.FlushAll()

		groupID := int64(1)
		now := time.Now()

		// Mock GetGroup (to get old members)
		groupRows := sqlmock.NewRows([]string{"id", "name", "description", "priority", "created_at", "updated_at", "deleted_at"}).
			AddRow(groupID, "old-name", "old-desc", int32(100), now, now, nil)
		// GORM's First() adds LIMIT 1, so we need to expect 2 arguments: id and limit
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_groups` WHERE id = ? AND deleted_at IS NULL")).
			WithArgs(groupID, 1).
			WillReturnRows(groupRows)

		memberRows := sqlmock.NewRows([]string{"group_id", "account_id", "created_at"}).
			AddRow(groupID, int64(10), now)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_group_members` WHERE group_id = ?")).
			WithArgs(groupID).
			WillReturnRows(memberRows)

		// Mock transaction for update
		mock.ExpectBegin()

		// Mock UPDATE account_groups
		// GORM sets fields in alphabetical order: description, name, priority, updated_at
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `account_groups` SET")).
			WithArgs("new-desc", "new-name", int32(150), sqlmock.AnyArg(), groupID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock DELETE old members
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `account_group_members` WHERE group_id = ?")).
			WithArgs(groupID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock INSERT new members
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `account_group_members`")).
			WithArgs(groupID, int64(20), sqlmock.AnyArg(), groupID, int64(30), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 2))

		mock.ExpectCommit()

		err := repo.UpdateGroup(ctx, groupID, "new-name", "new-desc", 150, []int64{20, 30})

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestDeleteGroup tests soft deleting a group
func TestDeleteGroup(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete group successfully", func(t *testing.T) {
		mr.FlushAll()

		groupID := int64(1)
		now := time.Now()

		// Mock GetGroup
		groupRows := sqlmock.NewRows([]string{"id", "name", "description", "priority", "created_at", "updated_at", "deleted_at"}).
			AddRow(groupID, "to-delete", "delete me", int32(100), now, now, nil)
		// GORM's First() adds LIMIT 1, so we need to expect 2 arguments: id and limit
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_groups` WHERE id = ? AND deleted_at IS NULL")).
			WithArgs(groupID, 1).
			WillReturnRows(groupRows)

		memberRows := sqlmock.NewRows([]string{"group_id", "account_id", "created_at"}).
			AddRow(groupID, int64(10), now)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `account_group_members` WHERE group_id = ?")).
			WithArgs(groupID).
			WillReturnRows(memberRows)

		// Mock soft delete UPDATE (GORM wraps UPDATE in transaction)
		// GORM automatically updates both deleted_at and updated_at
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `account_groups` SET `deleted_at`=?,`updated_at`=? WHERE id = ? AND deleted_at IS NULL")).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), groupID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		// Mock hard delete members (GORM wraps delete in transaction)
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `account_group_members` WHERE group_id = ?")).
			WithArgs(groupID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := repo.DeleteGroup(ctx, groupID)

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestGetAccountGroups tests getting groups for an account
func TestGetAccountGroups(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get account groups from database", func(t *testing.T) {
		mr.FlushAll()

		accountID := int64(10)
		now := time.Now()

		// Mock JOIN query (GORM uses explicit column names instead of *)
		groupRows := sqlmock.NewRows([]string{"id", "name", "description", "priority", "created_at", "updated_at", "deleted_at"}).
			AddRow(int64(1), "group1", "desc1", int32(100), now, now, nil).
			AddRow(int64(2), "group2", "desc2", int32(50), now, now, nil)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT `account_groups`.`id`,`account_groups`.`name`,`account_groups`.`description`,`account_groups`.`priority`,`account_groups`.`created_at`,`account_groups`.`updated_at`,`account_groups`.`deleted_at` FROM `account_groups` JOIN account_group_members ON account_groups.id = account_group_members.group_id WHERE account_group_members.account_id = ? AND account_groups.deleted_at IS NULL ORDER BY account_groups.priority DESC")).
			WithArgs(accountID).
			WillReturnRows(groupRows)

		groups, err := repo.GetAccountGroups(ctx, accountID)

		assert.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.Equal(t, "group1", groups[0].Name)
		assert.Equal(t, int32(100), groups[0].Priority)
		assert.Equal(t, "group2", groups[1].Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestGetAllGroupedAccountIDs tests getting all grouped account IDs
func TestGetAllGroupedAccountIDs(t *testing.T) {
	repo, mock, mr, cleanup := setupAccountGroupRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get all grouped account IDs", func(t *testing.T) {
		mr.FlushAll()

		// Mock JOIN query with DISTINCT
		memberRows := sqlmock.NewRows([]string{"account_id"}).
			AddRow(int64(10)).
			AddRow(int64(20)).
			AddRow(int64(30))

		mock.ExpectQuery(regexp.QuoteMeta("SELECT DISTINCT account_id FROM `account_group_members` JOIN account_groups ON account_group_members.group_id = account_groups.id WHERE account_groups.deleted_at IS NULL")).
			WillReturnRows(memberRows)

		accountIDs, err := repo.GetAllGroupedAccountIDs(ctx)

		assert.NoError(t, err)
		assert.Len(t, accountIDs, 3)
		assert.Contains(t, accountIDs, int64(10))
		assert.Contains(t, accountIDs, int64(20))
		assert.Contains(t, accountIDs, int64(30))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
