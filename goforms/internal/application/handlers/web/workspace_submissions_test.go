package web

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

func TestWorkspaceInboxProjectsEachAcceptedVersionAndCachesSchema(t *testing.T) {
	repo := mockform.NewMockRepository(gomock.NewController(t))
	owner, formID, siteID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	when := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	rows := []submission.WorkspaceRow{
		{Submission: &model.FormSubmission{ID: uuid.NewString(), FormID: formID, SchemaVersion: 1, Status: model.SubmissionStatusAccepted, Data: model.JSON{"secret": "do-not-show", "name": "Ada"}, SubmittedAt: when}, SiteID: &siteID, FormName: "contact", FormTitle: "Contact"},
		{Submission: &model.FormSubmission{ID: uuid.NewString(), FormID: formID, SchemaVersion: 1, Status: model.SubmissionStatusAccepted, Data: model.JSON{"secret": "also-hidden", "name": "Grace"}, SubmittedAt: when.Add(-time.Second)}, SiteID: &siteID, FormName: "contact", FormTitle: "Contact"},
		{Submission: &model.FormSubmission{ID: uuid.NewString(), FormID: formID, SchemaVersion: 2, Status: model.SubmissionStatusAccepted, Data: model.JSON{"name": "Lin"}, SubmittedAt: when.Add(-2 * time.Second)}, SiteID: &siteID, FormName: "contact", FormTitle: "Contact"},
	}
	accepted, err := model.RestoreSchemaVersion(formID, 1, model.JSON{submission.SensitiveAnnotation: []string{"/secret"}}, model.SchemaVersionPublished, when, nil)
	require.NoError(t, err)
	newer, err := model.RestoreSchemaVersion(formID, 2, model.JSON{}, model.SchemaVersionPublished, when, nil)
	require.NoError(t, err)
	repo.EXPECT().ListWorkspaceSubmissionsPage(gomock.Any(), owner, submission.WorkspaceListOptions{ListOptions: submission.ListOptions{Limit: 25}}).Return(rows, false, nil)
	repo.EXPECT().GetSchemaVersion(gomock.Any(), owner, formID, 1).Return(accepted, nil).Times(1)
	repo.EXPECT().GetSchemaVersion(gomock.Any(), owner, formID, 2).Return(newer, nil).Times(1)
	token, credential, err := auth.Issue(owner, []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	router := echo.New()
	NewV1APIHandler(repo, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
	response := requestJSON(t, router, http.MethodGet, "/v1/submissions", nil, credential, "", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NotContains(t, response.Body.String(), "do-not-show")
	require.NotContains(t, response.Body.String(), "also-hidden")
	var body struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Len(t, body.Data, 3)
	require.Equal(t, siteID, body.Data[0]["siteId"])
	require.Equal(t, "contact", body.Data[0]["formName"])
	require.Equal(t, "Contact", body.Data[0]["formTitle"])
	require.Equal(t, map[string]any{"name": "Ada"}, body.Data[0]["data"])
	require.Equal(t, map[string]any{"name": "Lin"}, body.Data[2]["data"])
	require.Equal(t, float64(25), body.Meta["limit"])
	require.Nil(t, body.Meta["nextCursor"])
}

func TestWorkspaceInboxRejectsInvalidSelectorsBeforeRepository(t *testing.T) {
	repo := mockform.NewMockRepository(gomock.NewController(t))
	token, credential, err := auth.Issue(uuid.NewString(), []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	router := echo.New()
	NewV1APIHandler(repo, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
	for _, query := range []string{"siteId=bad", "formId=bad", "siteId=", "siteId=a&siteId=b", "organizationId=" + uuid.NewString(), "limit=101"} {
		response := requestJSON(t, router, http.MethodGet, "/v1/submissions?"+query, nil, credential, "", nil)
		require.Equal(t, http.StatusBadRequest, response.Code, query+": "+response.Body.String())
	}
}
