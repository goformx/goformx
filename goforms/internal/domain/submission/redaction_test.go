package submission_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

func TestRedactionPreservesValuesAndNeverMutatesHistory(t *testing.T) {
	t.Parallel()
	const raw = `{"secret":"private-canary","nested":{"token":"private-canary","visible":9007199254740993},"items":["private-canary",{"decimal":0.1234567890123456789}],"a/b":"private-canary","~1":"private-canary","__proto__":"private-canary","optional":null,"empty":{}}`
	var data model.JSON
	require.NoError(t, json.Unmarshal([]byte(raw), &data))
	paths := []string{"/secret", "/nested/token", "/items/0", "/a~1b", "/~01", "/__proto__", "/absent", "/optional/token"}
	schema := model.JSON{submission.SensitiveAnnotation: paths}
	projected, redacted, err := submission.Redact(schema, data)
	require.NoError(t, err)
	require.ElementsMatch(t, paths, redacted)
	encoded, err := json.Marshal(projected)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-canary")
	require.Contains(t, string(encoded), "9007199254740993")
	require.Contains(t, string(encoded), "0.1234567890123456789")
	require.Contains(t, string(encoded), `"items":[null,`)
	require.Contains(t, string(encoded), `"empty":{}`)
	require.Contains(t, string(encoded), `"optional":null`)
	original, err := json.Marshal(data)
	require.NoError(t, err)
	var expected model.JSON
	require.NoError(t, json.Unmarshal([]byte(raw), &expected))
	require.True(t, model.EqualJSON(expected, data), string(original))
	require.Equal(t, paths, schema[submission.SensitiveAnnotation])
	projected["nested"].(map[string]any)["visible"] = "changed"
	require.Equal(t, json.Number("9007199254740993"), data["nested"].(map[string]any)["visible"])
}

func TestRedactionPolicyIsExplicitBoundedAndFailClosed(t *testing.T) {
	t.Parallel()
	data := model.JSON{"password": "unmarked", "nested": map[string]any{"secret": "marked"}, "items": []any{"value"}}
	plain, paths, err := submission.Redact(model.JSON{"writeOnly": true}, data)
	require.NoError(t, err)
	require.Empty(t, paths)
	require.Equal(t, "unmarked", plain["password"], "Only the documented root policy authorizes redaction")
	for _, rawPolicy := range []any{
		nil, "password", true, []any{1}, []any{"not/a/pointer"}, []any{"#/password"},
		[]any{"/bad~"}, []any{"/bad~2escape"}, []any{"/password", "/password"},
		[]string{"/" + strings.Repeat("x", submission.MaxSensitivePathCharacters)},
		make([]string, submission.MaxSensitivePaths+1),
	} {
		projected, _, err := submission.Redact(model.JSON{submission.SensitiveAnnotation: rawPolicy}, data)
		require.ErrorIs(t, err, submission.ErrRedactionPolicy)
		require.Nil(t, projected)
	}
	for _, invalidPath := range []string{"/password/child", "/items/01", "/items/-", "/items/*", "/items/99999999999999999999999"} {
		projected, _, err := submission.Redact(model.JSON{submission.SensitiveAnnotation: []string{invalidPath}}, data)
		require.ErrorIs(t, err, submission.ErrRedactionPolicy)
		require.Nil(t, projected)
	}
	root, paths, err := submission.Redact(model.JSON{submission.SensitiveAnnotation: []any{""}}, data)
	require.NoError(t, err)
	require.Equal(t, model.JSON{}, root)
	require.Equal(t, []string{""}, paths)
	parent, _, err := submission.Redact(model.JSON{submission.SensitiveAnnotation: []any{"/nested", "/nested/secret", "/items/10"}}, data)
	require.NoError(t, err)
	require.NotContains(t, parent, "nested")
	_, _, err = submission.Redact(nil, data)
	require.ErrorIs(t, err, submission.ErrRedactionPolicy)
	_, _, err = submission.Redact(model.JSON{}, nil)
	require.ErrorIs(t, err, submission.ErrRedactionPolicy, "A missing historical payload cannot masquerade as an empty object")
}

func TestSchemaVersionFreezesSensitivePaths(t *testing.T) {
	t.Parallel()
	paths := []string{"/secret"}
	schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object",
		"properties": map[string]any{"secret": map[string]any{"type": "string"}}, submission.SensitiveAnnotation: paths}
	// Restore covers typed programmatic snapshots as well as decoded []any.
	version, err := model.RestoreSchemaVersion("form-a", 1, schema, model.SchemaVersionPublished, time.Time{}, nil)
	require.NoError(t, err)
	paths[0] = "/elsewhere"
	copy := version.Schema()
	copy[submission.SensitiveAnnotation].([]string)[0] = "/changed"
	frozen, err := submission.SensitivePaths(version.Schema())
	require.NoError(t, err)
	require.Equal(t, []string{"/secret"}, frozen)
	decoded := model.JSON{}
	encoded, err := json.Marshal(version.Schema())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, validation.NewComprehensiveValidator().ValidateSchema(decoded).IsValid)
	decoded[submission.SensitiveAnnotation] = []any{"/bad~2"}
	require.False(t, validation.NewComprehensiveValidator().ValidateSchema(decoded).IsValid)
	schema[submission.SensitiveAnnotation] = []string{}
	empty, err := model.RestoreSchemaVersion("form-a", 2, schema, model.SchemaVersionDraft, time.Time{}, nil)
	require.NoError(t, err)
	encoded, err = json.Marshal(empty.Schema())
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"x-goformx-sensitive":[]`, "An empty policy must not turn into null")
}
