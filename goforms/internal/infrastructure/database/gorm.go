// Package database provides database connection and ORM utilities for the application.
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/infrastructure/config"
	"github.com/goformx/goforms/internal/infrastructure/logging"
)

const (
	// DefaultPingTimeout is the default timeout for database ping operations
	DefaultPingTimeout = 5 * time.Second
	// ConnectionPoolWarningThreshold is the percentage of max connections that triggers a warning
	ConnectionPoolWarningThreshold = 0.8
	// ConnectionPoolPercentageMultiplier is used to convert ratio to percentage
	ConnectionPoolPercentageMultiplier = 100
)

// GormDB wraps the GORM database connection
type GormDB struct {
	*gorm.DB
	logger logging.Logger
}

// TickerDuration controls how often the connection pool is monitored
var TickerDuration = 1 * time.Minute

// New creates a new GORM database connection
func New(cfg *config.Config, appLogger logging.Logger) (*GormDB, error) {
	// Configure GORM logger
	gormLogger := configureGormLogger(cfg, appLogger)

	// Configure GORM
	gormConfig := &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt: true, // Enable prepared statements for better performance
	}

	// Create database connection
	db, err := createDatabaseConnection(cfg, gormConfig)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	if poolErr := configureConnectionPool(db, cfg); poolErr != nil {
		return nil, poolErr
	}

	// Verify connection
	if verifyErr := verifyConnection(db, appLogger); verifyErr != nil {
		return nil, verifyErr
	}

	appLogger.Info("database connection established",
		"driver", cfg.Database.Driver,
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"max_open_conns", cfg.Database.MaxOpenConns)

	return &GormDB{
		DB:     db,
		logger: appLogger,
	}, nil
}

// createDatabaseConnection creates a database connection based on the configuration
func createDatabaseConnection(cfg *config.Config, gormConfig *gorm.Config) (*gorm.DB, error) {
	var db *gorm.DB

	var err error

	// Create database connection based on the selected driver
	switch cfg.Database.Driver {
	case "postgres":
		dsn := buildPostgresDSN(cfg)
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database connection type: %s", cfg.Database.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// buildPostgresDSN builds the PostgreSQL connection string
func buildPostgresDSN(cfg *config.Config) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)
}

// configureConnectionPool configures the database connection pool
func configureConnectionPool(db *gorm.DB, cfg *config.Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	return nil
}

// verifyConnection verifies the database connection by pinging it
func verifyConnection(db *gorm.DB, appLogger logging.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if pingErr := sqlDB.Ping(); pingErr != nil {
		appLogger.Error("failed to ping database", "error", pingErr)

		return fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return nil
}

// Close closes the database connection
func (db *GormDB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if closeErr := sqlDB.Close(); closeErr != nil {
		db.logger.Error("failed to close database connection", "error", closeErr)

		return fmt.Errorf("failed to close database connection: %w", closeErr)
	}

	return nil
}

// MonitorConnectionPool monitors the database connection pool and logs metrics
func (db *GormDB) MonitorConnectionPool(ctx context.Context) {
	db.logger.Debug("starting MonitorConnectionPool")

	ticker := time.NewTicker(TickerDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			db.logger.Debug("MonitorConnectionPool context done")

			return
		case <-ticker.C:
			db.logger.Debug("MonitorConnectionPool tick")
			db.collectAndLogMetrics()
		}
	}
}

// collectAndLogMetrics collects and logs database connection pool metrics
func (db *GormDB) collectAndLogMetrics() {
	db.logger.Debug("collectAndLogMetrics called")

	sqlDB, err := db.DB.DB()
	if err != nil {
		db.logger.Error("failed to get database instance", map[string]any{"error": err})

		return
	}

	stats := sqlDB.Stats()
	metrics := map[string]any{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}

	// Add database-specific metrics
	db.addDatabaseSpecificMetrics(metrics)

	// Log the metrics
	db.logger.Info("database connection pool status", map[string]any{"metrics": metrics})

	// Check for high usage
	if float64(stats.InUse)/float64(stats.MaxOpenConnections) > ConnectionPoolWarningThreshold {
		db.logger.Warn("database connection pool usage is high",
			map[string]any{
				"in_use":   stats.InUse,
				"max_open": stats.MaxOpenConnections,
			})
	}

	// Check for long wait times
	if stats.WaitDuration > time.Second*5 {
		db.logger.Warn("database connection wait time is high",
			map[string]any{
				"wait_duration": stats.WaitDuration,
				"wait_count":    stats.WaitCount,
			})
	}
}

// addDatabaseSpecificMetrics adds database-specific metrics to the metrics map
func (db *GormDB) addDatabaseSpecificMetrics(metrics map[string]any) {
	switch db.Name() {
	case "postgres":
		db.addPostgresMetrics(metrics)
	case "mysql":
		db.addMySQLMetrics(metrics)
	}
}

// addPostgresMetrics adds PostgreSQL-specific metrics
func (db *GormDB) addPostgresMetrics(metrics map[string]any) {
	var pgStats struct {
		ActiveConnections  int64
		IdleConnections    int64
		WaitingConnections int64
	}

	// Get active connections
	if err := db.DB.Raw(
		"SELECT count(*) as active_connections FROM pg_stat_activity WHERE state = 'active'",
	).Scan(&pgStats.ActiveConnections).Error; err == nil {
		metrics["postgres_active_connections"] = pgStats.ActiveConnections
	}

	// Get idle connections
	if err := db.DB.Raw(
		"SELECT count(*) as idle_connections FROM pg_stat_activity WHERE state = 'idle'",
	).Scan(&pgStats.IdleConnections).Error; err == nil {
		metrics["postgres_idle_connections"] = pgStats.IdleConnections
	}

	// Get waiting connections
	if err := db.DB.Raw(
		"SELECT count(*) as waiting_connections FROM pg_stat_activity WHERE wait_event_type IS NOT NULL",
	).Scan(&pgStats.WaitingConnections).Error; err == nil {
		metrics["postgres_waiting_connections"] = pgStats.WaitingConnections
	}
}

// addMySQLMetrics adds MySQL-specific metrics
func (db *GormDB) addMySQLMetrics(metrics map[string]any) {
	var mysqlStats []struct {
		VariableName string
		Value        string
	}

	if err := db.DB.Raw(
		"SHOW STATUS WHERE Variable_name IN ('Threads_connected', 'Threads_running', 'Threads_waiting')",
	).Scan(&mysqlStats).Error; err == nil {
		for _, stat := range mysqlStats {
			metrics["mysql_"+strings.ToLower(stat.VariableName)] = stat.Value
		}
	}
}

// Ping checks the database connection by executing a simple query
func (db *GormDB) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, DefaultPingTimeout)
	defer cancel()

	return db.DB.WithContext(pingCtx).Raw("SELECT 1").Error
}

// NewWithDB creates a new GormDB instance with an existing DB connection
func NewWithDB(db *gorm.DB, appLogger logging.Logger) *GormDB {
	return &GormDB{
		DB:     db,
		logger: appLogger,
	}
}

// GetDB returns the underlying GORM DB instance
func (db *GormDB) GetDB() *gorm.DB {
	return db.DB
}
