package plans_test

import (
	"testing"

	"github.com/goformx/goforms/internal/domain/common/plans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLimits_KnownTiers(t *testing.T) {
	tests := []struct {
		tier           string
		maxForms       int
		maxSubmissions int
	}{
		{"free", 3, 100},
		{"pro", 10, 1000},
		{"business", 50, 10000},
		{"growth", 150, 50000},
		{"enterprise", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			limits, err := plans.GetLimits(tt.tier)
			require.NoError(t, err)
			assert.Equal(t, tt.maxForms, limits.MaxForms)
			assert.Equal(t, tt.maxSubmissions, limits.MaxSubmissionsPerMonth)
		})
	}
}

func TestGetLimits_UnknownTier_ReturnsError(t *testing.T) {
	_, err := plans.GetLimits("unknown")
	require.Error(t, err)
}

func TestIsUnlimited(t *testing.T) {
	limits, _ := plans.GetLimits("enterprise")
	assert.True(t, limits.IsUnlimited())

	limits, _ = plans.GetLimits("free")
	assert.False(t, limits.IsUnlimited())
}

func TestIsValidTier(t *testing.T) {
	assert.True(t, plans.IsValidTier("free"))
	assert.True(t, plans.IsValidTier("pro"))
	assert.True(t, plans.IsValidTier("business"))
	assert.True(t, plans.IsValidTier("growth"))
	assert.True(t, plans.IsValidTier("enterprise"))
	assert.False(t, plans.IsValidTier("unknown"))
	assert.False(t, plans.IsValidTier(""))
}

func TestNextTier(t *testing.T) {
	assert.Equal(t, "pro", plans.NextTier("free"))
	assert.Equal(t, "business", plans.NextTier("pro"))
	assert.Equal(t, "growth", plans.NextTier("business"))
	assert.Equal(t, "enterprise", plans.NextTier("growth"))
	assert.Empty(t, plans.NextTier("enterprise"))
}
