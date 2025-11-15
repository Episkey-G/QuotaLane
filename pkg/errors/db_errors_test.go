package errors

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestClassifyDBError_GORMRecordNotFound(t *testing.T) {
	err := gorm.ErrRecordNotFound
	dbErr := ClassifyDBError(err)

	assert.NotNil(t, dbErr)
	assert.Equal(t, ErrorTypeNotFound, dbErr.Type)
	assert.Equal(t, "record not found", dbErr.Message)
	assert.True(t, errors.Is(dbErr.OriginalErr, gorm.ErrRecordNotFound))
}

func TestClassifyDBError_MySQLDuplicateKey(t *testing.T) {
	mysqlErr := &mysql.MySQLError{
		Number:  1062,
		Message: "Duplicate entry 'test-account' for key 'name'",
	}

	dbErr := ClassifyDBError(mysqlErr)

	assert.NotNil(t, dbErr)
	assert.Equal(t, ErrorTypeDuplicateKey, dbErr.Type)
	assert.Equal(t, uint16(1062), dbErr.MySQLErrCode)
	assert.Equal(t, "duplicate key constraint violation", dbErr.Message)
	assert.Contains(t, dbErr.Error(), "MySQL error 1062")
}

func TestClassifyDBError_MySQLInvalidJSON(t *testing.T) {
	tests := []struct {
		name     string
		errCode  uint16
		expected DatabaseErrorType
	}{
		{
			name:     "Invalid JSON text (3140)",
			errCode:  3140,
			expected: ErrorTypeInvalidJSON,
		},
		{
			name:     "Invalid JSON path (3141)",
			errCode:  3141,
			expected: ErrorTypeInvalidJSON,
		},
		{
			name:     "JSON document too large (3142)",
			errCode:  3142,
			expected: ErrorTypeInvalidJSON,
		},
		{
			name:     "Invalid JSON type (3143)",
			errCode:  3143,
			expected: ErrorTypeInvalidJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlErr := &mysql.MySQLError{
				Number:  tt.errCode,
				Message: "Invalid JSON",
			}

			dbErr := ClassifyDBError(mysqlErr)

			assert.NotNil(t, dbErr)
			assert.Equal(t, tt.expected, dbErr.Type)
			assert.Equal(t, tt.errCode, dbErr.MySQLErrCode)
			assert.Equal(t, "invalid JSON data", dbErr.Message)
		})
	}
}

func TestClassifyDBError_MySQLDataTooLong(t *testing.T) {
	mysqlErr := &mysql.MySQLError{
		Number:  1406,
		Message: "Data too long for column 'name' at row 1",
	}

	dbErr := ClassifyDBError(mysqlErr)

	assert.NotNil(t, dbErr)
	assert.Equal(t, ErrorTypeDataTooLong, dbErr.Type)
	assert.Equal(t, uint16(1406), dbErr.MySQLErrCode)
	assert.Equal(t, "data too long for column", dbErr.Message)
}

func TestClassifyDBError_MySQLForeignKeyConstraint(t *testing.T) {
	tests := []struct {
		name    string
		errCode uint16
		message string
	}{
		{
			name:    "Cannot add child row (1452)",
			errCode: 1452,
			message: "foreign key constraint violation",
		},
		{
			name:    "Cannot delete parent row (1451)",
			errCode: 1451,
			message: "cannot delete/update record due to foreign key constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlErr := &mysql.MySQLError{
				Number:  tt.errCode,
				Message: "Foreign key constraint fails",
			}

			dbErr := ClassifyDBError(mysqlErr)

			assert.NotNil(t, dbErr)
			assert.Equal(t, ErrorTypeConstraintViolation, dbErr.Type)
			assert.Equal(t, tt.errCode, dbErr.MySQLErrCode)
			assert.Equal(t, tt.message, dbErr.Message)
		})
	}
}

func TestClassifyDBError_MySQLDeadlock(t *testing.T) {
	mysqlErr := &mysql.MySQLError{
		Number:  1213,
		Message: "Deadlock found when trying to get lock",
	}

	dbErr := ClassifyDBError(mysqlErr)

	assert.NotNil(t, dbErr)
	assert.Equal(t, ErrorTypeDeadlock, dbErr.Type)
	assert.Equal(t, uint16(1213), dbErr.MySQLErrCode)
	assert.Equal(t, "deadlock detected", dbErr.Message)
}

func TestClassifyDBError_MySQLInvalidValue(t *testing.T) {
	tests := []struct {
		name    string
		errCode uint16
		message string
	}{
		{
			name:    "Column cannot be null (1048)",
			errCode: 1048,
			message: "column cannot be null",
		},
		{
			name:    "Data truncated (1265)",
			errCode: 1265,
			message: "invalid or truncated value",
		},
		{
			name:    "Truncated wrong value (1366)",
			errCode: 1366,
			message: "invalid or truncated value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlErr := &mysql.MySQLError{
				Number:  tt.errCode,
				Message: "Invalid value",
			}

			dbErr := ClassifyDBError(mysqlErr)

			assert.NotNil(t, dbErr)
			assert.Equal(t, ErrorTypeInvalidValue, dbErr.Type)
			assert.Equal(t, tt.errCode, dbErr.MySQLErrCode)
			assert.Equal(t, tt.message, dbErr.Message)
		})
	}
}

func TestClassifyDBError_ConnectionError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
	}{
		{
			name:   "Connection refused",
			errMsg: "dial tcp: connection refused",
		},
		{
			name:   "Connection reset",
			errMsg: "read tcp: connection reset by peer",
		},
		{
			name:   "Broken pipe",
			errMsg: "write tcp: broken pipe",
		},
		{
			name:   "Timeout",
			errMsg: "i/o timeout",
		},
		{
			name:   "No such host",
			errMsg: "dial tcp: lookup mysql.example.com: no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			dbErr := ClassifyDBError(err)

			assert.NotNil(t, dbErr)
			assert.Equal(t, ErrorTypeConnectionError, dbErr.Type)
			assert.Equal(t, "database connection error", dbErr.Message)
		})
	}
}

func TestClassifyDBError_UnknownError(t *testing.T) {
	err := errors.New("some random error")
	dbErr := ClassifyDBError(err)

	assert.NotNil(t, dbErr)
	assert.Equal(t, ErrorTypeUnknown, dbErr.Type)
	assert.Equal(t, "unknown database error", dbErr.Message)
}

func TestClassifyDBError_Nil(t *testing.T) {
	dbErr := ClassifyDBError(nil)
	assert.Nil(t, dbErr)
}

func TestDatabaseError_Error(t *testing.T) {
	t.Run("with MySQL error code", func(t *testing.T) {
		dbErr := &DatabaseError{
			Type:         ErrorTypeDuplicateKey,
			OriginalErr:  errors.New("original error"),
			MySQLErrCode: 1062,
			Message:      "duplicate key",
		}

		errMsg := dbErr.Error()
		assert.Contains(t, errMsg, "duplicate key")
		assert.Contains(t, errMsg, "MySQL error 1062")
		assert.Contains(t, errMsg, "original error")
	})

	t.Run("without MySQL error code", func(t *testing.T) {
		dbErr := &DatabaseError{
			Type:        ErrorTypeNotFound,
			OriginalErr: gorm.ErrRecordNotFound,
			Message:     "record not found",
		}

		errMsg := dbErr.Error()
		assert.Contains(t, errMsg, "record not found")
		assert.NotContains(t, errMsg, "MySQL error")
	})
}

func TestDatabaseError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	dbErr := &DatabaseError{
		OriginalErr: originalErr,
	}

	assert.True(t, errors.Is(dbErr, originalErr))
	assert.Equal(t, originalErr, dbErr.Unwrap())
}

func TestIsDuplicateKeyError(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1062}
	assert.True(t, IsDuplicateKeyError(mysqlErr))

	otherErr := errors.New("other error")
	assert.False(t, IsDuplicateKeyError(otherErr))

	assert.False(t, IsDuplicateKeyError(nil))
}

func TestIsNotFoundError(t *testing.T) {
	assert.True(t, IsNotFoundError(gorm.ErrRecordNotFound))

	otherErr := errors.New("other error")
	assert.False(t, IsNotFoundError(otherErr))

	assert.False(t, IsNotFoundError(nil))
}

func TestIsInvalidJSONError(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 3140}
	assert.True(t, IsInvalidJSONError(mysqlErr))

	otherErr := errors.New("other error")
	assert.False(t, IsInvalidJSONError(otherErr))

	assert.False(t, IsInvalidJSONError(nil))
}

func TestIsConstraintViolationError(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1452}
	assert.True(t, IsConstraintViolationError(mysqlErr))

	otherErr := errors.New("other error")
	assert.False(t, IsConstraintViolationError(otherErr))

	assert.False(t, IsConstraintViolationError(nil))
}

func TestIsDeadlockError(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1213}
	assert.True(t, IsDeadlockError(mysqlErr))

	otherErr := errors.New("other error")
	assert.False(t, IsDeadlockError(otherErr))

	assert.False(t, IsDeadlockError(nil))
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			str:      "connection refused",
			substr:   "connection refused",
			expected: true,
		},
		{
			name:     "case insensitive match",
			str:      "Connection Refused",
			substr:   "connection refused",
			expected: true,
		},
		{
			name:     "substring match",
			str:      "dial tcp: connection refused",
			substr:   "connection refused",
			expected: true,
		},
		{
			name:     "no match",
			str:      "some other error",
			substr:   "connection refused",
			expected: false,
		},
		{
			name:     "empty substring",
			str:      "test",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			str:      "",
			substr:   "test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.str, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
