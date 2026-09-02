package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestManagementAuditConstraintUpgradeRetainsHistoryAndScopeInventory(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	connection, err := pgx.Connect(t.Context(), databaseURL)
	require.NoError(t, err)
	schema := "audit_schema_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	require.NoError(t, execAuditSchema(t, connection, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, dropErr := connection.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		require.NoError(t, dropErr)
		require.NoError(t, connection.Close(ctx))
	})
	require.NoError(t, execAuditSchema(t, connection, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()))

	for _, migration := range []string{
		"2026083003_management_audit.up.sql",
		"2026083004_webhook_management_audit.up.sql",
		"2026090102_management_audit_correlation.up.sql",
	} {
		require.NoError(t, applyAuditMigration(t, connection, migration))
	}

	scopes, err := json.Marshal(auth.AllScopes())
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `INSERT INTO management_audit
		(audit_id, organization_id, subject_id, credential_class, credential_id, request_id,
		 event, target_id, scopes, expires_at, occurred_at, correlation_id)
		VALUES ($1, $2, 'subject', 'service_token', 'credential', 'request',
		 'service_token.created', 'target', $3, now() + interval '1 hour', now(), 'correlation')`,
		uuid.New(), uuid.NewString(), string(scopes))
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), `INSERT INTO management_audit
		(audit_id, organization_id, subject_id, credential_class, credential_id, request_id,
		 event, target_id, scopes, occurred_at, form_id, enabled)
		VALUES ($1, $2, 'subject', 'first_party_assertion', 'credential', 'request',
		 'webhook.created', $3, '[]', now(), $4, true)`,
		uuid.New(), uuid.NewString(), uuid.NewString(), uuid.New())
	require.NoError(t, err)

	require.Contains(t, auditConstraintNames(t, connection), "management_audit_check")
	require.NoError(t, applyAuditMigration(t, connection, "2026090103_stabilize_management_audit_constraints.up.sql"))
	assertManagementAuditSchema(t, connection, 2)

	require.NoError(t, applyAuditMigration(t, connection, "2026090103_stabilize_management_audit_constraints.down.sql"))
	names := auditConstraintNames(t, connection)
	require.Contains(t, names, "management_audit_check")
	require.NotContains(t, names, "management_audit_relationship_check")
	require.Equal(t, int64(2), auditRowCount(t, connection))

	require.NoError(t, applyAuditMigration(t, connection, "2026090103_stabilize_management_audit_constraints.up.sql"))
	assertManagementAuditSchema(t, connection, 2)
}

func applyAuditMigration(t *testing.T, connection *pgx.Conn, name string) error {
	t.Helper()
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "postgresql", name))
	if err != nil {
		return err
	}
	return execAuditSchema(t, connection, string(migration))
}

func execAuditSchema(t *testing.T, connection *pgx.Conn, statement string) error {
	t.Helper()
	_, err := connection.Exec(t.Context(), statement)
	return err
}

func auditConstraintNames(t *testing.T, connection *pgx.Conn) []string {
	t.Helper()
	rows, err := connection.Query(t.Context(), `SELECT conname FROM pg_constraint
		WHERE conrelid = 'management_audit'::regclass AND contype = 'c' ORDER BY conname`)
	require.NoError(t, err)
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	return names
}

func auditRowCount(t *testing.T, connection *pgx.Conn) int64 {
	t.Helper()
	var count int64
	require.NoError(t, connection.QueryRow(t.Context(), "SELECT count(*) FROM management_audit").Scan(&count))
	return count
}

func assertManagementAuditSchema(t *testing.T, connection *pgx.Conn, rows int64) {
	t.Helper()
	names := auditConstraintNames(t, connection)
	require.Contains(t, names, "management_audit_relationship_check")
	require.Contains(t, names, "management_audit_payload_check")
	require.NotContains(t, names, "management_audit_check")
	require.NotContains(t, names, "management_audit_check1")
	require.Equal(t, rows, auditRowCount(t, connection))

	var definition string
	require.NoError(t, connection.QueryRow(t.Context(), `SELECT pg_get_constraintdef(oid)
		FROM pg_constraint WHERE conrelid = 'management_audit'::regclass
		AND conname = 'management_audit_payload_check'`).Scan(&definition))
	require.Contains(t, definition, "jsonb_array_length(scopes) >= 1")
	require.Contains(t, definition, fmt.Sprintf("jsonb_array_length(scopes) <= %d", auth.ScopeCount()),
		"a scope-registry change requires a compatible schema migration")
}
