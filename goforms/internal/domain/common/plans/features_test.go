package plans_test

import (
	"testing"

	"github.com/goformx/goforms/internal/domain/common/plans"
	"github.com/stretchr/testify/assert"
)

func TestValidateSchemaFeatures_FileUploadOnFree_ReturnsError(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.Error(t, err)
}

func TestValidateSchemaFeatures_FileUploadOnPro_Succeeds(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "pro")
	assert.NoError(t, err)
}

func TestValidateSchemaFeatures_BasicFieldsOnFree_Succeeds(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "textfield", "key": "name"},
			map[string]any{"type": "email", "key": "email"},
			map[string]any{"type": "button", "key": "submit"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.NoError(t, err)
}

func TestValidateSchemaFeatures_Enterprise_AllowsEverything(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "file", "key": "upload"},
			map[string]any{"type": "signature", "key": "sig"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "enterprise")
	assert.NoError(t, err)
}

func TestValidateSchemaFeatures_SignatureOnFree_ReturnsError(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{"type": "signature", "key": "sig"},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.Error(t, err)
}

func TestValidateSchemaFeatures_NestedGatedComponent_ReturnsError(t *testing.T) {
	schema := map[string]any{
		"components": []any{
			map[string]any{
				"type": "panel",
				"key":  "panel1",
				"components": []any{
					map[string]any{"type": "file", "key": "upload"},
				},
			},
		},
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.Error(t, err)
}

func TestValidateSchemaFeatures_NoComponents_Succeeds(t *testing.T) {
	schema := map[string]any{
		"type": "object",
	}
	err := plans.ValidateSchemaFeatures(schema, "free")
	assert.NoError(t, err)
}
