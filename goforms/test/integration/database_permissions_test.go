package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	"github.com/goformx/goforms/internal/infrastructure/webhookrotation"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

// Permission tests never SET ROLE on an administrator's runtime connection.
// Every application/operator connection authenticates as its own LOGIN role.
type permissionDatabase struct {
	owner *pgx.Conn
	roles map[string]string
	dsns  map[string]string
}

func newPermissionDatabase(t *testing.T) *permissionDatabase {
	t.Helper()
	dsn := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	admin, err := pgx.Connect(t.Context(), dsn)
	require.NoError(t, err)
	prefix := "gf_permissions_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	fixture := &permissionDatabase{roles: map[string]string{}, dsns: map[string]string{}}
	createdRoles := []string{}
	databaseCreated := false
	// Drop only this test's generated database/roles, never the supplied database.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if databaseCreated {
			_, dropErr := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{prefix}.Sanitize()+" WITH (FORCE)")
			require.NoError(t, dropErr)
		}
		for _, role := range createdRoles {
			_, dropErr := admin.Exec(ctx, "DROP ROLE "+pgx.Identifier{role}.Sanitize())
			require.NoError(t, dropErr)
		}
		require.NoError(t, admin.Close(ctx))
	})
	for _, kind := range []string{"owner", "migrator", "runtime", "token_operator", "key_operator", "backup"} {
		role := prefix + "_" + kind
		password := uuid.NewString() // disposable test credential, never a production default
		login := "LOGIN"
		if kind == "owner" {
			login = "NOLOGIN"
		}
		_, err = admin.Exec(t.Context(), "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" "+login+
			" NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD '"+password+"'")
		require.NoError(t, err)
		createdRoles = append(createdRoles, role)
		fixture.roles[kind] = role
		parsed, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		parsed.User, parsed.Path = url.UserPassword(role, password), "/"+prefix
		fixture.dsns[kind] = parsed.String()
	}
	_, err = admin.Exec(t.Context(), "CREATE DATABASE "+pgx.Identifier{prefix}.Sanitize()+
		" OWNER "+pgx.Identifier{fixture.roles["owner"]}.Sanitize()+" TEMPLATE template0")
	require.NoError(t, err)
	databaseCreated = true
	_, err = admin.Exec(t.Context(), "GRANT "+pgx.Identifier{fixture.roles["owner"]}.Sanitize()+
		" TO "+pgx.Identifier{fixture.roles["migrator"]}.Sanitize()+" WITH INHERIT FALSE, SET TRUE")
	require.NoError(t, err)
	fixture.owner = fixture.connect(t, "migrator")
	// NOINHERIT makes omission of SET ROLE visible, rather than creating objects
	// under the login and silently missing the owner's default ACLs.
	permissionDenied(t, fixture.owner, "CREATE TABLE public.wrong_owner (id integer)")
	_, err = fixture.owner.Exec(t.Context(), "SET ROLE "+pgx.Identifier{fixture.roles["owner"]}.Sanitize())
	require.NoError(t, err)
	files, err := filepath.Glob("../../migrations/postgresql/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		migration, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		_, err = fixture.owner.Exec(t.Context(), string(migration))
		require.NoError(t, err, filepath.Base(file))
	}
	// The migration tool owns this metadata table; runtime must not read/write it.
	_, err = fixture.owner.Exec(t.Context(), "CREATE TABLE schema_migrations (version bigint PRIMARY KEY, dirty boolean NOT NULL)")
	require.NoError(t, err)
	profile, err := os.ReadFile("../../deploy/postgresql/permissions.sql")
	require.NoError(t, err)
	replacements := []string{`:"database"`, pgx.Identifier{prefix}.Sanitize()}
	for kind, role := range fixture.roles {
		replacements = append(replacements, `:"`+kind+`"`, pgx.Identifier{role}.Sanitize())
	}
	// Same psql identifier template as the reviewed deployment contract; no SQL
	// values or identifiers originate from a network request.
	profileSQL := strings.NewReplacer(replacements...).Replace(string(profile))
	for range 2 { // reapplication with unchanged grants is harmless
		_, err = fixture.owner.Exec(t.Context(), profileSQL)
		require.NoError(t, err)
	}
	return fixture
}

func (f *permissionDatabase) connect(t *testing.T, kind string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), f.dsns[kind])
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, conn.Close(ctx))
	})
	return conn
}

func (f *permissionDatabase) gorm(t *testing.T, kind string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(f.dsns[kind]), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func permissionDenied(t *testing.T, conn *pgx.Conn, statement string) {
	t.Helper()
	_, err := conn.Exec(t.Context(), statement)
	var postgresError *pgconn.PgError
	require.ErrorAs(t, err, &postgresError, statement)
	require.Equal(t, "42501", postgresError.Code, statement)
}

func TestDatabasePermissionContract(t *testing.T) {
	f := newPermissionDatabase(t)
	runtime := f.connect(t, "runtime")
	t.Run("roles and migrated object inventory", func(t *testing.T) {
		for kind, role := range f.roles {
			var privileged, ownsObjects bool
			err := runtime.QueryRow(t.Context(), `SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit FROM pg_roles WHERE rolname = $1`, role).Scan(&privileged)
			require.NoError(t, err)
			require.False(t, privileged, kind)
			if kind == "owner" || kind == "migrator" {
				continue
			}
			err = runtime.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM pg_class WHERE relowner = $1::regrole) OR EXISTS(SELECT 1 FROM pg_proc WHERE proowner = $1::regrole) OR EXISTS(SELECT 1 FROM pg_namespace WHERE nspowner = $1::regrole) OR EXISTS(SELECT 1 FROM pg_database WHERE datdba = $1::regrole)`, role).Scan(&ownsObjects)
			require.NoError(t, err)
			require.False(t, ownsObjects, kind)
			var memberships int
			require.NoError(t, runtime.QueryRow(t.Context(), "SELECT count(*) FROM pg_auth_members WHERE member = $1::regrole", role).Scan(&memberships))
			require.Zero(t, memberships, kind)
			var grantable int
			require.NoError(t, runtime.QueryRow(t.Context(), `SELECT count(*) FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relkind = 'r' AND
				(has_table_privilege($1, oid, 'SELECT WITH GRANT OPTION, INSERT WITH GRANT OPTION, UPDATE WITH GRANT OPTION, DELETE WITH GRANT OPTION, TRUNCATE WITH GRANT OPTION, REFERENCES WITH GRANT OPTION, TRIGGER WITH GRANT OPTION, MAINTAIN WITH GRANT OPTION') OR
				 has_any_column_privilege($1, oid, 'SELECT WITH GRANT OPTION, INSERT WITH GRANT OPTION, UPDATE WITH GRANT OPTION, REFERENCES WITH GRANT OPTION'))`, role).Scan(&grantable))
			require.Zero(t, grantable, kind)
		}
		rows, err := f.owner.Query(t.Context(), "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename")
		require.NoError(t, err)
		tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		require.Equal(t, []string{"first_party_assertion_replays", "form_schemas", "form_submissions", "forms", "management_audit", "schema_migrations", "service_tokens", "submission_export_audit", "users", "webhook_deliveries", "webhook_endpoints"}, tables, "new tables require an intentional permission inventory update")
		var sequences, definerFunctions int
		require.NoError(t, f.owner.QueryRow(t.Context(), "SELECT count(*) FROM pg_sequences WHERE schemaname = 'public'").Scan(&sequences))
		require.Zero(t, sequences, "introducing sequences requires explicit permission review")
		require.NoError(t, f.owner.QueryRow(t.Context(), "SELECT count(*) FROM pg_proc WHERE pronamespace = 'public'::regnamespace AND prosecdef").Scan(&definerFunctions))
		require.Zero(t, definerFunctions)
	})
	t.Run("runtime forbidden authority", func(t *testing.T) {
		for _, statement := range []string{
			"UPDATE management_audit SET request_id = 'changed'", "DELETE FROM management_audit", "TRUNCATE management_audit",
			"UPDATE submission_export_audit SET request_id = 'changed'", "DELETE FROM submission_export_audit", "TRUNCATE submission_export_audit",
			"ALTER TABLE management_audit DISABLE TRIGGER ALL", "DROP TABLE management_audit",
			"CREATE TABLE public.forbidden (id integer)", "CREATE SCHEMA forbidden", "CREATE TEMP TABLE forbidden (id integer)",
			"CREATE FUNCTION public.forbidden() RETURNS integer LANGUAGE sql AS 'SELECT 1'",
			"ALTER ROLE " + pgx.Identifier{f.roles["runtime"]}.Sanitize() + " SUPERUSER",
			"SET session_replication_role = replica", "SET ROLE " + pgx.Identifier{f.roles["owner"]}.Sanitize(),
			"SET ROLE " + pgx.Identifier{f.roles["migrator"]}.Sanitize(), "SET ROLE " + pgx.Identifier{f.roles["token_operator"]}.Sanitize(),
			"SET ROLE " + pgx.Identifier{f.roles["key_operator"]}.Sanitize(),
			"SELECT * FROM users", "SELECT * FROM schema_migrations", "DELETE FROM service_tokens",
			"UPDATE service_tokens SET token_hash = token_hash", "UPDATE service_tokens SET scopes = scopes",
			"UPDATE form_submissions SET data = data", "DELETE FROM form_submissions", "DELETE FROM forms",
			"UPDATE form_schemas SET schema = schema", "DELETE FROM form_schemas",
			"UPDATE webhook_deliveries SET encrypted_config = encrypted_config", "DELETE FROM webhook_deliveries",
		} {
			permissionDenied(t, runtime, statement)
		}
	})
	t.Run("future owner objects are private by default", func(t *testing.T) {
		migration := f.connect(t, "migrator")
		permissionDenied(t, migration, "CREATE TABLE public.wrong_owner (id integer)")
		_, err := migration.Exec(t.Context(), "SET ROLE "+pgx.Identifier{f.roles["owner"]}.Sanitize())
		require.NoError(t, err)
		_, err = migration.Exec(t.Context(), `CREATE TABLE future_private (id integer);
			CREATE SEQUENCE future_sequence;
			CREATE FUNCTION future_function() RETURNS integer LANGUAGE sql SECURITY DEFINER AS 'SELECT 1';`)
		require.NoError(t, err)
		for _, kind := range []string{"runtime", "token_operator", "key_operator", "backup"} {
			conn := f.connect(t, kind)
			for _, statement := range []string{"SELECT * FROM future_private", "INSERT INTO future_private VALUES (1)", "SELECT nextval('future_sequence')", "SELECT future_function()"} {
				permissionDenied(t, conn, statement)
			}
		}
	})
	t.Run("real runtime and maintenance operations", func(t *testing.T) { exercisePermissionOperations(t, f) })
}

func exercisePermissionOperations(t *testing.T, f *permissionDatabase) {
	t.Helper()
	db := f.gorm(t, "runtime")
	database := &boundaryDB{db: db}
	org := uuid.NewString()
	actor := auth.DatabaseAuditActor("permission-fixture", org)
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	cipher, err := domainwebhook.NewKeyring("old", map[string]string{"old": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}, "")
	require.NoError(t, err)
	forms := formrepository.NewStoreWithOptions(database, logger, formrepository.StoreOptions{WebhookCipher: cipher})
	form := model.NewForm(org, "Permission fixture", "", model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object"})
	form.Name = "permissions"
	require.NoError(t, forms.CreateForm(t.Context(), form))
	_, err = forms.PublishSchemaVersion(t.Context(), org, form.ID, 1)
	require.NoError(t, err)
	_, _, err = forms.GetPublishedSchemaVersion(t.Context(), form.PublicKey, 1)
	require.NoError(t, err)
	_, err = forms.CreateSchemaVersion(t.Context(), org, form.ID, model.JSON{"type": "object", "title": "next"})
	require.NoError(t, err)
	current, err := forms.GetFormByID(t.Context(), org, form.ID)
	require.NoError(t, err)
	previous := current.UpdatedAt
	current.Title = "Changed title"
	require.NoError(t, forms.UpdateForm(t.Context(), current, previous))

	tokens := tokenrepository.NewStore(database)
	token, secret, err := auth.Issue(org, []auth.Scope{auth.ScopeFormsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	require.NoError(t, tokens.Save(t.Context(), token, actor))
	loaded, err := tokens.FindByID(t.Context(), token.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Authenticate(secret, org, time.Now()))
	require.NoError(t, tokens.MarkUsed(t.Context(), token.ID, time.Now()))
	listed, hasMore, err := tokens.ListByOrganization(t.Context(), org, auth.TokenListOptions{Limit: 100})
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, listed, 1)
	require.NoError(t, tokens.RevokeByOrganization(t.Context(), org, token.ID, time.Now(), actor))
	loaded, err = tokens.FindByID(t.Context(), token.ID)
	require.NoError(t, err)
	require.Error(t, loaded.Authenticate(secret, org, time.Now()))

	replays := assertionreplay.NewStore(database)
	replay := auth.AssertionReplay{Issuer: "https://goformx.com", AssertionID: uuid.NewString(),
		SubjectID: uuid.NewString(), OrganizationID: org, KeyID: "permissions",
		FirstSeenAt: time.Now().Add(-2 * time.Minute), ExpiresAt: time.Now().Add(-time.Minute)}
	require.NoError(t, replays.Consume(t.Context(), replay))
	require.ErrorIs(t, replays.Consume(t.Context(), replay), auth.ErrFirstPartyAssertionReplay)
	deleted, err := replays.DeleteExpired(t.Context(), time.Now(), 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	config := domainwebhook.SecretConfig{SigningSecret: strings.Repeat("s", 32)}
	_, err = forms.PutWebhookEndpoint(t.Context(), org, form.ID, "https://example.test/receiver", config, true, actor)
	require.NoError(t, err)
	_, err = forms.PutWebhookEndpoint(t.Context(), org, form.ID, "https://example.test/replaced", config, true, actor)
	require.NoError(t, err)
	for _, enabled := range []bool{false, true} {
		_, err = forms.PatchWebhookEndpoint(t.Context(), org, form.ID, domainwebhook.EndpointChange{Enabled: &enabled}, actor)
		require.NoError(t, err)
	}
	stored, repeated, err := forms.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{
		FormID: form.ID, SchemaVersion: 1, IdempotencyKey: uuid.NewString(), RequestID: uuid.NewString(),
		Data: model.JSON{"message": "fixture"}, SubmittedAt: time.Now(), Status: model.SubmissionStatusAccepted})
	require.NoError(t, err)
	require.False(t, repeated)
	_, repeated, err = forms.CreateSubmissionIdempotent(t.Context(), stored)
	require.NoError(t, err)
	require.True(t, repeated)
	delivery, event, err := forms.ClaimDelivery(t.Context(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, delivery)
	require.Equal(t, stored.ID, event.SubmissionID)
	require.NoError(t, forms.MarkDeliveryFailed(t.Context(), delivery.ID, "network", nil, false, 1, time.Second, time.Minute, time.Now()))
	require.NoError(t, forms.ReplayWebhookDelivery(t.Context(), org, form.ID, delivery.ID, actor))
	_, _, err = forms.ClaimDelivery(t.Context(), time.Minute)
	require.NoError(t, err)
	require.NoError(t, forms.MarkDeliveryDelivered(t.Context(), delivery.ID, 200, time.Now()))
	history, err := forms.ListWebhookDeliveries(t.Context(), org, form.ID, 100)
	require.NoError(t, err)
	require.Len(t, history, 1)
	exported, err := forms.ReadSubmissionExport(t.Context(), org, form.ID, submission.ExportFilters{})
	require.NoError(t, err)
	require.Len(t, exported, 1)
	require.NoError(t, forms.SaveSubmissionExportAudit(t.Context(), submission.ExportAudit{ID: uuid.NewString(),
		OrganizationID: org, FormID: form.ID, SubjectID: token.ID, CredentialID: token.ID,
		CredentialClass: "service_token", RequestID: uuid.NewString(), Format: submission.ExportJSON,
		RowCount: 1, ByteCount: 10, PreparedAt: time.Now()}))
	var audits int64
	require.NoError(t, db.Table("management_audit").Count(&audits).Error)
	require.GreaterOrEqual(t, audits, int64(7))
	require.NoError(t, db.Table("submission_export_audit").Count(&audits).Error)
	require.EqualValues(t, 1, audits)

	// The actual rotation implementation must obtain its strong lock with only
	// MAINTAIN + ciphertext-column UPDATE, not ownership or broad table UPDATE.
	keyOperator := f.connect(t, "key_operator")
	next, err := domainwebhook.NewKeyring("next", map[string]string{
		"old":  base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
		"next": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)),
	}, "")
	require.NoError(t, err)
	rotated, err := webhookrotation.Run(t.Context(), keyOperator, next, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, rotated.Reencrypted)
	_, err = webhookrotation.Run(t.Context(), keyOperator, next, true)
	require.NoError(t, err)
	permissionDenied(t, keyOperator, "UPDATE webhook_deliveries SET status = 'pending'")
	permissionDenied(t, keyOperator, "SELECT * FROM service_tokens")
	permissionDenied(t, keyOperator, "DELETE FROM webhook_endpoints")
	permissionDenied(t, keyOperator, "ALTER TABLE webhook_deliveries DISABLE TRIGGER ALL")

	exercisePermissionTokenCLI(t, f, org)
	operator := f.connect(t, "token_operator")
	for _, statement := range []string{"SELECT * FROM form_submissions", "UPDATE service_tokens SET scopes = scopes", "DELETE FROM service_tokens", "UPDATE management_audit SET request_id = 'changed'"} {
		permissionDenied(t, operator, statement)
	}
	backup := f.connect(t, "backup")
	for _, table := range []string{"users", "forms", "form_schemas", "form_submissions", "service_tokens", "first_party_assertion_replays", "webhook_endpoints", "webhook_deliveries", "management_audit", "submission_export_audit", "schema_migrations"} {
		// pg_dump needs SELECT and ACCESS SHARE; it must not inherit restore DDL.
		tx, beginErr := backup.Begin(t.Context())
		require.NoError(t, beginErr)
		_, err = tx.Exec(t.Context(), "LOCK TABLE "+pgx.Identifier{table}.Sanitize()+" IN ACCESS SHARE MODE")
		require.NoError(t, err)
		_, err = tx.Exec(t.Context(), "SELECT * FROM "+pgx.Identifier{table}.Sanitize())
		require.NoError(t, err)
		require.NoError(t, tx.Rollback(t.Context()))
	}
	permissionDenied(t, backup, "DELETE FROM forms")
	permissionDenied(t, backup, "CREATE TABLE public.restore_not_allowed (id integer)")
	permissionDenied(t, backup, "SET ROLE "+pgx.Identifier{f.roles["owner"]}.Sanitize())
	require.NoError(t, forms.DeleteWebhookEndpoint(t.Context(), org, form.ID, actor))
}

func exercisePermissionTokenCLI(t *testing.T, f *permissionDatabase, org string) {
	t.Helper()
	run := func(arguments ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/goformx-token"}, arguments...)...)
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(), "DATABASE_URL="+f.dsns["token_operator"])
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		require.NoError(t, err, "token operator failed: %s", stderr.String())
		return output // never print the one-time plaintext token
	}
	var issued, rotated struct {
		TokenID string `json:"tokenId"`
	}
	require.NoError(t, json.Unmarshal(run("issue", "--owner", org, "--scopes", "forms:read"), &issued))
	require.NotEmpty(t, issued.TokenID)
	require.NoError(t, json.Unmarshal(run("rotate", "--token-id", issued.TokenID), &rotated))
	require.NotEmpty(t, rotated.TokenID)
	run("revoke", "--token-id", rotated.TokenID)
	operator := f.connect(t, "token_operator")
	var count int
	actor := auth.DatabaseAuditActor(f.roles["token_operator"], org)
	require.NoError(t, operator.QueryRow(t.Context(), "SELECT count(*) FROM management_audit WHERE subject_id = $1 AND credential_class = $2 AND credential_id = $3", actor.SubjectID, actor.CredentialClass, actor.CredentialID).Scan(&count))
	require.Equal(t, 3, count, "CLI issue, rotation and revocation retain the authenticated database role")
}
