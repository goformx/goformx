package assertionreplay_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/auth"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
)

type integrationDB struct{ db *gorm.DB }

func (d *integrationDB) Close() error                          { return nil }
func (d *integrationDB) MonitorConnectionPool(context.Context) {}
func (d *integrationDB) Ping(context.Context) error            { return nil }
func (d *integrationDB) GetDB() *gorm.DB                       { return d.db }

func TestStoreAtomicallyConsumesAndExpiresReplayIdentity(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GOFORMX_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	store := assertionreplay.NewStore(&integrationDB{db: db})
	now := time.Now().UTC()
	replay := auth.AssertionReplay{
		Issuer: "https://goformx.com", AssertionID: "33333333-3333-4333-8333-333333333333",
		ExpiresAt: now.Add(time.Minute), FirstSeenAt: now,
		SubjectID:      "11111111-1111-4111-8111-111111111111",
		OrganizationID: "22222222-2222-4222-8222-222222222222", KeyID: "gofx-fpa-test-a",
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM first_party_assertion_replays WHERE issuer = ? AND assertion_id = ?",
			replay.Issuer, replay.AssertionID).Error
	})
	require.NoError(t, store.Consume(t.Context(), replay))
	require.ErrorIs(t, store.Consume(t.Context(), replay), auth.ErrFirstPartyAssertionReplay)

	deleted, err := store.DeleteExpired(t.Context(), now, 100)
	require.NoError(t, err)
	require.Zero(t, deleted)
	deleted, err = store.DeleteExpired(t.Context(), replay.ExpiresAt.Add(time.Second), 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	require.NoError(t, store.Consume(t.Context(), replay))
}
