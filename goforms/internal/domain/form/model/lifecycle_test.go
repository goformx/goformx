package model_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestSchemaVersionIsValidatedImmutableAndPublishesAsCopy(t *testing.T) {
	t.Parallel()
	validator := validation.NewComprehensiveValidator()
	schema := model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	}
	version, err := model.NewSchemaVersion("form-1", 3, schema, validator)
	require.NoError(t, err)
	schema["type"] = "array"
	require.Equal(t, "object", version.Schema()["type"])

	published, err := version.Publish(time.Unix(100, 0))
	require.NoError(t, err)
	require.Equal(t, model.SchemaVersionDraft, version.State())
	require.Equal(t, model.SchemaVersionPublished, published.State())

	invalid := model.JSON{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": "name"}
	_, err = model.NewSchemaVersion("form-1", 4, invalid, validator)
	require.Error(t, err)
}

func TestLifecycleBlocksNonPublishedFormsAndResolvesExactVersion(t *testing.T) {
	t.Parallel()
	key := "gfpk_" + strings.Repeat("a", 32)
	lifecycle, err := model.NewLifecycle(key)
	require.NoError(t, err)
	_, err = lifecycle.ResolvePublicVersion(0, map[int]struct{}{1: {}})
	require.Error(t, err)

	require.NoError(t, lifecycle.Publish(2))
	version, err := lifecycle.ResolvePublicVersion(1, map[int]struct{}{1: {}, 2: {}})
	require.NoError(t, err)
	require.Equal(t, 1, version)
	version, err = lifecycle.ResolvePublicVersion(0, map[int]struct{}{1: {}, 2: {}})
	require.NoError(t, err)
	require.Equal(t, 2, version)

	require.NoError(t, lifecycle.Disable())
	require.False(t, lifecycle.CanAcceptSubmissions())
	lifecycle.Archive()
	require.Error(t, lifecycle.Publish(3))
}

func TestLifecycleStatusRejectsInventedPersistenceValues(t *testing.T) {
	t.Parallel()

	for _, status := range []model.LifecycleStatus{
		model.LifecycleDraft,
		model.LifecyclePublished,
		model.LifecycleDisabled,
		model.LifecycleArchived,
	} {
		require.True(t, status.IsValid())
	}
	require.False(t, model.LifecycleStatus("publishing").IsValid())
}

func TestSubmissionValidationUsesSuppliedImmutableVersion(t *testing.T) {
	t.Parallel()
	validator := validation.NewComprehensiveValidator()
	v1, err := model.NewSchemaVersion("form-1", 1, model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []any{"name"},
	}, validator)
	require.NoError(t, err)
	v2, err := model.NewSchemaVersion("form-1", 2, model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}},
		"required":   []any{"email"},
	}, validator)
	require.NoError(t, err)

	require.True(t, validator.ValidateVersion(v1, model.JSON{"name": "Ada"}).IsValid)
	require.False(t, validator.ValidateVersion(v2, model.JSON{"name": "Ada"}).IsValid)
}
