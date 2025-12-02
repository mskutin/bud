package policy

import (
	"testing"

	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestResolvePolicy_AccountPriority(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{
				Account:       "123456789012",
				Name:          "Critical Account",
				GrowthBuffer:  10,
				MinimumBudget: 100,
			},
		},
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-prod-12345678",
				Name:          "Production OU",
				GrowthBuffer:  15,
				MinimumBudget: 50,
			},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"

	// Account policy should take priority over OU policy
	policy := resolver.ResolvePolicy("123456789012")

	assert.Equal(t, "Critical Account", policy.Name)
	assert.Equal(t, 10.0, policy.GrowthBuffer)
	assert.Equal(t, 100.0, policy.MinimumBudget)
	assert.Equal(t, 10.0, policy.RoundingIncrement) // Inherited from default
}

func TestResolvePolicy_OUPriority(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-prod-12345678",
				Name:          "Production OU",
				GrowthBuffer:  15,
				MinimumBudget: 50,
			},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["234567890123"] = "ou-prod-12345678"

	policy := resolver.ResolvePolicy("234567890123")

	assert.Equal(t, "Production OU", policy.Name)
	assert.Equal(t, 15.0, policy.GrowthBuffer)
	assert.Equal(t, 50.0, policy.MinimumBudget)
	assert.Equal(t, 10.0, policy.RoundingIncrement) // Inherited
}

func TestResolvePolicy_TagPriority(t *testing.T) {
	config := types.PolicyConfig{
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Production Tag",
				GrowthBuffer:  12,
				MinimumBudget: 75,
			},
		},
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-dev-87654321",
				Name:          "Development OU",
				GrowthBuffer:  30,
				MinimumBudget: 10,
			},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["345678901234"] = "ou-dev-87654321"
	resolver.accountToTags["345678901234"] = map[string]string{
		"Environment": "production",
	}

	// Tag policy should take priority over OU policy
	policy := resolver.ResolvePolicy("345678901234")

	assert.Equal(t, "Production Tag", policy.Name)
	assert.Equal(t, 12.0, policy.GrowthBuffer)
	assert.Equal(t, 75.0, policy.MinimumBudget)
}

func TestResolvePolicy_DefaultFallback(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{},
		OUPolicies:      []types.OUPolicy{},
		TagPolicies:     []types.TagPolicy{},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	policy := resolver.ResolvePolicy("999999999999")

	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
	assert.Equal(t, 10.0, policy.MinimumBudget)
	assert.Equal(t, 10.0, policy.RoundingIncrement)
}

func TestResolvePolicy_PartialOverride(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{
				Account:      "123456789012",
				Name:         "Custom Growth",
				GrowthBuffer: 25,
				// MinimumBudget and RoundingIncrement not specified
			},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	policy := resolver.ResolvePolicy("123456789012")

	assert.Equal(t, "Custom Growth", policy.Name)
	assert.Equal(t, 25.0, policy.GrowthBuffer)      // Overridden
	assert.Equal(t, 10.0, policy.MinimumBudget)     // Inherited
	assert.Equal(t, 10.0, policy.RoundingIncrement) // Inherited
}

func TestMergePolicy(t *testing.T) {
	resolver := &Resolver{}

	base := types.RecommendationPolicy{
		Name:              "Base",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	// Test partial override
	merged := resolver.mergePolicy(base, "Override", 30, 0, 0)

	assert.Equal(t, "Override", merged.Name)
	assert.Equal(t, 30.0, merged.GrowthBuffer)
	assert.Equal(t, 10.0, merged.MinimumBudget)     // Kept from base
	assert.Equal(t, 10.0, merged.RoundingIncrement) // Kept from base
}

func TestResolvePolicy_MultipleTagsFirstMatch(t *testing.T) {
	config := types.PolicyConfig{
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Production",
				GrowthBuffer:  15,
				MinimumBudget: 50,
			},
			{
				TagKey:        "Team",
				TagValue:      "engineering",
				Name:          "Engineering",
				GrowthBuffer:  25,
				MinimumBudget: 20,
			},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToTags["123456789012"] = map[string]string{
		"Environment": "production",
		"Team":        "engineering",
	}

	// Should match first policy in list
	policy := resolver.ResolvePolicy("123456789012")

	assert.Equal(t, "Production", policy.Name)
	assert.Equal(t, 15.0, policy.GrowthBuffer)
}
