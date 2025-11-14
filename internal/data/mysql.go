package data

import (
	"QuotaLane/internal/conf"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewMySQLClient creates a new GORM MySQL client.
// The connection is created based on the configuration in conf.Data.
func NewMySQLClient(c *conf.Data, l log.Logger) (*gorm.DB, func(), error) {
	helper := log.NewHelper(l)

	if c.Database == nil {
		helper.Error("database configuration is missing")
		return nil, nil, fmt.Errorf("database configuration is required")
	}

	// Parse DSN and create GORM logger
	gormLogger := logger.New(
		&gormLogAdapter{helper: helper},
		logger.Config{
			SlowThreshold:             200 * time.Millisecond, // Slow SQL threshold
			LogLevel:                  logger.Warn,            // Log level: Warn only
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound
			Colorful:                  false,                  // Disable color
		},
	)

	// Open MySQL connection
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true, // Disable default transaction for better performance
		PrepareStmt:            true, // Prepare statement cache
	})
	if err != nil {
		helper.Errorf("failed to connect to MySQL: %v", err)
		return nil, nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		helper.Errorf("failed to get sql.DB: %v", err)
		return nil, nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)                  // Max idle connections
	sqlDB.SetMaxOpenConns(100)                 // Max open connections
	sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connection max lifetime

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		helper.Errorf("failed to ping MySQL: %v", err)
		return nil, nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	helper.Info("MySQL connection established successfully")

	cleanup := func() {
		helper.Info("closing MySQL connection")
		if err := sqlDB.Close(); err != nil {
			helper.Errorf("failed to close MySQL: %v", err)
		}
	}

	return db, cleanup, nil
}

// gormLogAdapter adapts Kratos log.Helper to GORM logger interface.
type gormLogAdapter struct {
	helper *log.Helper
}

// Printf implements gorm/logger.Writer interface.
func (g *gormLogAdapter) Printf(format string, v ...interface{}) {
	g.helper.Infof(format, v...)
}
