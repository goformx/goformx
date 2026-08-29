package token_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
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
	ownerID := uuid.NewString()
	require.NoError(t, db.Exec(`
		INSERT INTO users (uuid, email, hashed_password, first_name, last_name)
		VALUES (?, ?, 'not-used', 'Token', 'Fixture')
	`, ownerID, ownerID+"@example.test").Error)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM service_tokens WHERE organization_id = ?", ownerID).Error
		_ = db.Exec("DELETE FROM users WHERE uuid = ?", ownerID).Error
	})
	now := time.Now().UTC()
	token, plaintext, err := auth.Issue(ownerID,
		[]auth.Scope{auth.ScopeFormsRead, auth.ScopeFormsWrite}, time.Hour, now)
	require.NoError(t, err)
	require.NoError(t, store.Save(t.Context(), token))
	listed, err := store.ListByOrganization(t.Context(), ownerID, 25)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, token.ID, listed[0].ID)
	require.Equal(t, token.Name, listed[0].Name)
	require.Empty(t, listed[0].Hash, "metadata listing must not load the stored secret hash")
	foreign, err := store.ListByOrganization(t.Context(), uuid.NewString(), 25)
	require.NoError(t, err)
	require.Empty(t, foreign)

	loaded, err := store.FindByID(t.Context(), auth.LookupID(plaintext))
	require.NoError(t, err)
	require.NoError(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsWrite, now))
	require.NoError(t, store.MarkUsed(t.Context(), loaded.ID, now))
	loaded, err = store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.LastUsedAt)
	require.WithinDuration(t, now, *loaded.LastUsedAt, time.Second)
	require.Error(t, store.RevokeByOrganization(t.Context(), uuid.NewString(), loaded.ID, now))
	stillActive, err := store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.NoError(t, stillActive.Authorize(plaintext, ownerID, auth.ScopeFormsRead, now))
	require.NoError(t, store.RevokeByOrganization(t.Context(), ownerID, loaded.ID, now))
	require.NoError(t, store.RevokeByOrganization(t.Context(), ownerID, loaded.ID, now.Add(time.Minute)),
		"revocation must be idempotent for an owned token")
	loaded, err = store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.Error(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsRead, now))

	var plaintextCount int64
	require.NoError(t, db.Raw("SELECT count(*) FROM service_tokens WHERE encode(token_hash, 'escape') = ?", plaintext).
		Scan(&plaintextCount).Error)
	require.Zero(t, plaintextCount)
}
