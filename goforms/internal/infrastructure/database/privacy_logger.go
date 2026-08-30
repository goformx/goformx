package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/goformx/goforms/internal/infrastructure/config"
	"github.com/goformx/goforms/internal/infrastructure/logging"
)

const defaultSlowQueryThreshold = 200 * time.Millisecond

// ormLogger emits metadata only. SQL and driver messages may contain accepted
// payloads, tokens, or webhook secrets even when bind parameters are hidden.
type ormLogger struct {
	output         logging.Logger
	level          logger.LogLevel
	slowThreshold  time.Duration
	ignoreNotFound bool
}

var _ logger.Interface = ormLogger{}

func configureGormLogger(cfg *config.Config, output logging.Logger) logger.Interface {
	level := logger.Warn
	switch cfg.Database.Logging.LogLevel {
	case "silent":
		level = logger.Silent
	case "error":
		level = logger.Error
	case "info":
		level = logger.Info
	}
	threshold := cfg.Database.Logging.SlowThreshold
	if threshold <= 0 {
		threshold = defaultSlowQueryThreshold
	}
	return ormLogger{output: output, level: level, slowThreshold: threshold, ignoreNotFound: cfg.Database.Logging.IgnoreNotFound}
}

func (l ormLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.level = level
	return l
}

// Arbitrary ORM diagnostic strings/arguments are deliberately not forwarded.
func (l ormLogger) Info(_ context.Context, _ string, _ ...any) {
	if l.level >= logger.Info {
		l.output.Info("database diagnostic")
	}
}

func (l ormLogger) Warn(_ context.Context, _ string, _ ...any) {
	if l.level >= logger.Warn {
		l.output.Warn("database diagnostic")
	}
}

func (l ormLogger) Error(_ context.Context, _ string, _ ...any) {
	if l.level >= logger.Error {
		l.output.Error("database diagnostic")
	}
}

func (l ormLogger) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	if l.level == logger.Silent {
		return
	}
	if l.ignoreNotFound && errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	duration := max(time.Since(begin).Milliseconds(), 0)
	if err != nil {
		if l.level >= logger.Error {
			l.output.Error("database operation failed", "category", databaseErrorCategory(err), "duration_ms", duration)
		}
		return
	}
	if time.Since(begin) > l.slowThreshold && l.level >= logger.Warn {
		l.output.Warn("slow database operation", "duration_ms", duration, "threshold_ms", l.slowThreshold.Milliseconds())
	} else if l.level >= logger.Info {
		l.output.Info("database operation completed", "duration_ms", duration)
	}
}

// Prevent interpolation even if another GORM path asks for query parameters.
func (l ormLogger) ParamsFilter(_ context.Context, sql string, _ ...any) (string, []any) {
	return sql, nil
}

func databaseErrorCategory(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "not_found"
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return "conflict"
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return "reference_violation"
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "23505":
			return "conflict"
		case "23503":
			return "reference_violation"
		case "23514":
			return "constraint_violation"
		case "22001", "22003", "22021", "22P02", "22P05":
			return "invalid_data"
		case "40001":
			return "serialization_failure"
		case "40P01":
			return "deadlock"
		case "57014":
			return "canceled"
		}
	}
	return "database_error"
}
