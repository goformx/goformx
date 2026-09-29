package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/site"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

type siteAPITestRepository struct {
	V1Repository
	values map[string]*site.Site
}

func (r *siteAPITestRepository) CreateSite(_ context.Context, candidate *site.Site) (*site.Site, bool, error) {
	for _, existing := range r.values {
		if existing.OrganizationID == candidate.OrganizationID && existing.Origin == candidate.Origin {
			if existing.Name != candidate.Name {
				return nil, false, common.NewConflictError("create", "site", existing.ID, site.ErrInvalid)
			}
			return existing, false, nil
		}
	}
	candidate.CreatedAt, candidate.UpdatedAt = time.Now(), time.Now()
	r.values[candidate.ID] = candidate
	return candidate, true, nil
}
func (r *siteAPITestRepository) GetSite(_ context.Context, organizationID, id string) (*site.Site, error) {
	value := r.values[id]
	if value == nil || value.OrganizationID != organizationID {
		return nil, common.NewNotFoundError("get", "site", id)
	}
	return value, nil
}
func (r *siteAPITestRepository) ListSites(_ context.Context, organizationID string, _, _ int) ([]*site.Site, int64, error) {
	result := make([]*site.Site, 0)
	for _, value := range r.values {
		if value.OrganizationID == organizationID {
			result = append(result, value)
		}
	}
	return result, int64(len(result)), nil
}

func TestSiteRoutesRespectScopesTenancyAndRetry(t *testing.T) {
	repo := &siteAPITestRepository{values: make(map[string]*site.Site)}
	owner := uuid.NewString()
	readToken, readPlaintext, err := auth.Issue(owner, []auth.Scope{auth.ScopeFormsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	writeToken, writePlaintext, err := auth.Issue(owner, []auth.Scope{auth.ScopeFormsWrite}, time.Hour, time.Now())
	require.NoError(t, err)
	tokens := siteTokenRepository{tokens: map[string]*auth.ServiceToken{readToken.ID: readToken, writeToken.ID: writeToken}}
	router := echo.New()
	NewV1APIHandler(repo, tokens, nil).RegisterRoutes(router)
	empty := requestJSON(t, router, http.MethodGet, "/v1/sites", nil, readPlaintext, "", nil)
	require.Equal(t, http.StatusOK, empty.Code)
	require.Contains(t, empty.Body.String(), `"data":[]`)
	require.Contains(t, empty.Body.String(), `"organizationId":"`+owner+`"`)
	created := requestJSON(t, router, http.MethodPost, "/v1/sites", map[string]any{"name": "Portfolio", "origin": "https://Example.COM:443"}, writePlaintext, "", nil)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	require.Contains(t, created.Body.String(), `"origin":"https://example.com"`)
	retry := requestJSON(t, router, http.MethodPost, "/v1/sites", map[string]any{"name": "Portfolio", "origin": "https://example.com:0443"}, writePlaintext, "", nil)
	require.Equal(t, http.StatusOK, retry.Code, retry.Body.String())
	require.Equal(t, created.Header().Get("Location"), retry.Header().Get("Location"))
	conflict := requestJSON(t, router, http.MethodPost, "/v1/sites", map[string]any{"name": "Other", "origin": "https://example.com"}, writePlaintext, "", nil)
	require.Equal(t, http.StatusConflict, conflict.Code)
	denied := requestJSON(t, router, http.MethodPost, "/v1/sites", map[string]any{"name": "Other", "origin": "https://other.example"}, readPlaintext, "", nil)
	require.Equal(t, http.StatusForbidden, denied.Code)
	denied = requestJSON(t, router, http.MethodGet, "/v1/sites", nil, writePlaintext, "", nil)
	require.Equal(t, http.StatusForbidden, denied.Code)
	listed := requestJSON(t, router, http.MethodGet, "/v1/sites", nil, readPlaintext, "", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), `"total":1`)
	require.Contains(t, listed.Body.String(), `"organizationId":"`+owner+`"`)
	foreign := &site.Site{ID: uuid.NewString(), OrganizationID: uuid.NewString(), Name: "Foreign", Origin: "https://foreign.example"}
	repo.values[foreign.ID] = foreign
	hidden := requestJSON(t, router, http.MethodGet, "/v1/sites/"+foreign.ID, nil, readPlaintext, "", nil)
	require.Equal(t, http.StatusNotFound, hidden.Code)
}

func TestPatchSiteIDNullClearsAssociation(t *testing.T) {
	owner := uuid.NewString()
	form := model.NewForm(owner, "Contact", "", model.JSON{"type": "object"})
	form.SiteID = new(string)
	*form.SiteID = uuid.NewString()
	expected := form.UpdatedAt
	repository := mockform.NewMockRepository(gomock.NewController(t))
	gomock.InOrder(
		repository.EXPECT().GetFormByID(gomock.Any(), owner, form.ID).Return(form, nil),
		repository.EXPECT().UpdateForm(gomock.Any(), form, expected).DoAndReturn(func(_ context.Context, changed *model.Form, _ time.Time) error {
			require.True(t, changed.SiteIDSet)
			require.Nil(t, changed.SiteID)
			return nil
		}),
		repository.EXPECT().GetFormByID(gomock.Any(), owner, form.ID).Return(form, nil),
	)
	token, plaintext, err := auth.Issue(owner, []auth.Scope{auth.ScopeFormsWrite}, time.Hour, time.Now())
	require.NoError(t, err)
	router := echo.New()
	NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
	response := requestJSON(t, router, http.MethodPatch, "/v1/forms/"+form.ID, map[string]any{"siteId": nil}, plaintext, "",
		map[string]string{constants.HeaderIfMatch: formETag(form), echo.HeaderContentType: "application/merge-patch+json"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"siteId":null`)
}

type siteTokenRepository struct{ tokens map[string]*auth.ServiceToken }

func (r siteTokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	return r.tokens[tokenID], nil
}
func (r siteTokenRepository) MarkUsed(_ context.Context, _ string, _ time.Time) error { return nil }
