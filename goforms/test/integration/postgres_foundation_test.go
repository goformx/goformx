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

	ownerID, formID, schemaID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	publicKey := "gfpk_" + uuid.NewString()
	_, err = pool.Exec(t.Context(), `
		INSERT INTO users (uuid, email, hashed_password, first_name, last_name)
		VALUES ($1, $2, 'not-used', 'Schema', 'Fixture')
	`, ownerID, ownerID+"@example.test")
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `
		INSERT INTO forms (uuid, organization_id, name, title, description, active, status, public_key,
			current_schema_version, cors_origins, cors_methods, cors_headers)
		VALUES ($1, $2, 'schema-fixture', 'Schema Fixture', '', true, 'draft', $3, 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)
	`, formID, ownerID, publicKey)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `
		INSERT INTO form_schemas (uuid, form_id, schema, version, state, created_at)
		VALUES ($1, $2, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{}}'::jsonb,
			1, 'draft', now())
	`, schemaID, formID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM forms WHERE organization_id = $1", ownerID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE uuid = $1", ownerID)
	})

	var storedKey string
	var currentVersion int
	err = pool.QueryRow(t.Context(), `SELECT public_key, current_schema_version FROM forms WHERE uuid = $1`, formID).
		Scan(&storedKey, &currentVersion)
	require.NoError(t, err)
	require.Equal(t, publicKey, storedKey)
	require.Equal(t, 1, currentVersion)

	rows, err := pool.Query(t.Context(), `
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND (table_name, column_name) IN (
			('form_schemas', 'schema'),
			('form_submissions', 'data'),
			('form_submissions', 'metadata')
		  )
		ORDER BY table_name, column_name
	`)
	require.NoError(t, err)
	type storageColumn struct{ table, column, dataType string }
	var storage []storageColumn
	for rows.Next() {
		var column storageColumn
		require.NoError(t, rows.Scan(&column.table, &column.column, &column.dataType))
		storage = append(storage, column)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Equal(t, []storageColumn{
		{"form_schemas", "schema", "jsonb"},
		{"form_submissions", "data", "json"},
		{"form_submissions", "metadata", "jsonb"},
	}, storage, "migrated storage types are an explicit persistence contract")

	_, err = pool.Exec(t.Context(), `UPDATE form_schemas SET state = 'published', published_at = now() WHERE uuid = $1`, schemaID)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE form_schemas SET schema = '{}'::jsonb WHERE uuid = $1`, schemaID)
	require.ErrorContains(t, err, "published schema versions are immutable")

	var mutableSchemaColumns int
	err = pool.QueryRow(t.Context(), `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'forms' AND column_name = 'schema'
	`).Scan(&mutableSchemaColumns)
	require.NoError(t, err)
	require.Zero(t, mutableSchemaColumns)

	idempotencyKey := "integration-" + uuid.NewString()
	_, err = pool.Exec(t.Context(), `
		INSERT INTO form_submissions (uuid, form_id, schema_version, request_id, data, submitted_at, status, idempotency_key)
		VALUES ($1, $2, 1, $3, '{"email":"ada@example.com"}'::json, now(), 'accepted', $4)
	`, uuid.NewString(), formID, "req_"+uuid.NewString(), idempotencyKey)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `
		INSERT INTO form_submissions (uuid, form_id, schema_version, request_id, data, submitted_at, status, idempotency_key)
		VALUES ($1, $2, 1, $3, '{"email":"ada@example.com"}'::json, now(), 'accepted', $4)
	`, uuid.NewString(), formID, "req_"+uuid.NewString(), idempotencyKey)
	require.Error(t, err)
}
