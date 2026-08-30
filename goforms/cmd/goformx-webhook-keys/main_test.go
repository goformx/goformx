package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/webhook"
)

func TestCommandRejectsArgumentsAndConfigurationWithoutEchoingSecrets(t *testing.T) {
	var output bytes.Buffer
	for _, args := range [][]string{nil, {"canary-password"}, {"rotate", "--key=canary-password"}} {
		err := run(t.Context(), args, &output)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "canary")
	}
	t.Setenv("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID", "")
	require.ErrorContains(t, run(t.Context(), []string{"rotate"}, &output), "ID is required")
	t.Setenv("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID", "key")
	t.Setenv("WEBHOOK_ENCRYPTION_KEY", "")
	t.Setenv("WEBHOOK_ENCRYPTION_KEYRING", `{"key":"canary-key"}`)
	err := run(t.Context(), []string{"rotate"}, &output)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "canary")
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	t.Setenv("WEBHOOK_ENCRYPTION_KEYRING", `{"key":"`+key+`"}`)
	t.Setenv("DATABASE_URL", "")
	require.ErrorContains(t, run(t.Context(), []string{"rotate"}, &output), "DATABASE_URL is required")
	t.Setenv("DATABASE_URL", "postgres://canary-password:invalid%password@localhost/db")
	err = run(t.Context(), []string{"verify"}, &output)
	require.EqualError(t, err, "cannot connect to webhook database")
	require.Empty(t, output.String(), "failed operations never print configuration or a success summary")
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, errors.New("canary-output-error") }

func TestCommandRotatesAndVerifiesThroughEnvironmentAndPrintsOnlyCounts(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	database, err := url.Parse(databaseURL)
	require.NoError(t, err)
	schema := "webhook_cli_test_" + uuid.NewString()[:8]
	query := database.Query()
	query.Set("search_path", schema)
	database.RawQuery = query.Encode()
	connection, err := pgx.Connect(t.Context(), database.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = connection.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		_ = connection.Close(context.Background())
	})
	_, err = connection.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `
		CREATE TABLE webhook_endpoints (LIKE public.webhook_endpoints INCLUDING ALL);
		CREATE TABLE webhook_deliveries (LIKE public.webhook_deliveries INCLUDING ALL);`)
	require.NoError(t, err)
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	newKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	legacy, err := webhook.NewCipher(oldKey)
	require.NoError(t, err)
	formID := uuid.NewString()
	encrypted, err := legacy.Encrypt(webhook.SecretConfig{SigningSecret: "canary-signing-secret"}, formID)
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `INSERT INTO webhook_endpoints
		(uuid, form_id, destination_origin, encrypted_config) VALUES ($1, $1, 'https://example.test', $2)`, formID, encrypted)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", database.String())
	t.Setenv("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID", "new")
	t.Setenv("WEBHOOK_ENCRYPTION_KEYRING", `{"new":"`+newKey+`"}`)
	t.Setenv("WEBHOOK_ENCRYPTION_KEY", oldKey)
	var output bytes.Buffer
	require.NoError(t, run(t.Context(), []string{"rotate"}, &output))
	var summary map[string]int
	require.NoError(t, json.Unmarshal(output.Bytes(), &summary))
	require.Equal(t, map[string]int{"endpoints": 1, "deliveries": 0, "reencrypted": 1}, summary)
	output.Reset()
	t.Setenv("WEBHOOK_ENCRYPTION_KEY", "")
	require.NoError(t, run(t.Context(), []string{"verify"}, &output))
	require.JSONEq(t, `{"endpoints":1,"deliveries":0,"reencrypted":0}`, output.String())
	err = run(t.Context(), []string{"rotate"}, failedOutput{})
	require.EqualError(t, err, "operation committed but summary output failed; run verify before resuming service")
	_, err = connection.Exec(t.Context(), "UPDATE webhook_endpoints SET encrypted_config = $1", encrypted)
	require.NoError(t, err)
	output.Reset()
	err = run(t.Context(), []string{"verify"}, &output)
	require.ErrorContains(t, err, "authentication failed")
	require.Empty(t, output.String())
}
