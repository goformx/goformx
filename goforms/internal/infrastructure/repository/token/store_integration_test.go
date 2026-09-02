package token_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
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
	actor := auth.DatabaseAuditActor("integration-fixture", ownerID)
	require.Error(t, store.Save(t.Context(), token, auth.AuditActor{}), "actor is mandatory, not an optional fallback")
	require.Error(t, store.Save(t.Context(), token, auth.DatabaseAuditActor("fixture", uuid.NewString())))
	require.NoError(t, store.Save(t.Context(), token, actor))
	listed, hasMore, err := store.ListByOrganization(t.Context(), ownerID, auth.TokenListOptions{Limit: 25})
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, listed, 1)
	require.Equal(t, token.ID, listed[0].ID)
	require.Equal(t, token.Name, listed[0].Name)
	require.Empty(t, listed[0].Hash, "metadata listing must not load the stored secret hash")
	base := now.Add(-time.Hour)
	for index := range 105 {
		id := fmt.Sprintf("%016d", index)
		createdAt := base.Add(time.Duration(index) * time.Second)
		if index >= 56 && index <= 66 {
			createdAt = base.Add(66 * time.Second)
		}
		switch index {
		case 65:
			id = "aaaaaaaaaaaaaa-_"
		case 64:
			id = "______________--"
		}
		require.NoError(t, db.Exec(`INSERT INTO service_tokens
			(token_id, name, organization_id, token_hash, scopes, created_at, expires_at)
			VALUES (?, ?, ?, ?, '["forms:read"]'::jsonb, ?, ?)`,
			id, "bulk-"+id, ownerID, []byte("hash-"+id), createdAt, now.Add(time.Hour)).Error)
	}
	seen := map[string]struct{}{}
	options := auth.TokenListOptions{Limit: 40}
	for page := 0; ; page++ {
		pageTokens, more, err := store.ListByOrganization(t.Context(), ownerID, options)
		require.NoError(t, err)
		for _, item := range pageTokens {
			_, duplicate := seen[item.ID]
			require.False(t, duplicate, item.ID)
			seen[item.ID] = struct{}{}
		}
		if page == 0 {
			require.NoError(t, db.Exec(`INSERT INTO service_tokens
				(token_id, name, organization_id, token_hash, scopes, created_at, expires_at)
				VALUES ('zzzzzzzzzzzzzzzz', 'concurrent-newer', ?, ?, '["forms:read"]'::jsonb, ?, ?)`,
				ownerID, []byte("concurrent-newer-hash"), now.Add(time.Hour), now.Add(2*time.Hour)).Error)
		}
		if !more {
			break
		}
		last := pageTokens[len(pageTokens)-1]
		options.Before, options.BeforeID = last.CreatedAt, last.ID
	}
	require.Len(t, seen, 106)
	require.Contains(t, seen, "aaaaaaaaaaaaaa-_")
	require.Contains(t, seen, "______________--")
	require.NotContains(t, seen, "zzzzzzzzzzzzzzzz", "newer inserts must not enter an existing keyset walk")
	foreign, hasMore, err := store.ListByOrganization(t.Context(), uuid.NewString(), auth.TokenListOptions{Limit: 25})
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Empty(t, foreign)

	loaded, err := store.FindByID(t.Context(), auth.LookupID(plaintext))
	require.NoError(t, err)
	require.NoError(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsWrite, now))
	require.NoError(t, store.MarkUsed(t.Context(), loaded.ID, now))
	loaded, err = store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.LastUsedAt)
	require.WithinDuration(t, now, *loaded.LastUsedAt, time.Second)
	foreignID := uuid.NewString()
	require.ErrorIs(t, store.RevokeByOrganization(t.Context(), foreignID, loaded.ID, now,
		auth.DatabaseAuditActor("integration-fixture", foreignID)), common.ErrNotFound)
	stillActive, err := store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.NoError(t, stillActive.Authorize(plaintext, ownerID, auth.ScopeFormsRead, now))
	require.NoError(t, store.RevokeByOrganization(t.Context(), ownerID, loaded.ID, now, actor))
	require.NoError(t, store.RevokeByOrganization(t.Context(), ownerID, loaded.ID, now.Add(time.Minute), actor),
		"revocation must be idempotent for an owned token")
	loaded, err = store.FindByID(t.Context(), loaded.ID)
	require.NoError(t, err)
	require.Error(t, loaded.Authorize(plaintext, loaded.OwnerID, auth.ScopeFormsRead, now))

	var plaintextCount int64
	require.NoError(t, db.Raw("SELECT count(*) FROM service_tokens WHERE encode(token_hash, 'escape') = ?", plaintext).
		Scan(&plaintextCount).Error)
	require.Zero(t, plaintextCount)

	concurrent, _, err := auth.Issue(ownerID, []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	require.NoError(t, store.Save(t.Context(), concurrent, actor))
	results := make(chan error, 8)
	for range cap(results) {
		go func() {
			results <- store.RevokeByOrganization(t.Context(), ownerID, concurrent.ID, now,
				auth.DatabaseAuditActor("concurrent-fixture", ownerID))
		}()
	}
	for range cap(results) {
		require.NoError(t, <-results)
	}
	var revocations int64
	require.NoError(t, db.Table("management_audit").Where("target_id = ? AND event = ?", concurrent.ID, "service_token.revoked").Count(&revocations).Error)
	require.EqualValues(t, 1, revocations, "row locking must serialize concurrent idempotent revocation")

	// A request identity groups work; it is not an event key. One logical
	// operator request may append multiple independently identified events.
	multiEventActor := auth.DatabaseAuditActor("multi-event-fixture", ownerID)
	for range 2 {
		additional, _, issueErr := auth.Issue(ownerID, []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
		require.NoError(t, issueErr)
		require.NoError(t, store.Save(t.Context(), additional, multiEventActor))
	}
	var groupedEvents, distinctEventIDs int64
	require.NoError(t, db.Table("management_audit").Where("request_id = ?", multiEventActor.RequestID).Count(&groupedEvents).Error)
	require.NoError(t, db.Raw("SELECT count(DISTINCT audit_id) FROM management_audit WHERE request_id = ?", multiEventActor.RequestID).
		Scan(&distinctEventIDs).Error)
	require.EqualValues(t, 2, groupedEvents)
	require.Equal(t, groupedEvents, distinctEventIDs, "audit_id, not request_id, is unique event identity")
}
