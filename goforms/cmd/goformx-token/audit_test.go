package main

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/managementaudit"
)

func captureTokenCommand(t *testing.T, arguments ...string) ([]byte, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original; _ = reader.Close(); _ = writer.Close() }()
	runErr := run(t.Context(), arguments)
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return output, runErr
}

func TestOperatorTokenMutationsCommitWithRoleAttributedAudit(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	database, err := url.Parse(databaseURL)
	require.NoError(t, err)
	schema := "token_cli_audit_" + uuid.NewString()[:8]
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
		CREATE TABLE service_tokens (LIKE public.service_tokens INCLUDING ALL);
		CREATE TABLE management_audit (LIKE public.management_audit INCLUDING ALL);`)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", database.String())
	organizationID := uuid.NewString()
	issueArgs := []string{"issue", "--owner", organizationID, "--name", "private-cli-nickname", "--scopes", "forms:read"}
	output, err := captureTokenCommand(t, issueArgs...)
	require.NoError(t, err)
	var issued struct {
		ID    string `json:"tokenId"`
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(output, &issued))
	require.Equal(t, issued.ID, auth.LookupID(issued.Token))
	var role, subject, credential, kind string
	err = connection.QueryRow(t.Context(), `SELECT current_user, subject_id, credential_class, event
		FROM management_audit WHERE target_id = $1`, issued.ID).Scan(&role, &subject, &credential, &kind)
	require.NoError(t, err)
	require.Equal(t, auth.DatabaseAuditActor(role, organizationID).SubjectID, subject)
	require.Equal(t, string(auth.CredentialClassDatabaseOperator), credential)
	require.Equal(t, string(managementaudit.TokenCreated), kind)
	_, err = connection.Exec(t.Context(), `CREATE FUNCTION reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'cli-audit-secret-canary'; END $$;
		CREATE TRIGGER reject_audit BEFORE INSERT ON management_audit FOR EACH ROW EXECUTE FUNCTION reject_audit();`)
	require.NoError(t, err)
	for _, arguments := range [][]string{issueArgs, {"rotate", "--token-id", issued.ID}, {"revoke", "--token-id", issued.ID}} {
		output, err := captureTokenCommand(t, arguments...)
		require.ErrorIs(t, err, managementaudit.ErrUnavailable)
		require.Empty(t, output, "failed mutations must not reveal a token or success response")
		require.NotContains(t, err.Error(), "canary")
		var active, audits int
		require.NoError(t, connection.QueryRow(t.Context(), "SELECT count(*) FROM service_tokens WHERE revoked_at IS NULL").Scan(&active))
		require.NoError(t, connection.QueryRow(t.Context(), "SELECT count(*) FROM management_audit").Scan(&audits))
		require.Equal(t, 1, active)
		require.Equal(t, 1, audits)
		var total int
		require.NoError(t, connection.QueryRow(t.Context(), "SELECT count(*) FROM service_tokens").Scan(&total))
		require.Equal(t, 1, total, "failed rotation cannot leave a replacement token")
	}
	_, err = connection.Exec(t.Context(), "DROP TRIGGER reject_audit ON management_audit")
	require.NoError(t, err)
	output, err = captureTokenCommand(t, "rotate", "--token-id", issued.ID)
	require.NoError(t, err)
	var replacement struct {
		ID    string `json:"tokenId"`
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(output, &replacement))
	var related string
	err = connection.QueryRow(t.Context(), "SELECT related_id FROM management_audit WHERE target_id = $1 AND event = 'service_token.rotated'",
		issued.ID).Scan(&related)
	require.NoError(t, err)
	require.Equal(t, replacement.ID, related)
	_, err = captureTokenCommand(t, "revoke", "--token-id", replacement.ID)
	require.NoError(t, err)
	var records string
	require.NoError(t, connection.QueryRow(t.Context(), "SELECT jsonb_agg(to_jsonb(a))::text FROM management_audit a").Scan(&records))
	for _, forbidden := range []string{issued.Token, replacement.Token, "private-cli-nickname", "token_hash"} {
		require.NotContains(t, records, forbidden)
	}
	var audits int
	require.NoError(t, connection.QueryRow(t.Context(), "SELECT count(*) FROM management_audit").Scan(&audits))
	require.Equal(t, 3, audits)
}

func TestOperatorDatabaseFailuresDoNotEchoDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://private-user:canary-invalid%password@localhost/db")
	for _, arguments := range [][]string{
		{"issue", "--owner", uuid.NewString(), "--scopes", "forms:read"},
		{"rotate", "--token-id", "target"}, {"revoke", "--token-id", "target"},
	} {
		output, err := captureTokenCommand(t, arguments...)
		require.EqualError(t, err, "connect to PostgreSQL failed")
		require.Empty(t, output)
	}
}
