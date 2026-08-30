package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestPublishedClientCompletesManagementFlow(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by the canonical task verify command")
	}
	node, err := exec.LookPath("node")
	require.NoError(t, err, "task verify requires Node for the published client proof")
	example := filepath.Join("..", "..", ".contract-client", "examples", "management-flow.mjs")
	_, err = os.Stat(example)
	require.NoError(t, err, "run npm run contract:check before the integration suite")
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	database := &boundaryDB{db: db}
	organizationID := uuid.NewString()
	tokens := tokenrepository.NewStore(database)
	token, plaintext, err := auth.Issue(organizationID, []auth.Scope{
		auth.ScopeFormsRead, auth.ScopeFormsWrite, auth.ScopeFormsPublish, auth.ScopeSubmissionsRead,
	}, time.Hour, time.Now())
	require.NoError(t, err)
	require.NoError(t, tokens.Save(t.Context(), token))
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id = ?", organizationID).Error)
		require.NoError(t, db.Exec("DELETE FROM service_tokens WHERE organization_id = ?", organizationID).Error)
	})
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	router := echo.New()
	web.NewV1APIHandler(formrepository.NewStore(database, logger), tokens, nil).RegisterRoutes(router)
	var publicRequests, managementRequests, leakedCredentials atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/v1/public/") {
			publicRequests.Add(1)
			if request.Header.Get("Authorization") != "" {
				leakedCredentials.Add(1)
				http.Error(writer, "public request carried a credential", http.StatusBadRequest)
				return
			}
		} else {
			managementRequests.Add(1)
		}
		router.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, node, example)
	command.Env = append(os.Environ(), "GOFORMX_API_URL="+server.URL,
		"GOFORMX_SERVICE_TOKEN="+plaintext, "GOFORMX_ALLOW_EXAMPLE_WRITES=1")
	output, err := command.CombinedOutput()
	require.NoError(t, err, "published client failed (output withheld to avoid credential/payload disclosure)")
	require.False(t, strings.Contains(string(output), plaintext), "client output contains a credential")
	require.False(t, strings.Contains(string(output), "example@example.test"), "client output contains an email")
	require.False(t, strings.Contains(string(output), "Synthetic example"), "client output contains submission content")
	var result struct {
		FormID        string   `json:"formId"`
		SubmissionID  string   `json:"submissionId"`
		SchemaVersion int      `json:"schemaVersion"`
		ExportIDs     []string `json:"exportIds"`
	}
	require.NoError(t, json.Unmarshal(output, &result))
	require.NoError(t, uuid.Validate(result.FormID))
	require.NoError(t, uuid.Validate(result.SubmissionID))
	require.Equal(t, 2, result.SchemaVersion)
	require.EqualValues(t, 6, publicRequests.Load(), "schema, allowed/denied browser origins, invalid submission, accepted submission, retry")
	require.EqualValues(t, 13, managementRequests.Load(), "create, forms list, version, publish, three detail reads, two metadata patches, submission detail/list, JSON/CSV exports")
	require.Zero(t, leakedCredentials.Load())
	var acceptedCount int64
	require.NoError(t, db.Table("form_submissions").Where("form_id = ? AND schema_version = ?", result.FormID, 2).Count(&acceptedCount).Error)
	require.EqualValues(t, 1, acceptedCount, "invalid/replayed submissions must not create extra rows")
	var ownedCount int64
	require.NoError(t, db.Table("forms").Where("uuid = ? AND organization_id = ?", result.FormID, organizationID).Count(&ownedCount).Error)
	require.EqualValues(t, 1, ownedCount)
	require.Len(t, result.ExportIDs, 2)
	for _, id := range result.ExportIDs {
		require.NoError(t, uuid.Validate(id))
	}
	var auditedCount int64
	require.NoError(t, db.Table("submission_export_audit").Where("export_id IN ? AND organization_id = ? AND form_id = ? AND credential_class = ? AND credential_id = ? AND row_count = 1 AND byte_count > 0",
		result.ExportIDs, organizationID, result.FormID, "service_token", token.ID).Count(&auditedCount).Error)
	require.EqualValues(t, 2, auditedCount, "Both downloads must have persisted audit records, not only successful responses")
}
