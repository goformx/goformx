package web

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.yaml.in/yaml/v3"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/validation"
	deliveryapp "github.com/goformx/goforms/internal/application/webhook"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	domainsubmission "github.com/goformx/goforms/internal/domain/submission"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

const (
	scopeOrganizationID = "11111111-1111-4111-8111-111111111111"
	scopeFormID         = "22222222-2222-4222-8222-222222222222"
	scopeResourceID     = "33333333-3333-4333-8333-333333333333"
)

type managementOperation struct {
	ID       string                `yaml:"operationId"`
	Security []map[string][]string `yaml:"security"`
	Scopes   []auth.Scope          `yaml:"x-goformx-required-scopes"`
	method   string
	path     string
}

// Keep the contract independent of production scope wiring: copying the route
// table's scopes into test cases would reproduce the same mistake on both sides.
func managementOperations(t *testing.T) []managementOperation {
	t.Helper()
	document, err := os.ReadFile("../../../../contracts/openapi.v1.yaml")
	require.NoError(t, err)
	var contract struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(document, &contract))
	var operations []managementOperation
	seen := map[string]bool{}
	for path, item := range contract.Paths {
		if !strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1/public/") {
			continue
		}
		for method, node := range item {
			if !slices.Contains([]string{"get", "post", "put", "patch", "delete", "head", "options", "trace"}, method) {
				continue
			}
			var operation managementOperation
			require.NoError(t, node.Decode(&operation))
			require.NotEmpty(t, operation.ID)
			require.False(t, seen[operation.ID], "duplicate operationId: %s", operation.ID)
			seen[operation.ID] = true
			require.Len(t, operation.Scopes, 1, "%s needs one required scope", operation.ID)
			require.True(t, operation.Scopes[0].Valid())
			require.ElementsMatch(t, []map[string][]string{{"serviceToken": {}}, {"firstPartyAssertion": {}}}, operation.Security)
			operation.method, operation.path = strings.ToUpper(method), path
			operations = append(operations, operation)
		}
	}
	require.NotEmpty(t, operations)
	slices.SortFunc(operations, func(a, b managementOperation) int { return strings.Compare(a.ID, b.ID) })
	return operations
}

func TestManagementScopeContract(t *testing.T) {
	t.Parallel()
	operations := managementOperations(t)
	assertManagementRouteInventory(t, operations)
	for _, operation := range operations {
		t.Run(operation.ID, func(t *testing.T) {
			t.Run("anonymous", func(t *testing.T) {
				runManagementScopeCase(t, operation, "anonymous", nil)
			})
			for _, credentialClass := range []string{"serviceToken", "firstPartyAssertion"} {
				t.Run(credentialClass, func(t *testing.T) {
					for _, scope := range auth.AllScopes() {
						t.Run(string(scope), func(t *testing.T) {
							runManagementScopeCase(t, operation, credentialClass, []auth.Scope{scope})
						})
					}
					t.Run("all_except_required", func(t *testing.T) {
						scopes := slices.DeleteFunc(auth.AllScopes(), func(scope auth.Scope) bool { return scope == operation.Scopes[0] })
						runManagementScopeCase(t, operation, credentialClass, scopes)
					})
				})
			}
		})
	}
}

func assertManagementRouteInventory(t *testing.T, operations []managementOperation) {
	t.Helper()
	router := echo.New()
	NewV1APIHandler(nil, nil, nil).RegisterRoutes(router)
	parameters := regexp.MustCompile(`\{([^}]+)\}`)
	var expected, actual []string
	for _, operation := range operations {
		expected = append(expected, operation.method+" "+parameters.ReplaceAllString(operation.path, ":$1"))
	}
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/v1/public/") {
			continue
		}
		actual = append(actual, route.Method+" "+route.Path)
	}
	require.ElementsMatch(t, expected, actual, "every registered management route must have a contract and vice versa")
}

type scopeRepositories struct {
	*mockform.MockRepository
	*mockform.MockWebhookRepository
}

type scopeTokenRepository struct {
	fixedTokenRepository
	*mockform.MockServiceTokenManagementRepository
}

func runManagementScopeCase(t *testing.T, operation managementOperation, credentialClass string, scopes []auth.Scope) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repositories := scopeRepositories{mockform.NewMockRepository(ctrl), mockform.NewMockWebhookRepository(ctrl)}
	tokens := scopeTokenRepository{MockServiceTokenManagementRepository: mockform.NewMockServiceTokenManagementRepository(ctrl)}
	verifier, assertion := scopeAssertion(t, scopes)
	var credential string
	switch credentialClass {
	case "serviceToken":
		token, plaintext, err := auth.Issue(scopeOrganizationID, scopes, time.Hour, time.Now())
		require.NoError(t, err)
		tokens.token, credential = token, plaintext
	case "firstPartyAssertion":
		credential = assertion
	}
	handler := NewV1APIHandlerWithLimits(repositories, tokens, nil, DefaultV1Limits(), verifier)
	handler.destinations = deliveryapp.NewDestinationPolicy(webhookResolver{
		"hooks.example.com": {netip.MustParseAddr("8.8.8.8")},
	})
	router := echo.New()
	handler.RegisterRoutes(router)

	allowed := slices.Contains(scopes, operation.Scopes[0])
	fixture := managementSuccessFixture(t, operation.ID, repositories, tokens.MockServiceTokenManagementRepository, allowed, credentialClass)
	status := http.StatusForbidden
	if credentialClass == "anonymous" {
		status = http.StatusUnauthorized
	} else if allowed {
		status = fixture.status
	}
	path := strings.NewReplacer("{formId}", scopeFormID, "{version}", "1",
		"{submissionId}", scopeResourceID, "{deliveryId}", scopeResourceID, "{tokenId}", scopeResourceID).Replace(operation.path)
	require.NotContains(t, path, "{", "new path parameter needs a concrete fixture")
	response := requestJSON(t, router, operation.method, path, fixture.body, credential, "", fixture.headers)
	require.Equal(t, status, response.Code, response.Body.String())
	if allowed {
		require.Equal(t, uint64(1), handler.requests.Load(), "correct scope must dispatch the real business handler")
		if status != http.StatusNoContent {
			require.Contains(t, response.Body.String(), `"data":`)
		}
	} else {
		require.Zero(t, handler.requests.Load(), "authorization must reject before handler dispatch")
		require.Contains(t, response.Body.String(), `"error":`)
	}
	// Denied cases install no repository expectations. Any read or mutation fails
	// through gomock, including webhook and service-token management repositories.
}

type managementFixture struct {
	status  int
	body    any
	headers map[string]string
}

func managementSuccessFixture(t *testing.T, operation string, repositories scopeRepositories,
	tokens *mockform.MockServiceTokenManagementRepository, allowed bool, credentialClass string,
) managementFixture {
	t.Helper()
	fixture := managementFixture{status: http.StatusOK}
	schema := contactSchema("email")
	form := model.NewForm(scopeOrganizationID, "Scope fixture", "", schema)
	form.ID, form.Name = scopeFormID, "scope-fixture"
	form.UpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	version, err := model.NewSchemaVersion(scopeFormID, 1, schema, validation.NewComprehensiveValidator())
	require.NoError(t, err)
	submission := &model.FormSubmission{ID: scopeResourceID, FormID: scopeFormID, SchemaVersion: 1, Data: model.JSON{}}
	endpoint := &domainwebhook.Endpoint{ID: scopeResourceID, FormID: scopeFormID, Enabled: true}
	ownedForm := func() {
		if allowed {
			repositories.MockRepository.EXPECT().GetFormByID(gomock.Any(), scopeOrganizationID, scopeFormID).Return(form, nil)
		}
	}
	switch operation {
	case "listForms":
		if allowed {
			repositories.MockRepository.EXPECT().ListForms(gomock.Any(), scopeOrganizationID, gomock.Any()).Return([]*model.Form{form}, 1, nil)
		}
	case "createForm":
		fixture.status, fixture.body = http.StatusCreated, map[string]any{"name": form.Name, "title": form.Title, "schema": schema}
		if allowed {
			repositories.MockRepository.EXPECT().CreateForm(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, created *model.Form) error {
				require.Equal(t, scopeOrganizationID, created.OrganizationID)
				require.Equal(t, form.Name, created.Name)
				created.ID = scopeFormID
				return nil
			})
		}
	case "getForm":
		ownedForm()
	case "updateForm":
		ownedForm()
		fixture.body, fixture.headers = map[string]string{"title": "Updated title"}, map[string]string{constants.HeaderIfMatch: formETag(form)}
		if allowed {
			repositories.MockRepository.EXPECT().UpdateForm(gomock.Any(), form, form.UpdatedAt).Return(nil)
			ownedForm()
		}
	case "listSchemaVersions":
		ownedForm()
		if allowed {
			repositories.MockRepository.EXPECT().ListSchemaVersions(gomock.Any(), scopeOrganizationID, scopeFormID, 25, 0).Return([]*model.SchemaVersion{version}, 1, nil)
		}
	case "createSchemaVersion":
		ownedForm()
		fixture.status, fixture.body = http.StatusCreated, map[string]any{"schema": schema}
		if allowed {
			repositories.MockRepository.EXPECT().CreateSchemaVersion(gomock.Any(), scopeOrganizationID, scopeFormID, schema).Return(version, nil)
		}
	case "getSchemaVersion", "publishSchemaVersion":
		ownedForm()
		if allowed {
			repositories.MockRepository.EXPECT().GetSchemaVersion(gomock.Any(), scopeOrganizationID, scopeFormID, 1).Return(version, nil)
			if operation == "publishSchemaVersion" {
				published, publishErr := version.Publish(time.Now())
				require.NoError(t, publishErr)
				repositories.MockRepository.EXPECT().PublishSchemaVersion(gomock.Any(), scopeOrganizationID, scopeFormID, 1).Return(published, nil)
			}
		}
	case "listSubmissions":
		ownedForm()
		if allowed {
			repositories.MockRepository.EXPECT().GetSchemaVersion(gomock.Any(), scopeOrganizationID, scopeFormID, 1).Return(version, nil)
			repositories.MockRepository.EXPECT().ListSubmissionsPage(gomock.Any(), scopeOrganizationID, scopeFormID, domainsubmission.ListOptions{Limit: 25}).Return([]*model.FormSubmission{submission}, false, nil)
		}
	case "getSubmission":
		if allowed {
			repositories.MockRepository.EXPECT().GetSchemaVersion(gomock.Any(), scopeOrganizationID, scopeFormID, 1).Return(version, nil)
			repositories.MockRepository.EXPECT().GetSubmissionByOrganization(gomock.Any(), scopeOrganizationID, scopeFormID, scopeResourceID).Return(submission, nil)
		}
	case "exportSubmissions":
		ownedForm()
		fixture.body = map[string]string{"format": "json"}
		if allowed {
			repositories.MockRepository.EXPECT().ReadSubmissionExport(gomock.Any(), scopeOrganizationID, scopeFormID, domainsubmission.ExportFilters{}).
				Return([]domainsubmission.ExportRecord{{Submission: submission, SchemaFormID: scopeFormID, AcceptedVersion: 1, Policy: model.JSON{}}}, nil)
			repositories.MockRepository.EXPECT().SaveSubmissionExportAudit(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, audit domainsubmission.ExportAudit) error {
				require.NoError(t, audit.Validate())
				require.Equal(t, scopeOrganizationID, audit.OrganizationID)
				require.Equal(t, scopeFormID, audit.FormID)
				require.Equal(t, 1, audit.RowCount)
				require.Equal(t, map[string]string{"serviceToken": "service_token", "firstPartyAssertion": "first_party_assertion"}[credentialClass], audit.CredentialClass)
				if credentialClass == "serviceToken" {
					require.Equal(t, audit.CredentialID, audit.SubjectID)
				} else {
					require.NotEqual(t, audit.CredentialID, audit.SubjectID)
				}
				return nil
			})
		}
	case "getWebhookEndpoint":
		ownedForm()
		if allowed {
			repositories.MockWebhookRepository.EXPECT().GetWebhookEndpoint(gomock.Any(), scopeOrganizationID, scopeFormID).Return(endpoint, nil)
		}
	case "putWebhookEndpoint":
		ownedForm()
		secret := domainwebhook.SecretConfig{DestinationURL: "https://hooks.example.com/receive", SigningSecret: strings.Repeat("s", 32)}
		fixture.body = map[string]any{"url": secret.DestinationURL, "signingSecret": secret.SigningSecret}
		if allowed {
			repositories.MockWebhookRepository.EXPECT().PutWebhookEndpoint(gomock.Any(), scopeOrganizationID, scopeFormID, secret.DestinationURL, secret, true).Return(endpoint, nil)
		}
	case "deleteWebhookEndpoint":
		ownedForm()
		fixture.status = http.StatusNoContent
		if allowed {
			repositories.MockWebhookRepository.EXPECT().DeleteWebhookEndpoint(gomock.Any(), scopeOrganizationID, scopeFormID).Return(nil)
		}
	case "listWebhookDeliveries":
		ownedForm()
		if allowed {
			repositories.MockWebhookRepository.EXPECT().ListWebhookDeliveries(gomock.Any(), scopeOrganizationID, scopeFormID, 25).Return([]*domainwebhook.Delivery{}, nil)
		}
	case "replayWebhookDelivery":
		ownedForm()
		fixture.status = http.StatusAccepted
		if allowed {
			repositories.MockWebhookRepository.EXPECT().ReplayWebhookDelivery(gomock.Any(), scopeOrganizationID, scopeFormID, scopeResourceID).Return(nil)
		}
	case "listServiceTokens":
		if allowed {
			tokens.EXPECT().ListByOrganization(gomock.Any(), scopeOrganizationID, 25).Return([]*auth.ServiceToken{}, nil)
		}
	case "createServiceToken":
		fixture.status, fixture.body = http.StatusCreated, map[string]any{"name": "Delegated token", "scopes": []auth.Scope{auth.ScopeTokensWrite}}
		if allowed {
			tokens.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, token *auth.ServiceToken, actor auth.AuditActor) error {
				require.Equal(t, scopeOrganizationID, token.OwnerID)
				require.NoError(t, actor.Validate())
				require.Equal(t, scopeOrganizationID, actor.OrganizationID)
				require.Equal(t, map[string]auth.CredentialClass{"serviceToken": auth.CredentialClassServiceToken,
					"firstPartyAssertion": auth.CredentialClassFirstPartyAssertion}[credentialClass], actor.CredentialClass)
				require.Equal(t, map[auth.Scope]struct{}{auth.ScopeTokensWrite: {}}, token.Scopes)
				return nil
			})
		}
	case "revokeServiceToken":
		fixture.status = http.StatusNoContent
		if allowed {
			tokens.EXPECT().RevokeByOrganization(gomock.Any(), scopeOrganizationID, scopeResourceID, gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _, _ string, _ time.Time, actor auth.AuditActor) error {
					require.NoError(t, actor.Validate())
					require.Equal(t, scopeOrganizationID, actor.OrganizationID)
					return nil
				})
		}
	default:
		t.Fatalf("management operation %q needs an explicit successful request fixture", operation)
	}
	return fixture
}

type scopeAssertionStore struct {
	key  auth.VerificationKey
	seen map[string]bool
}

func (s *scopeAssertionStore) FindKey(_ context.Context, id string) (auth.VerificationKey, error) {
	if id != s.key.ID {
		return auth.VerificationKey{}, auth.ErrInvalidFirstPartyAssertion
	}
	return s.key, nil
}

func (s *scopeAssertionStore) Consume(_ context.Context, replay auth.AssertionReplay) error {
	id := replay.Issuer + "/" + replay.AssertionID
	if s.seen[id] {
		return auth.ErrFirstPartyAssertionReplay
	}
	s.seen[id] = true
	return nil
}

// Exercise the production Ed25519 verifier, not an always-authenticated stub.
// PostgreSQL atomic replay and key rotation have separate integration tests.
func scopeAssertion(t *testing.T, scopes []auth.Scope) (*auth.FirstPartyVerifier, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &scopeAssertionStore{key: auth.VerificationKey{ID: "scope-matrix", PublicKey: public, State: auth.VerificationKeyActive}, seen: map[string]bool{}}
	verifier, err := auth.NewFirstPartyVerifier("https://control.example", "https://api.example", store, store)
	require.NoError(t, err)
	encode := func(value any) string {
		data, marshalErr := json.Marshal(value)
		require.NoError(t, marshalErr)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	now := time.Now().Unix()
	header := encode(map[string]any{"alg": auth.FirstPartyAssertionAlgorithm, "typ": auth.FirstPartyAssertionType, "kid": store.key.ID})
	claims := encode(map[string]any{
		"iss": "https://control.example", "aud": "https://api.example", "sub": scopeResourceID,
		"org": scopeOrganizationID, "scp": scopes, "iat": now, "nbf": now, "exp": now + 60,
		"jti": uuid.NewString(), "rid": uuid.NewString(), "ver": auth.FirstPartyAssertionVersion,
	})
	message := header + "." + claims
	return verifier, message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(message)))
}
