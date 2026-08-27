package token_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/auth"
	tokenstore "github.com/goformx/goforms/internal/infrastructure/repository/token"
)

type integrationDB struct{ db *gorm.DB }

func (d *integrationDB) Close() error                          { return nil }
func (d *integrationDB) MonitorConnectionPool(context.Context) {}
func (d *integrationDB) Ping(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
func (d *integrationDB) GetDB() *gorm.DB { return d.db }

func TestStorePersistsOnlyTokenHashScopesAndRevocation(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GOFORMX_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	store := tokenstore.NewStore(&integrationDB{db: db})
	now := time.Now().UTC()
	token, plaintext, err := auth.Issue("11111111-1111-4111-8111-111111111111",
		[]auth.Scope{auth.ScopeFormsRead, auth.ScopeFormsWrite}, time.Hour, now)
	require.NoError(t, err)
	require.NoError(t, store.Save(t.Context(), token))

	loaded, err := store.FindByID(t.Context(), auth.LookupID(plaintext))
	require.NoError(t, err)
	require.NoError(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsWrite, now))
	require.NoError(t, store.Revoke(t.Context(), loaded.ID, now))
	loaded, err = store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.Error(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsRead, now))

	var plaintextCount int64
	require.NoError(t, db.Raw("SELECT count(*) FROM service_tokens WHERE encode(token_hash, 'escape') = ?", plaintext).
		Scan(&plaintextCount).Error)
	require.Zero(t, plaintextCount)
}
