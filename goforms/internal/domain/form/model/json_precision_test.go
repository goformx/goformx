package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestJSONPreservesNumericPrecisionAtRequestAndStorageBoundaries(t *testing.T) {
	t.Parallel()
	const source = `{"integer":9007199254740993,"decimal":0.1234567890123456789,"nested":[{"negative":-9007199254740993}],"empty":{},"text":"9007199254740993"}`
	for name, decode := range map[string]func(*model.JSON) error{
		"request": func(value *model.JSON) error { return json.Unmarshal([]byte(source), value) },
		"storage": func(value *model.JSON) error { return value.Scan([]byte(source)) },
	} {
		t.Run(name, func(t *testing.T) {
			var value model.JSON
			require.NoError(t, decode(&value))
			require.Equal(t, json.Number("9007199254740993"), value["integer"])
			require.Equal(t, json.Number("0.1234567890123456789"), value["decimal"])
			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			require.Contains(t, string(encoded), `"integer":9007199254740993`)
			require.Contains(t, string(encoded), `"decimal":0.1234567890123456789`)
			require.Contains(t, string(encoded), `"negative":-9007199254740993`)
			require.Contains(t, string(encoded), `"empty":{}`)
			require.Contains(t, string(encoded), `"text":"9007199254740993"`)
			stored, err := value.Value()
			require.NoError(t, err)
			var restored model.JSON
			require.NoError(t, restored.Scan(stored))
			require.Equal(t, value, restored)
		})
	}
}

func TestJSONPrecisionCompatibilityAndResourceLimits(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`{"x":1} {"x":2}`, `{"x":1} trailing`, `{"x":01}`, `{"x":NaN}`,
		`{"x":1e1000000000}`, `{"x":1e-1000000000}`, `{"x":1e1025}`, `{"x":1e-1025}`,
		`{"x":` + strings.Repeat("1", model.MaxJSONNumberBytes+1) + `}`,
		`{"x":1e1024}`, `{"x":` + strings.Repeat("1", model.MaxJSONIntegerDigits+1) + `}`,
		`{"x":0.` + strings.Repeat("0", model.MaxJSONFractionDigits) + `1}`,
		`{"x":0.00e-1024}`, `{"x":1.00e-1024}`,
	} {
		var value model.JSON
		require.Error(t, value.UnmarshalJSON([]byte(input)))
		require.Error(t, value.Scan([]byte(input)))
	}
	for _, input := range []string{`{"x":1e1023}`, `{"x":1e-1024}`, `{"x":0.00e-1022}`, `{"x":` + strings.Repeat("1", model.MaxJSONIntegerDigits) + `}`} {
		var value model.JSON
		require.NoError(t, value.UnmarshalJSON([]byte(input)))
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, input, string(encoded))
	}
	var legacy model.JSON
	require.NoError(t, legacy.Scan([]byte(`[9007199254740993,{},null,false]`)))
	require.Equal(t, []any{json.Number("9007199254740993"), map[string]any{}, nil, false}, legacy["data"])
	require.NoError(t, legacy.Scan(nil))
	require.Nil(t, legacy)
	require.NoError(t, json.Unmarshal([]byte("null"), &legacy))
	require.Nil(t, legacy)
	err := legacy.Scan("synthetic-private-value")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic-private-value")
}

func TestJSONExactValuesKeepSchemaValidationBoundariesDistinct(t *testing.T) {
	t.Parallel()
	for name, minimum := range map[string]string{
		"integer": "9007199254740993", "decimal": "0.1234567890123456789",
	} {
		t.Run(name, func(t *testing.T) {
			var schema model.JSON
			require.NoError(t, json.Unmarshal([]byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"value":{"type":"number","minimum":`+minimum+`}}}`), &schema))
			validator := validation.NewComprehensiveValidator()
			var valid, invalid model.JSON
			require.NoError(t, json.Unmarshal([]byte(`{"value":`+minimum+`}`), &valid))
			below := map[string]string{"integer": "9007199254740992", "decimal": "0.1234567890123456788"}[name]
			require.NoError(t, json.Unmarshal([]byte(`{"value":`+below+`}`), &invalid))
			require.True(t, validator.ValidateForm(schema, valid).IsValid)
			require.False(t, validator.ValidateForm(schema, invalid).IsValid, "adjacent values must not collapse through float64")
		})
	}
}

func TestJSONNumericEqualityMatchesValuesNotSpellings(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"1e3", "1000"}, {"0.100", "0.1"}, {"-0", "0"}, {"9007199254740993.0", "9007199254740993"}} {
		require.True(t, model.EqualJSON(model.JSON{"value": json.Number(pair[0])}, model.JSON{"value": json.Number(pair[1])}))
	}
	for _, pair := range [][2]string{{"9007199254740993", "9007199254740992"}, {"0.1234567890123456789", "0.1234567890123456788"}} {
		require.False(t, model.EqualJSON(model.JSON{"value": json.Number(pair[0])}, model.JSON{"value": json.Number(pair[1])}))
	}
	require.True(t, model.EqualJSON(model.JSON{"nested": []any{map[string]any{"value": 1}}}, model.JSON{"nested": []any{map[string]any{"value": json.Number("1.0")}}}))
	require.False(t, model.EqualJSON(model.JSON{"value": "1"}, model.JSON{"value": 1}))
	require.False(t, model.EqualJSON(model.JSON{"a": nil}, model.JSON{"b": nil}))
	require.False(t, model.EqualJSON(model.JSON{"value": json.Number("1e1000000000")}, model.JSON{"value": 1}))
}
