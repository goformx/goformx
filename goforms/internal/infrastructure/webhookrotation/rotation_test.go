package webhookrotation

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/webhook"
)

func rotationFixture(t *testing.T) (*pgx.Conn, *webhook.Cipher, *webhook.Cipher, *webhook.Cipher) {
	t.Helper()
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	configuration, err := pgx.ParseConfig(databaseURL)
	require.NoError(t, err)
	schema := "webhook_rotation_test_" + uuid.NewString()[:8]
	configuration.RuntimeParams["search_path"] = schema
	connection, err := pgx.ConnectConfig(t.Context(), configuration)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, err := pgx.ConnectConfig(context.Background(), configuration)
		if err == nil {
			_, _ = cleanup.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
			_ = cleanup.Close(context.Background())
		}
		_ = connection.Close(context.Background())
	})
	// Isolated tables inherit the actual migrated layouts and CHECK constraints;
	// they cannot rotate or lock another parallel test's synthetic tenant data.
	_, err = connection.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `
		CREATE TABLE webhook_endpoints (LIKE public.webhook_endpoints INCLUDING ALL);
		CREATE TABLE webhook_deliveries (LIKE public.webhook_deliveries INCLUDING ALL);
	`)
	require.NoError(t, err)
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	newKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	legacy, err := webhook.NewCipher(oldKey)
	require.NoError(t, err)
	ring, err := webhook.NewKeyring("new", map[string]string{"old": oldKey, "new": newKey}, oldKey)
	require.NoError(t, err)
	newOnly, err := webhook.NewKeyring("new", map[string]string{"new": newKey}, "")
	require.NoError(t, err)
	for index := range batchSize + 1 {
		id := fmt.Sprintf("%036d", index+1)
		encoded, err := legacy.Encrypt(webhook.SecretConfig{SigningSecret: "canary-signing-secret",
			Headers: map[string]string{"Authorization": "canary-header"}, DestinationURL: "https://example.test/private"}, id)
		require.NoError(t, err)
		_, err = connection.Exec(t.Context(), `
			INSERT INTO webhook_endpoints (uuid, form_id, destination_origin, encrypted_config, enabled)
			VALUES ($1, $1, 'https://example.test', $2, $3);
		`, id, encoded, index%2 == 0)
		require.NoError(t, err)
		_, err = connection.Exec(t.Context(), `
			INSERT INTO webhook_deliveries (uuid, form_id, submission_id, endpoint_id, destination_origin, encrypted_config, status)
			VALUES ($1, $1, $1, $1, 'https://example.test', $2, $3);
		`, id, encoded, []string{"pending", "processing", "delivered", "dead_letter"}[index%4])
		require.NoError(t, err)
	}
	return connection, legacy, ring, newOnly
}

func snapshot(t *testing.T, connection *pgx.Conn) []byte {
	t.Helper()
	var buffer bytes.Buffer
	_, err := connection.PgConn().CopyTo(t.Context(), &buffer, `COPY (
		SELECT 'endpoint', uuid, form_id, encode(encrypted_config, 'hex'), enabled::text FROM webhook_endpoints
		UNION ALL SELECT 'delivery', uuid, form_id, encode(encrypted_config, 'hex'), status FROM webhook_deliveries
		ORDER BY 1, 2) TO STDOUT`)
	require.NoError(t, err)
	return buffer.Bytes()
}

func TestRotationAuthenticatesAllStatesAndIsIdempotent(t *testing.T) {
	connection, _, ring, newOnly := rotationFixture(t)
	_, err := Run(t.Context(), connection, ring, true)
	require.ErrorContains(t, err, "requires a previous or legacy key")
	before := snapshot(t, connection)
	result, err := Run(t.Context(), connection, ring, false)
	require.NoError(t, err)
	require.EqualValues(t, batchSize+1, result.Endpoints)
	require.EqualValues(t, batchSize+1, result.Deliveries)
	require.EqualValues(t, 2*(batchSize+1), result.Reencrypted)
	after := snapshot(t, connection)
	require.NotEqual(t, before, after)
	result, err = Run(t.Context(), connection, newOnly, true)
	require.NoError(t, err)
	require.Zero(t, result.Reencrypted)
	_, err = Run(t.Context(), connection, newOnly, false)
	require.NoError(t, err)
	require.Equal(t, after, snapshot(t, connection))
	rows, err := connection.Query(t.Context(), "SELECT form_id, encrypted_config FROM webhook_deliveries")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var formID string
		var encrypted []byte
		require.NoError(t, rows.Scan(&formID, &encrypted))
		decoded, err := newOnly.Decrypt(encrypted, formID)
		require.NoError(t, err)
		require.Equal(t, "canary-signing-secret", decoded.SigningSecret)
		require.Equal(t, "canary-header", decoded.Headers["Authorization"])
	}
	require.NoError(t, rows.Err())
}

func TestRotationWrongKeyRollsBackAlreadyProcessedEndpoints(t *testing.T) {
	connection, _, ring, _ := rotationFixture(t)
	_, err := connection.Exec(t.Context(), "UPDATE webhook_deliveries SET encrypted_config = $1 WHERE uuid = $2",
		[]byte("canary-invalid-secret"), fmt.Sprintf("%036d", batchSize+1))
	require.NoError(t, err)
	before := snapshot(t, connection)
	result, err := Run(t.Context(), connection, ring, false)
	require.ErrorContains(t, err, "authentication failed")
	require.NotContains(t, err.Error(), "canary")
	require.Equal(t, Result{}, result)
	require.Equal(t, before, snapshot(t, connection), "no endpoint or earlier batch may remain re-encrypted")
}

func TestRotationInterruptionRollsBackAndRestartSucceeds(t *testing.T) {
	connection, _, ring, newOnly := rotationFixture(t)
	before := snapshot(t, connection)
	_, err := connection.Exec(t.Context(), `
		CREATE FUNCTION interrupt_rotation() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_sleep(5); RETURN NEW; END $$;
		CREATE TRIGGER interrupt_rotation BEFORE UPDATE ON webhook_deliveries
		FOR EACH ROW EXECUTE FUNCTION interrupt_rotation();
	`)
	require.NoError(t, err)
	observer, err := pgx.ConnectConfig(t.Context(), connection.Config())
	require.NoError(t, err)
	t.Cleanup(func() { _ = observer.Close(context.Background()) })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	pid := connection.PgConn().PID()
	go func() {
		_, rotationErr := Run(ctx, connection, ring, false)
		finished <- rotationErr
	}()
	// Wait for the delivery trigger, which is reached only AFTER all endpoint
	// updates. Cancellation must test rollback of actual work, not early setup.
	require.Eventually(t, func() bool {
		var sleeping bool
		queryErr := observer.QueryRow(t.Context(),
			"SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event = 'PgSleep')", pid).Scan(&sleeping)
		return queryErr == nil && sleeping
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, before, snapshot(t, observer), "readers cannot see uncommitted encryption changes")
	_, err = observer.Exec(t.Context(), "SET statement_timeout = '100ms'")
	require.NoError(t, err)
	for _, query := range []string{
		"UPDATE webhook_endpoints SET enabled = false",
		"DELETE FROM webhook_deliveries",
		"INSERT INTO webhook_deliveries SELECT * FROM webhook_deliveries LIMIT 1",
	} {
		_, writeErr := observer.Exec(t.Context(), query)
		var postgresError *pgconn.PgError
		require.ErrorAs(t, writeErr, &postgresError)
		require.Equal(t, "57014", postgresError.Code, "competing writes must block, not enter the locked tables")
	}
	cancel()
	err = <-finished
	require.Error(t, err)
	// Cancellation can close pgx's socket. A restarted operator has a fresh
	// connection and must observe the pre-transaction state, not partial batches.
	restarted, err := pgx.ConnectConfig(t.Context(), connection.Config())
	require.NoError(t, err)
	t.Cleanup(func() { _ = restarted.Close(context.Background()) })
	connection = restarted
	require.Equal(t, before, snapshot(t, connection))
	_, err = connection.Exec(t.Context(), "DROP TRIGGER interrupt_rotation ON webhook_deliveries")
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, ring, false)
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, newOnly, true)
	require.NoError(t, err)
}

func TestRotationLogicalBackupRestoreAndReverseRotation(t *testing.T) {
	connection, legacy, ring, newOnly := rotationFixture(t)
	var endpoints, deliveries bytes.Buffer
	_, err := connection.PgConn().CopyTo(t.Context(), &endpoints, "COPY webhook_endpoints TO STDOUT WITH (FORMAT binary)")
	require.NoError(t, err)
	_, err = connection.PgConn().CopyTo(t.Context(), &deliveries, "COPY webhook_deliveries TO STDOUT WITH (FORMAT binary)")
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, ring, false)
	require.NoError(t, err)
	// Reverse rotation keeps the keyring-capable binary and selects the old key.
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	newKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	reverse, err := webhook.NewKeyring("old", map[string]string{"old": oldKey, "new": newKey}, "")
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, reverse, false)
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, reverse, true)
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, newOnly, true)
	require.Error(t, err)
	// A pre-rotation backup still needs its old vault key, not just the new one.
	_, err = connection.Exec(t.Context(), "TRUNCATE webhook_endpoints, webhook_deliveries")
	require.NoError(t, err)
	_, err = connection.PgConn().CopyFrom(t.Context(), &endpoints, "COPY webhook_endpoints FROM STDIN WITH (FORMAT binary)")
	require.NoError(t, err)
	_, err = connection.PgConn().CopyFrom(t.Context(), &deliveries, "COPY webhook_deliveries FROM STDIN WITH (FORMAT binary)")
	require.NoError(t, err)
	var encrypted []byte
	err = connection.QueryRow(t.Context(), "SELECT encrypted_config FROM webhook_endpoints ORDER BY uuid LIMIT 1").Scan(&encrypted)
	require.NoError(t, err)
	_, err = legacy.Decrypt(encrypted, fmt.Sprintf("%036d", 1))
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, newOnly, true)
	require.Error(t, err)
	_, err = Run(t.Context(), connection, ring, false)
	require.NoError(t, err)
	_, err = Run(t.Context(), connection, newOnly, true)
	require.NoError(t, err)
}
