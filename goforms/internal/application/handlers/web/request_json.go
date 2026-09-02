package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	mediaTypeJSON       = "application/json"
	mediaTypeMergePatch = "application/merge-patch+json"
	maxRequestBodyBytes = 1 << 20
)

var errUnsupportedRequestMediaType = errors.New("unsupported request media type")

func decodeJSON(c echo.Context, destination any, expectedMediaType string) error {
	if err := requireRequestMediaType(c.Request(), expectedMediaType); err != nil {
		return err
	}
	document, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, maxRequestBodyBytes))
	if err != nil {
		return fmt.Errorf("request body must not exceed %d bytes: %w", maxRequestBodyBytes, err)
	}
	if err := rejectDuplicateJSONMembers(document); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func requireRequestMediaType(request *http.Request, expected string) error {
	if len(request.Header.Values(echo.HeaderContentType)) != 1 {
		return errUnsupportedRequestMediaType
	}
	raw := request.Header.Get(echo.HeaderContentType)
	mediaType, parameters, err := mime.ParseMediaType(raw)
	if err != nil || !strings.EqualFold(mediaType, expected) {
		return errUnsupportedRequestMediaType
	}
	for name, value := range parameters {
		if !strings.EqualFold(name, "charset") || !strings.EqualFold(value, "utf-8") {
			return errUnsupportedRequestMediaType
		}
	}
	if len(parameters) > 0 {
		charsetParameters := 0
		for _, parameter := range strings.Split(raw, ";")[1:] {
			name, _, present := strings.Cut(parameter, "=")
			if present && strings.EqualFold(strings.TrimSpace(name), "charset") {
				charsetParameters++
			}
		}
		if charsetParameters != 1 {
			return errUnsupportedRequestMediaType
		}
	}
	return nil
}

func rejectDuplicateJSONMembers(document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return errors.New("request body must contain one JSON object")
	}
	if err := walkJSONMembers(decoder, token); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func walkJSONMembers(decoder *json.Decoder, token json.Token) error {
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is invalid")
			}
			if _, duplicate := seen[key]; duplicate {
				return errors.New("duplicate JSON object key")
			}
			seen[key] = struct{}{}
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := walkJSONMembers(decoder, value); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := walkJSONMembers(decoder, value); err != nil {
				return err
			}
		}
	default:
		return errors.New("JSON delimiter is invalid")
	}
	end, err := decoder.Token()
	expectedEnd := json.Delim('}')
	if delimiter == '[' {
		expectedEnd = ']'
	}
	if err != nil || end != json.Token(expectedEnd) {
		return errors.New("JSON composite is incomplete")
	}
	return nil
}

func (h *V1APIHandler) writeRequestDecodeError(c echo.Context, err error, safeMessage string) error {
	if errors.Is(err, errUnsupportedRequestMediaType) {
		return h.writeError(c, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must match the operation's documented JSON media type; only an optional UTF-8 charset parameter is accepted.", nil)
	}
	if safeMessage != "" {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", safeMessage, nil)
	}
	return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
}
