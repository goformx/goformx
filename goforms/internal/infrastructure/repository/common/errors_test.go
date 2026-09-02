package common_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

func TestStoreErrorsExposeStableCategories(t *testing.T) {
	privateCause := errors.New("private rejected value")
	for name, test := range map[string]struct {
		err  error
		kind error
	}{
		"not found":     {common.NewNotFoundError("get", "form", "id"), common.ErrNotFound},
		"invalid input": {common.NewInvalidInputError("get", "form", "id", privateCause), common.ErrInvalidInput},
		"gorm conflict": {common.NewDatabaseError("create", "form", "id", gorm.ErrDuplicatedKey), common.ErrConflict},
		"postgres conflict": {common.NewDatabaseError("create", "form", "id",
			fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "23505"})), common.ErrConflict},
		"postgres invalid UUID": {common.NewDatabaseError("get", "form", "id",
			fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "22P02"})), common.ErrInvalidInput},
		"constraint failure": {common.NewDatabaseError("create", "form", "id",
			&pgconn.PgError{Code: "23514"}), common.ErrDatabaseError},
		"unknown": {common.NewDatabaseError("create", "form", "id",
			privateCause), common.ErrDatabaseError},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, test.err, test.kind)
		})
	}
}
