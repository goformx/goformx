package plans

import (
	"github.com/goformx/goforms/internal/domain/common/errors"
)

// featureRequirements maps Form.io component types to the minimum tier required.
var featureRequirements = map[string]string{
	"file":      TierPro,
	"signature": TierPro,
}

var tierRank = map[string]int{
	TierFree:       0,
	TierPro:        1,
	TierBusiness:   2,
	TierEnterprise: 3,
}

func hasTierAccess(userTier, requiredTier string) bool {
	return tierRank[userTier] >= tierRank[requiredTier]
}

// ValidateSchemaFeatures checks if a form schema uses any gated component types.
func ValidateSchemaFeatures(schema map[string]any, planTier string) error {
	components, ok := schema["components"].([]any)
	if !ok {
		return nil
	}

	return validateComponents(components, planTier)
}

func validateComponents(components []any, planTier string) error {
	for _, comp := range components {
		compMap, ok := comp.(map[string]any)
		if !ok {
			continue
		}

		compType, _ := compMap["type"].(string)
		if requiredTier, gated := featureRequirements[compType]; gated {
			if !hasTierAccess(planTier, requiredTier) {
				return errors.NewFeatureNotAvailable(compType, requiredTier)
			}
		}

		// Check nested components recursively
		if nested, ok := compMap["components"].([]any); ok {
			if err := validateComponents(nested, planTier); err != nil {
				return err
			}
		}

		if columns, ok := compMap["columns"].([]any); ok {
			for _, col := range columns {
				if colMap, ok := col.(map[string]any); ok {
					if nested, ok := colMap["components"].([]any); ok {
						if err := validateComponents(nested, planTier); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
