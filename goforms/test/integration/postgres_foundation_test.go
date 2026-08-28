package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestSchemaFirstPostgresFoundation(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by the canonical task verify command")
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(t.Context()))

	var publicKey string
	var currentVersion int
	err = pool.QueryRow(t.Context(), `
		SELECT public_key, current_schema_version FROM forms
		WHERE uuid = '22222222-2222-4222-8222-222222222222'
	`).Scan(&publicKey, &currentVersion)
	require.NoError(t, err)
	require.Regexp(t, `^gfpk_[A-Za-z0-9_-]{20,}$`, publicKey)
	require.Equal(t, 1, currentVersion)

	_, err = pool.Exec(t.Context(), `
		UPDATE form_schemas SET state = 'published', published_at = now()
		WHERE form_id = '22222222-2222-4222-8222-222222222222' AND version = 1
	`)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `
		UPDATE form_schemas SET schema = '{}'::jsonb
		WHERE form_id = '22222222-2222-4222-8222-222222222222' AND version = 1
	`)
	require.ErrorContains(t, err, "published schema versions are immutable")

	var mutableSchemaColumns int
	err = pool.QueryRow(context.Background(), `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'forms' AND column_name = 'schema'
	`).Scan(&mutableSchemaColumns)
	require.NoError(t, err)
	require.Zero(t, mutableSchemaColumns)

	firstSubmissionID := uuid.NewString()
	secondSubmissionID := uuid.NewString()
	idempotencyKey := "integration-" + uuid.NewString()
	_, err = pool.Exec(t.Context(), `
		INSERT INTO form_submissions (uuid, form_id, schema_version, data, submitted_at, status, idempotency_key)
		VALUES ($1, '22222222-2222-4222-8222-222222222222', 1,
		        '{"email":"ada@example.com"}'::jsonb, now(), 'pending', $2)
	`, firstSubmissionID, idempotencyKey)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `
		INSERT INTO form_submissions (uuid, form_id, schema_version, data, submitted_at, status, idempotency_key)
		VALUES ($1, '22222222-2222-4222-8222-222222222222', 1,
		        '{"email":"ada@example.com"}'::jsonb, now(), 'pending', $2)
	`, secondSubmissionID, idempotencyKey)
	require.Error(t, err)
}
