package plans

import "fmt"

const (
	TierFree       = "free"
	TierPro        = "pro"
	TierBusiness   = "business"
	TierEnterprise = "enterprise"

	freeForms               = 3
	freeSubmissionsPerMonth = 100
	proForms                = 25
	proSubmissionsPerMonth  = 2500
	bizForms                = 100
	bizSubmissionsPerMonth  = 25000
)

// Limits defines the usage limits for a subscription plan tier.
type Limits struct {
	MaxForms               int
	MaxSubmissionsPerMonth int
}

// IsUnlimited returns true when both limits are zero (enterprise tier).
func (l Limits) IsUnlimited() bool {
	return l.MaxForms == 0 && l.MaxSubmissionsPerMonth == 0
}

var tierLimits = map[string]Limits{
	TierFree:       {MaxForms: freeForms, MaxSubmissionsPerMonth: freeSubmissionsPerMonth},
	TierPro:        {MaxForms: proForms, MaxSubmissionsPerMonth: proSubmissionsPerMonth},
	TierBusiness:   {MaxForms: bizForms, MaxSubmissionsPerMonth: bizSubmissionsPerMonth},
	TierEnterprise: {MaxForms: 0, MaxSubmissionsPerMonth: 0},
}

var tierOrder = []string{TierFree, TierPro, TierBusiness, TierEnterprise}

// GetLimits returns the usage limits for the given plan tier.
func GetLimits(tier string) (Limits, error) {
	limits, ok := tierLimits[tier]
	if !ok {
		return Limits{}, fmt.Errorf("unknown plan tier: %s", tier)
	}

	return limits, nil
}

// IsValidTier returns whether the given string is a recognized plan tier.
func IsValidTier(tier string) bool {
	_, ok := tierLimits[tier]

	return ok
}

// NextTier returns the next higher tier, or empty string if already at the highest.
func NextTier(current string) string {
	for i, t := range tierOrder {
		if t == current && i+1 < len(tierOrder) {
			return tierOrder[i+1]
		}
	}

	return ""
}
