package web

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	deliveryapp "github.com/goformx/goforms/internal/application/webhook"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

type webhookAPIRepository struct {
	V1Repository
	endpoint   *domainwebhook.Endpoint
	secret     domainwebhook.SecretConfig
	deliveries []*domainwebhook.Delivery
	replayed   string
}

func (r *webhookAPIRepository) PutWebhookEndpoint(
	_ context.Context,
	formID, destinationURL string,
	config domainwebhook.SecretConfig,
	enabled bool,
) (*domainwebhook.Endpoint, error) {
	r.secret = config
	r.endpoint = &domainwebhook.Endpoint{ID: "22222222-2222-4222-8222-222222222222",
		FormID: formID, Origin: "https://hooks.example.com", Enabled: enabled,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return r.endpoint, nil
}

func (r *webhookAPIRepository) GetWebhookEndpoint(context.Context, string) (*domainwebhook.Endpoint, error) {
	if r.endpoint == nil {
		return nil, domainwebhook.ErrNotFound
	}
	return r.endpoint, nil
}

func (r *webhookAPIRepository) DeleteWebhookEndpoint(context.Context, string) error {
	r.endpoint = nil
	return nil
}

func (r *webhookAPIRepository) ListWebhookDeliveries(context.Context, string, int) ([]*domainwebhook.Delivery, error) {
	return r.deliveries, nil
}

func (r *webhookAPIRepository) ReplayWebhookDelivery(_ context.Context, _ string, deliveryID string) error {
	r.replayed = deliveryID
	return nil
}

type webhookResolver map[string][]netip.Addr

func (r webhookResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addresses := r[host]
	if len(addresses) == 0 {
		return nil, &net.DNSError{Err: "not found", Name: host}
	}
	return addresses, nil
}

func TestWebhookAPIKeepsSecretsWriteOnlyAndBlocksUnsafeInputs(t *testing.T) {
	t.Parallel()
	base := mockform.NewMockRepository(gomock.NewController(t))
	base.EXPECT().GetFormByID(gomock.Any(), "11111111-1111-4111-8111-111111111111").
		Return(&model.Form{ID: "11111111-1111-4111-8111-111111111111", UserID: "owner-a"}, nil).
		AnyTimes()
	repository := &webhookAPIRepository{V1Repository: base}
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{
		auth.ScopeFormsRead, auth.ScopeFormsWrite, auth.ScopeSubmissionsRead,
	}, time.Hour, time.Now())
	require.NoError(t, err)
	handler := newV1APIHandler(repository, fixedTokenRepository{token: token}, nil, nil)
	handler.destinations = deliveryapp.NewDestinationPolicy(webhookResolver{
		"hooks.example.com": {netip.MustParseAddr("8.8.8.8")},
		"private.example":   {netip.MustParseAddr("10.0.0.1")},
	})
	router := echo.New()
	handler.RegisterRoutes(router)
	path := "/v1/forms/11111111-1111-4111-8111-111111111111/webhook"
	secret := "signing-secret-with-at-least-32-characters"
	response := requestJSON(t, router, http.MethodPut, path, map[string]any{
		"url":           "https://hooks.example.com/receive",
		"headers":       map[string]string{"Authorization": "Bearer never-return-this"},
		"signingSecret": secret,
	}, plaintext, "", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, secret, repository.secret.SigningSecret)
	require.NotContains(t, response.Body.String(), secret)
	require.NotContains(t, response.Body.String(), "Authorization")
	require.NotContains(t, response.Body.String(), "never-return-this")
	require.NotContains(t, response.Body.String(), "/receive")

	unsafe := requestJSON(t, router, http.MethodPut, path, map[string]any{
		"url": "https://private.example/receive", "signingSecret": secret,
	}, plaintext, "", nil)
	require.Equal(t, http.StatusUnprocessableEntity, unsafe.Code, unsafe.Body.String())
	require.Contains(t, unsafe.Body.String(), "unsafe_destination")

	reserved := requestJSON(t, router, http.MethodPut, path, map[string]any{
		"url": "https://hooks.example.com/receive", "signingSecret": secret,
		"headers": map[string]string{deliveryapp.HeaderSignature: "forged"},
	}, plaintext, "", nil)
	require.Equal(t, http.StatusUnprocessableEntity, reserved.Code, reserved.Body.String())
	require.Contains(t, reserved.Body.String(), "reserved_header")
}

func TestWebhookDeliveryStatusAndDeadLetterReplay(t *testing.T) {
	t.Parallel()
	base := mockform.NewMockRepository(gomock.NewController(t))
	formID := "11111111-1111-4111-8111-111111111111"
	deliveryID := "33333333-3333-4333-8333-333333333333"
	base.EXPECT().GetFormByID(gomock.Any(), formID).
		Return(&model.Form{ID: formID, UserID: "owner-a"}, nil).AnyTimes()
	repository := &webhookAPIRepository{V1Repository: base, deliveries: []*domainwebhook.Delivery{{
		ID: deliveryID, SubmissionID: "22222222-2222-4222-8222-222222222222",
		Status: domainwebhook.DeliveryDeadLetter, AttemptCount: 8, LastErrorCategory: "network",
		CreatedAt: time.Now(), UpdatedAt: time.Now(), NextAttemptAt: time.Now(),
	}}}
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{
		auth.ScopeFormsWrite, auth.ScopeSubmissionsRead,
	}, time.Hour, time.Now())
	require.NoError(t, err)
	router := echo.New()
	newV1APIHandler(repository, fixedTokenRepository{token: token}, nil, nil).RegisterRoutes(router)

	status := requestJSON(t, router, http.MethodGet, "/v1/forms/"+formID+"/deliveries",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, status.Code, status.Body.String())
	require.Contains(t, status.Body.String(), `"status":"dead_letter"`)
	require.NotContains(t, status.Body.String(), "encrypted")

	replay := requestJSON(t, router, http.MethodPost,
		"/v1/forms/"+formID+"/deliveries/"+deliveryID+"/replay", nil, plaintext, "", nil)
	require.Equal(t, http.StatusAccepted, replay.Code, replay.Body.String())
	require.Equal(t, deliveryID, repository.replayed)
}
