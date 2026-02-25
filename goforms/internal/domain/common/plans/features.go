package plans

import (
	"github.com/goformx/goforms/internal/domain/common/errors"
)

// featureRequirements maps Form.io component types to the minimum tier required.
var featureRequirements = map[string]string{
	"file":      TierPro,
	"signature": TierPro,
}

// Tier rank constants for comparison ordering.
const (
	tierRankFree       = 0
	tierRankPro        = 1
	tierRankBusiness   = 2
	tierRankEnterprise = 3
)

var tierRank = map[string]int{
	TierFree:       tierRankFree,
	TierPro:        tierRankPro,
	TierBusiness:   tierRankBusiness,
	TierEnterprise: tierRankEnterprise,
}

func hasTierAccess(userTier, requiredTier string) bool {
	userRank, uOK := tierRank[userTier]
	reqRank, rOK := tierRank[requiredTier]
	if !uOK || !rOK {
		return false
	}

	return userRank >= reqRank
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

		if err := checkComponentFeature(compMap, planTier); err != nil {
			return err
		}

		if err := validateNestedComponents(compMap, planTier); err != nil {
			return err
		}
	}

	return nil
}

func checkComponentFeature(compMap map[string]any, planTier string) error {
	compType, _ := compMap["type"].(string)
	if requiredTier, gated := featureRequirements[compType]; gated {
		if !hasTierAccess(planTier, requiredTier) {
			return errors.NewFeatureNotAvailable(compType, requiredTier)
		}
	}

	return nil
}

func validateNestedComponents(compMap map[string]any, planTier string) error {
	if nested, ok := compMap["components"].([]any); ok {
		if err := validateComponents(nested, planTier); err != nil {
			return err
		}
	}

	if columns, ok := compMap["columns"].([]any); ok {
		if err := validateColumnComponents(columns, planTier); err != nil {
			return err
		}
	}

	return nil
}

func validateColumnComponents(columns []any, planTier string) error {
	for _, col := range columns {
		colMap, ok := col.(map[string]any)
		if !ok {
			continue
		}

		nested, hasNested := colMap["components"].([]any)
		if hasNested {
			if err := validateComponents(nested, planTier); err != nil {
				return err
			}
		}
	}

	return nil
}
