// Package errors provides database error classification and handling utilities.
package errors

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// DatabaseErrorType represents the type of database error.
type DatabaseErrorType int

const (
	// ErrorTypeUnknown represents an unknown database error.
	ErrorTypeUnknown DatabaseErrorType = iota
	// ErrorTypeDuplicateKey represents a duplicate key constraint violation (MySQL 1062).
	ErrorTypeDuplicateKey
	// ErrorTypeConstraintViolation represents a foreign key or check constraint violation.
	ErrorTypeConstraintViolation
	// ErrorTypeInvalidJSON represents an invalid JSON data error (MySQL 3140, 3141).
	ErrorTypeInvalidJSON
	// ErrorTypeDataTooLong represents a data too long error (MySQL 1406).
	ErrorTypeDataTooLong
	// ErrorTypeNotFound represents a record not found error.
	ErrorTypeNotFound
	// ErrorTypeDeadlock represents a deadlock error (MySQL 1213).
	ErrorTypeDeadlock
	// ErrorTypeConnectionError represents a database connection error.
	ErrorTypeConnectionError
	// ErrorTypeInvalidValue represents an invalid value error.
	ErrorTypeInvalidValue
)

// DatabaseError wraps a database error with classification information.
type DatabaseError struct {
	Type         DatabaseErrorType
	OriginalErr  error
	MySQLErrCode uint16 // MySQL error code (e.g., 1062, 3140)
	Message      string
}

// Error implements the error interface.
func (e *DatabaseError) Error() string {
	if e.MySQLErrCode > 0 {
		return fmt.Sprintf("%s (MySQL error %d): %v", e.Message, e.MySQLErrCode, e.OriginalErr)
	}
	return fmt.Sprintf("%s: %v", e.Message, e.OriginalErr)
}

// Unwrap returns the underlying error for errors.Is and errors.As compatibility.
func (e *DatabaseError) Unwrap() error {
	return e.OriginalErr
}

// ClassifyDBError classifies a database error into a specific error type.
//
// It handles GORM errors and MySQL-specific errors:
//   - ErrRecordNotFound → ErrorTypeNotFound
//   - MySQL 1062 (Duplicate entry) → ErrorTypeDuplicateKey
//   - MySQL 3140/3141 (Invalid JSON) → ErrorTypeInvalidJSON
//   - MySQL 1406 (Data too long) → ErrorTypeDataTooLong
//   - MySQL 1452 (Foreign key constraint) → ErrorTypeConstraintViolation
//   - MySQL 1213 (Deadlock) → ErrorTypeDeadlock
//   - Connection errors → ErrorTypeConnectionError
//
// Example:
//
//	err := repo.CreateAccount(ctx, account)
//	if err != nil {
//	    dbErr := errors.ClassifyDBError(err)
//	    switch dbErr.Type {
//	    case errors.ErrorTypeDuplicateKey:
//	        return status.Error(codes.AlreadyExists, "account name already exists")
//	    case errors.ErrorTypeInvalidJSON:
//	        return status.Error(codes.InvalidArgument, "invalid JSON in metadata")
//	    default:
//	        return status.Error(codes.Internal, "database error")
//	    }
//	}
func ClassifyDBError(err error) *DatabaseError {
	if err == nil {
		return nil
	}

	// Handle GORM ErrRecordNotFound
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &DatabaseError{
			Type:        ErrorTypeNotFound,
			OriginalErr: err,
			Message:     "record not found",
		}
	}

	// Try to extract MySQL error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return classifyMySQLError(mysqlErr)
	}

	// Check for connection errors (common patterns)
	errMsg := err.Error()
	if isConnectionError(errMsg) {
		return &DatabaseError{
			Type:        ErrorTypeConnectionError,
			OriginalErr: err,
			Message:     "database connection error",
		}
	}

	// Unknown error type
	return &DatabaseError{
		Type:        ErrorTypeUnknown,
		OriginalErr: err,
		Message:     "unknown database error",
	}
}

// classifyMySQLError classifies a MySQL-specific error.
func classifyMySQLError(err *mysql.MySQLError) *DatabaseError {
	switch err.Number {
	case 1062: // ER_DUP_ENTRY
		return &DatabaseError{
			Type:         ErrorTypeDuplicateKey,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "duplicate key constraint violation",
		}

	case 3140, 3141, 3142, 3143: // JSON-related errors
		// 3140: Invalid JSON text
		// 3141: Invalid JSON path
		// 3142: JSON document too large
		// 3143: Invalid JSON type
		return &DatabaseError{
			Type:         ErrorTypeInvalidJSON,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "invalid JSON data",
		}

	case 1406: // ER_DATA_TOO_LONG
		return &DatabaseError{
			Type:         ErrorTypeDataTooLong,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "data too long for column",
		}

	case 1452: // ER_NO_REFERENCED_ROW_2 (Foreign key constraint)
		return &DatabaseError{
			Type:         ErrorTypeConstraintViolation,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "foreign key constraint violation",
		}

	case 1451: // ER_ROW_IS_REFERENCED_2 (Cannot delete/update parent row)
		return &DatabaseError{
			Type:         ErrorTypeConstraintViolation,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "cannot delete/update record due to foreign key constraint",
		}

	case 1213: // ER_LOCK_DEADLOCK
		return &DatabaseError{
			Type:         ErrorTypeDeadlock,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "deadlock detected",
		}

	case 1048: // ER_BAD_NULL_ERROR
		return &DatabaseError{
			Type:         ErrorTypeInvalidValue,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "column cannot be null",
		}

	case 1265, 1366: // ER_WARN_DATA_TRUNCATED, ER_TRUNCATED_WRONG_VALUE
		return &DatabaseError{
			Type:         ErrorTypeInvalidValue,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "invalid or truncated value",
		}

	default:
		// Unknown MySQL error
		return &DatabaseError{
			Type:         ErrorTypeUnknown,
			OriginalErr:  err,
			MySQLErrCode: err.Number,
			Message:      "MySQL error",
		}
	}
}

// isConnectionError checks if the error message indicates a connection problem.
func isConnectionError(errMsg string) bool {
	connectionKeywords := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"timeout",
		"connection lost",
		"can't connect",
		"dial tcp",
	}

	for _, keyword := range connectionKeywords {
		if len(errMsg) > 0 && contains(errMsg, keyword) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive).
func contains(str, substr string) bool {
	// Simple case-insensitive check
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := str[i+j]
			c2 := substr[j]
			// Convert to lowercase
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// IsDuplicateKeyError checks if the error is a duplicate key constraint violation.
func IsDuplicateKeyError(err error) bool {
	dbErr := ClassifyDBError(err)
	return dbErr != nil && dbErr.Type == ErrorTypeDuplicateKey
}

// IsNotFoundError checks if the error is a record not found error.
func IsNotFoundError(err error) bool {
	dbErr := ClassifyDBError(err)
	return dbErr != nil && dbErr.Type == ErrorTypeNotFound
}

// IsInvalidJSONError checks if the error is an invalid JSON error.
func IsInvalidJSONError(err error) bool {
	dbErr := ClassifyDBError(err)
	return dbErr != nil && dbErr.Type == ErrorTypeInvalidJSON
}

// IsConstraintViolationError checks if the error is a constraint violation.
func IsConstraintViolationError(err error) bool {
	dbErr := ClassifyDBError(err)
	return dbErr != nil && dbErr.Type == ErrorTypeConstraintViolation
}

// IsDeadlockError checks if the error is a deadlock error.
func IsDeadlockError(err error) bool {
	dbErr := ClassifyDBError(err)
	return dbErr != nil && dbErr.Type == ErrorTypeDeadlock
}
