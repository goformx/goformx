package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

func TestSubmissionReadsFailClosedAtPrivacyBoundary(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing_version", "wrong_form", "wrong_version", "malformed_policy", "incompatible_shape", "missing_data"} {
		for _, operation := range []string{"list", "detail"} {
			t.Run(scenario+"/"+operation, func(t *testing.T) {
				repository := mockform.NewMockRepository(gomock.NewController(t))
				token, credential, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
				require.NoError(t, err)
				row := &model.FormSubmission{ID: "submission-a", FormID: "form-a", SchemaVersion: 2,
					Data: model.JSON{"secret": "private-canary"}}
				formID, number := "form-a", 2
				schema := model.JSON{submission.SensitiveAnnotation: []string{"/secret"}}
				switch scenario {
				case "wrong_form":
					formID = "form-b"
				case "wrong_version":
					number = 3
				case "malformed_policy":
					schema[submission.SensitiveAnnotation] = []string{"#/secret"}
				case "incompatible_shape":
					schema[submission.SensitiveAnnotation] = []string{"/secret/child"}
				case "missing_data":
					row.Data = nil
				}
				version, err := model.RestoreSchemaVersion(formID, number, schema, model.SchemaVersionPublished, time.Now(), nil)
				require.NoError(t, err)
				if scenario == "missing_version" {
					version = nil
				}
				repository.EXPECT().GetSchemaVersion(gomock.Any(), "owner-a", "form-a", 2).Return(version, nil)
				path := "/v1/forms/form-a/submissions"
				if operation == "list" {
					// Failure after a successfully projected row must not release a partial page.
					first := &model.FormSubmission{ID: "first-row", FormID: "form-a", SchemaVersion: 1,
						Data: model.JSON{"name": "visible-but-withheld"}}
					firstVersion, err := model.RestoreSchemaVersion("form-a", 1, model.JSON{}, model.SchemaVersionPublished, time.Now(), nil)
					require.NoError(t, err)
					repository.EXPECT().GetFormByID(gomock.Any(), "owner-a", "form-a").Return(&model.Form{ID: "form-a", OrganizationID: "owner-a"}, nil)
					repository.EXPECT().GetSchemaVersion(gomock.Any(), "owner-a", "form-a", 1).Return(firstVersion, nil)
					repository.EXPECT().ListSubmissionsPage(gomock.Any(), "owner-a", "form-a", submission.ListOptions{Limit: 25}).Return([]*model.FormSubmission{first, row}, false, nil)
				} else {
					path += "/submission-a"
					repository.EXPECT().GetSubmissionByOrganization(gomock.Any(), "owner-a", "form-a", "submission-a").Return(row, nil)
				}
				router := echo.New()
				NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
				response := requestJSON(t, router, http.MethodGet, path, "", credential, "", nil)
				require.Equal(t, http.StatusInternalServerError, response.Code)
				require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
				require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
				require.NotContains(t, response.Body.String(), "private-canary")
				require.NotContains(t, response.Body.String(), "visible-but-withheld")
				require.NotContains(t, response.Body.String(), `"data":`)
			})
		}
	}
}
