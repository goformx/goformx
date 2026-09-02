package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestRequestMediaTypeContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, supplied, expected string
		accepted                 bool
	}{
		{"canonical JSON", "application/json", mediaTypeJSON, true},
		{"case insensitive", "Application/JSON", mediaTypeJSON, true},
		{"UTF-8 parameter", `application/json; charset="UTF-8"`, mediaTypeJSON, true},
		{"canonical merge patch", "application/merge-patch+json", mediaTypeMergePatch, true},
		{"missing", "", mediaTypeJSON, false},
		{"wrong operation media type", "application/json", mediaTypeMergePatch, false},
		{"unsupported charset", "application/json; charset=iso-8859-1", mediaTypeJSON, false},
		{"unknown parameter", "application/json; profile=private", mediaTypeJSON, false},
		{"duplicate parameter", "application/json; charset=utf-8; charset=utf-8", mediaTypeJSON, false},
		{"unrelated media type", "text/plain", mediaTypeJSON, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":1}`))
			if test.supplied != "" {
				request.Header.Set(echo.HeaderContentType, test.supplied)
			}
			err := requireRequestMediaType(request, test.expected)
			if test.accepted {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, errUnsupportedRequestMediaType)
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":1}`))
	request.Header.Add(echo.HeaderContentType, mediaTypeJSON)
	request.Header.Add(echo.HeaderContentType, mediaTypeJSON)
	require.ErrorIs(t, requireRequestMediaType(request, mediaTypeJSON), errUnsupportedRequestMediaType)
}

func TestDecodeJSONRejectsDuplicateMembersBeforeMapCollapse(t *testing.T) {
	t.Parallel()
	for _, document := range []string{
		`{"name":"first","name":"second"}`,
		`{"name":"first","na\u006de":"second"}`,
		`{"schema":{"type":"object","type":"array"}}`,
		`{"items":[{"name":"first","name":"second"}]}`,
	} {
		t.Run(document, func(t *testing.T) {
			var destination map[string]any
			err := decodeJSON(requestContext(document, mediaTypeJSON), &destination, mediaTypeJSON)
			require.ErrorContains(t, err, "duplicate JSON object key")
		})
	}
}

func TestDecodeJSONPreservesExactNumbersAndRejectsMalformedBoundaries(t *testing.T) {
	t.Parallel()
	type requestBody struct {
		Data model.JSON `json:"data"`
	}
	var decoded requestBody
	require.NoError(t, decodeJSON(requestContext(`{"data":{"precise":12345678901234567890.123456789}}`, mediaTypeJSON),
		&decoded, mediaTypeJSON))
	number, ok := decoded.Data["precise"].(json.Number)
	require.True(t, ok)
	require.Equal(t, "12345678901234567890.123456789", number.String())

	for _, document := range []string{
		``, `null`, `[]`, `{"data":{}} {}`, `{"data":`, `{"data":{},"unknown":true}`,
	} {
		var destination requestBody
		require.Error(t, decodeJSON(requestContext(document, mediaTypeJSON), &destination, mediaTypeJSON), document)
	}

	large := `{"data":{"value":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}}`
	var destination requestBody
	require.ErrorContains(t, decodeJSON(requestContext(large, mediaTypeJSON), &destination, mediaTypeJSON), "must not exceed")
}

func requestContext(body, contentType string) echo.Context {
	server := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, contentType)
	return server.NewContext(request, httptest.NewRecorder())
}
