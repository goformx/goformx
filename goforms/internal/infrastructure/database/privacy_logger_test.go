package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/goformx/goforms/internal/infrastructure/config"
	"github.com/goformx/goforms/internal/infrastructure/logging"
)

func observedLogger(t *testing.T) (logging.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, observed := observer.New(zapcore.DebugLevel)
	factory, err := logging.NewFactory(&logging.FactoryConfig{AppName: "privacy-test", Environment: "production", LogLevel: "debug"}, nil)
	require.NoError(t, err)
	output, err := factory.WithTestCore(core).CreateLogger()
	require.NoError(t, err)
	return output, observed
}

func TestProductionORMLoggerExcludesPayloadsAndDriverMessages(t *testing.T) {
	for _, parameterized := range []bool{false, true} {
		t.Run(map[bool]string{false: "default_config", true: "parameterized"}[parameterized], func(t *testing.T) {
			appLogger, observed := observedLogger(t)
			cfg := &config.Config{}
			cfg.Database.Logging.LogLevel = "info"
			cfg.Database.Logging.Parameterized = parameterized
			ormLogger := configureGormLogger(cfg, appLogger)
			db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=unused dbname=unused"}), &gorm.Config{
				DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: ormLogger,
			})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.WithContext(t.Context()).Exec("SELECT ?", "private-canary-payload").Error)
			ormLogger.Trace(t.Context(), time.Now().Add(-time.Second), func() (string, int64) {
				return "SELECT 'private-canary-literal'", 0
			}, errors.New("database rejected private-canary-driver-message"))
			require.NotZero(t, observed.Len(), "Telemetry must remain observable")
			encoded, err := json.Marshal(observed.All())
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "private-canary")
		})
	}
}

func TestORMTelemetryPreservesLevelsWithoutEvaluatingSQL(t *testing.T) {
	output, observed := observedLogger(t)
	cfg := &config.Config{}
	cfg.Database.Logging.IgnoreNotFound = true
	cfg.Database.Logging.SlowThreshold = 20 * time.Millisecond
	base := configureGormLogger(cfg, output)
	query := func() (string, int64) { t.Error("SQL callback must not be evaluated"); return "private-canary", 1 }
	base.Trace(t.Context(), time.Now(), query, nil)
	require.Zero(t, observed.Len(), "Default warn level does not emit fast success")
	base.Trace(t.Context(), time.Now().Add(-time.Second), query, nil)
	require.Equal(t, zapcore.WarnLevel, observed.TakeAll()[0].Level)
	base.Trace(t.Context(), time.Now().Add(-time.Second), query, gorm.ErrRecordNotFound)
	require.Zero(t, observed.Len(), "IgnoreNotFound suppresses even slow not-found queries")
	base.LogMode(logger.Silent).Trace(t.Context(), time.Now(), query, errors.New("private-canary"))
	require.Zero(t, observed.Len())
	base.LogMode(logger.Info).Trace(t.Context(), time.Now(), query, nil)
	require.Equal(t, zapcore.InfoLevel, observed.TakeAll()[0].Level)
	base.Trace(t.Context(), time.Now(), query, nil)
	require.Zero(t, observed.Len(), "LogMode must not mutate the shared logger")
	base.Info(t.Context(), "private-canary", "private-canary")
	require.Zero(t, observed.Len())
	base.Warn(t.Context(), "private-canary", "private-canary")
	base.Error(t.Context(), "private-canary", "private-canary")
	base.LogMode(logger.Info).Info(t.Context(), "private-canary", "private-canary")
	for _, event := range observed.TakeAll() {
		require.Equal(t, "database diagnostic", event.Message)
		require.Empty(t, event.Context)
	}
	filteredSQL, parameters := base.(ormLogger).ParamsFilter(t.Context(), "SELECT $1", "private-canary")
	require.Equal(t, "SELECT $1", filteredSQL)
	require.Nil(t, parameters)
	for _, level := range []string{"silent", "error", "warn", "info", "unknown"} {
		cfg.Database.Logging.LogLevel = level
		configured := configureGormLogger(cfg, output).(ormLogger)
		require.Equal(t, map[string]logger.LogLevel{"silent": logger.Silent, "error": logger.Error, "warn": logger.Warn, "info": logger.Info, "unknown": logger.Warn}[level], configured.level)
	}
}

type privateDriverError string

func (e privateDriverError) Error() string    { return "private-canary-driver" }
func (e privateDriverError) SQLState() string { return string(e) }

func TestDatabaseErrorCategoriesAreBounded(t *testing.T) {
	for state, category := range map[string]string{
		"23505": "conflict", "23503": "reference_violation", "23514": "constraint_violation",
		"22001": "invalid_data", "22003": "invalid_data", "22021": "invalid_data", "22P02": "invalid_data",
		"40001": "serialization_failure", "40P01": "deadlock", "57014": "canceled", "private-canary-state": "database_error",
	} {
		require.Equal(t, category, databaseErrorCategory(fmt.Errorf("wrapped: %w", privateDriverError(state))))
	}
	for err, category := range map[error]string{
		context.Canceled: "canceled", context.DeadlineExceeded: "timeout", gorm.ErrRecordNotFound: "not_found",
		gorm.ErrDuplicatedKey: "conflict", gorm.ErrForeignKeyViolated: "reference_violation",
	} {
		require.Equal(t, category, databaseErrorCategory(err))
	}
}

func TestRealPostgresDriverFailureDoesNotEnterRuntimeLogs(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	output, observed := observedLogger(t)
	cfg := &config.Config{}
	cfg.Database.Logging.LogLevel = "info"
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: configureGormLogger(cfg, output)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	err = db.WithContext(t.Context()).Exec("SELECT CAST(? AS integer)", "private-canary-database-value").Error
	require.ErrorContains(t, err, "private-canary", "The driver must actually return a sensitive diagnostic for the probe")
	events := observed.All()
	require.NotEmpty(t, events)
	require.Equal(t, zapcore.ErrorLevel, events[len(events)-1].Level)
	require.Equal(t, "invalid_data", events[len(events)-1].ContextMap()["category"])
	encoded, err := json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-canary")
}
