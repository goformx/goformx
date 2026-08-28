package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestParseScopesRejectsUnknownAndDeduplicates(t *testing.T) {
	scopes, err := parseScopes("forms:write, forms:publish,forms:write")
	require.NoError(t, err)
	require.Equal(t, []auth.Scope{auth.ScopeFormsWrite, auth.ScopeFormsPublish}, scopes)
	_, err = parseScopes("admin")
	require.ErrorContains(t, err, "unsupported scope")
}

func TestRotateAtomicallyRevokesAndLinksReplacement(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by the canonical task verify command")
	}
	connection, err := pgx.Connect(t.Context(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close(t.Context()) })
	now := time.Now().UTC()
	original, _, err := auth.Issue("11111111-1111-4111-8111-111111111111",
		[]auth.Scope{auth.ScopeFormsRead, auth.ScopeFormsWrite}, time.Hour, now)
	require.NoError(t, err)
	scopes, err := json.Marshal([]string{string(auth.ScopeFormsRead), string(auth.ScopeFormsWrite)})
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `
		INSERT INTO service_tokens (token_id, owner_id, token_hash, scopes, created_at, expires_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
	`, original.ID, original.OwnerID, original.Hash[:], string(scopes), original.CreatedAt, original.ExpiresAt)
	require.NoError(t, err)

	require.NoError(t, rotate(t.Context(), []string{
		"--database-url", databaseURL, "--token-id", original.ID, "--ttl", "2h",
	}))
	var replacedBy string
	var reason string
	var revokedAt time.Time
	err = connection.QueryRow(t.Context(), `
		SELECT replaced_by_token_id, revocation_reason, revoked_at
		FROM service_tokens WHERE token_id = $1
	`, original.ID).Scan(&replacedBy, &reason, &revokedAt)
	require.NoError(t, err)
	require.NotEmpty(t, replacedBy)
	require.Equal(t, "rotated", reason)
	require.WithinDuration(t, now, revokedAt, 5*time.Second)

	var replacementOwner string
	var replacementScopes []byte
	err = connection.QueryRow(t.Context(), `
		SELECT owner_id, scopes FROM service_tokens WHERE token_id = $1
	`, replacedBy).Scan(&replacementOwner, &replacementScopes)
	require.NoError(t, err)
	require.Equal(t, original.OwnerID, replacementOwner)
	require.JSONEq(t, string(scopes), string(replacementScopes))
}
