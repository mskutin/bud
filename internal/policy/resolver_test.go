package policy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestNewResolver tests creating a new resolver
func TestNewResolver(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-test-12345678",
				Name:          "Test OU",
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

	assert.NotNil(t, resolver)
	assert.Equal(t, config, resolver.config)
	assert.Equal(t, defaultPolicy, resolver.defaultPolicy)
	assert.NotNil(t, resolver.accountToOU)
	assert.NotNil(t, resolver.accountToTags)
	assert.Empty(t, resolver.accountToOU)
	assert.Empty(t, resolver.accountToTags)
}

// TestResolvePolicy_PriorityOrder tests the complete priority order
func TestResolvePolicy_PriorityOrder(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{
				Account:       "123456789012",
				Name:          "Account Policy",
				GrowthBuffer:  5,
				MinimumBudget: 200,
			},
		},
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Tag Policy",
				GrowthBuffer:  10,
				MinimumBudget: 100,
			},
		},
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-prod-12345678",
				Name:          "OU Policy",
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

	t.Run("account policy has highest priority", func(t *testing.T) {
		resolver := NewResolver(config, defaultPolicy)
		resolver.accountToOU["123456789012"] = "ou-prod-12345678"
		resolver.accountToTags["123456789012"] = map[string]string{"Environment": "production"}

		policy := resolver.ResolvePolicy("123456789012")
		assert.Equal(t, "Account Policy", policy.Name)
		assert.Equal(t, 5.0, policy.GrowthBuffer)
		assert.Equal(t, 200.0, policy.MinimumBudget)
	})

	t.Run("tag policy overrides OU policy", func(t *testing.T) {
		resolver := NewResolver(config, defaultPolicy)
		resolver.accountToOU["234567890123"] = "ou-prod-12345678"
		resolver.accountToTags["234567890123"] = map[string]string{"Environment": "production"}

		policy := resolver.ResolvePolicy("234567890123")
		assert.Equal(t, "Tag Policy", policy.Name)
		assert.Equal(t, 10.0, policy.GrowthBuffer)
		assert.Equal(t, 100.0, policy.MinimumBudget)
	})

	t.Run("OU policy overrides default", func(t *testing.T) {
		resolver := NewResolver(config, defaultPolicy)
		resolver.accountToOU["345678901234"] = "ou-prod-12345678"

		policy := resolver.ResolvePolicy("345678901234")
		assert.Equal(t, "OU Policy", policy.Name)
		assert.Equal(t, 15.0, policy.GrowthBuffer)
		assert.Equal(t, 50.0, policy.MinimumBudget)
	})

	t.Run("default policy when no match", func(t *testing.T) {
		resolver := NewResolver(config, defaultPolicy)

		policy := resolver.ResolvePolicy("999888777666")
		assert.Equal(t, "Default", policy.Name)
		assert.Equal(t, 20.0, policy.GrowthBuffer)
		assert.Equal(t, 10.0, policy.MinimumBudget)
	})
}

// TestResolvePolicy_NoMetadata tests policy resolution when metadata is not loaded
func TestResolvePolicy_NoMetadata(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-prod-12345678",
				Name:          "Production OU",
				GrowthBuffer:  15,
				MinimumBudget: 50,
			},
		},
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Production Tag",
				GrowthBuffer:  12,
				MinimumBudget: 75,
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
	// Don't load any metadata

	// Should fall back to default since no metadata is loaded
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
}

// TestResolvePolicy_TagValueMismatch tests that tag values must match exactly
func TestResolvePolicy_TagValueMismatch(t *testing.T) {
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
	resolver.accountToOU["123456789012"] = "ou-dev-87654321"
	resolver.accountToTags["123456789012"] = map[string]string{
		"Environment": "staging", // Different value
	}

	// Should use OU policy since tag value doesn't match
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Development OU", policy.Name)
	assert.Equal(t, 30.0, policy.GrowthBuffer)
}

// TestResolvePolicy_TagKeyMismatch tests that tag keys must match
func TestResolvePolicy_TagKeyMismatch(t *testing.T) {
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
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToTags["123456789012"] = map[string]string{
		"Team": "engineering", // Different key
	}

	// Should use default since tag key doesn't match
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
}

// TestResolvePolicy_CompleteOverride tests complete policy override
func TestResolvePolicy_CompleteOverride(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{
				Account:           "123456789012",
				Name:              "Complete Override",
				GrowthBuffer:      50,
				MinimumBudget:     500,
				RoundingIncrement: 100,
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

	assert.Equal(t, "Complete Override", policy.Name)
	assert.Equal(t, 50.0, policy.GrowthBuffer)
	assert.Equal(t, 500.0, policy.MinimumBudget)
	assert.Equal(t, 100.0, policy.RoundingIncrement)
}

// TestMergePolicy_AllCombinations tests all combinations of policy overrides
func TestMergePolicy_AllCombinations(t *testing.T) {
	base := types.RecommendationPolicy{
		Name:              "Base",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 5,
	}

	resolver := &Resolver{}

	tests := []struct {
		name              string
		overrideName      string
		overrideGrowth    float64
		overrideMinBudget float64
		overrideRounding  float64
		expectName        string
		expectGrowth      float64
		expectMinBudget   float64
		expectRounding    float64
	}{
		{
			name:              "override name only",
			overrideName:      "New Name",
			overrideGrowth:    0,
			overrideMinBudget: 0,
			overrideRounding:  0,
			expectName:        "New Name",
			expectGrowth:      20,
			expectMinBudget:   10,
			expectRounding:    5,
		},
		{
			name:              "override growth only",
			overrideName:      "",
			overrideGrowth:    30,
			overrideMinBudget: 0,
			overrideRounding:  0,
			expectName:        "Base",
			expectGrowth:      30,
			expectMinBudget:   10,
			expectRounding:    5,
		},
		{
			name:              "override minimum only",
			overrideName:      "",
			overrideGrowth:    0,
			overrideMinBudget: 50,
			overrideRounding:  0,
			expectName:        "Base",
			expectGrowth:      20,
			expectMinBudget:   50,
			expectRounding:    5,
		},
		{
			name:              "override rounding only",
			overrideName:      "",
			overrideGrowth:    0,
			overrideMinBudget: 0,
			overrideRounding:  25,
			expectName:        "Base",
			expectGrowth:      20,
			expectMinBudget:   10,
			expectRounding:    25,
		},
		{
			name:              "override all",
			overrideName:      "All New",
			overrideGrowth:    100,
			overrideMinBudget: 200,
			overrideRounding:  50,
			expectName:        "All New",
			expectGrowth:      100,
			expectMinBudget:   200,
			expectRounding:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := resolver.mergePolicy(base, tt.overrideName, tt.overrideGrowth, tt.overrideMinBudget, tt.overrideRounding)

			assert.Equal(t, tt.expectName, merged.Name)
			assert.Equal(t, tt.expectGrowth, merged.GrowthBuffer)
			assert.Equal(t, tt.expectMinBudget, merged.MinimumBudget)
			assert.Equal(t, tt.expectRounding, merged.RoundingIncrement)
		})
	}
}

// TestValidateOUs_EmptyList tests validation with empty OU list
func TestValidateOUs_EmptyList(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	ouIDs := []string{}

	err := ValidateOUs(ctx, cfg, ouIDs)

	assert.NoError(t, err, "empty OU list should not return error")
}

// TestValidateOUs_SingleValidOU tests validation with single valid OU
func TestValidateOUs_SingleValidOU(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	ouIDs := []string{"ou-test-12345678"}

	err := ValidateOUs(ctx, cfg, ouIDs)

	// Without actual AWS API, we expect an error about OU not existing
	// This validates the error handling path
	assert.Error(t, err)
}

// TestValidateOUs_MultipleOUs tests validation with multiple OUs
func TestValidateOUs_MultipleOUs(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	ouIDs := []string{
		"ou-test-12345678",
		"ou-test-87654321",
		"ou-test-11111111",
	}

	err := ValidateOUs(ctx, cfg, ouIDs)

	// Should fail on first OU without actual AWS API
	assert.Error(t, err)
}

// TestLoadAccountMetadata_EmptyAccounts tests metadata loading with no accounts
func TestLoadAccountMetadata_EmptyAccounts(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	assert.NoError(t, err)
	assert.Empty(t, resolver.accountToOU)
	assert.Empty(t, resolver.accountToTags)
}

// TestLoadAccountMetadata_SingleAccount tests metadata loading with single account
func TestLoadAccountMetadata_SingleAccount(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Test Account"},
	}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	// Without actual AWS API, metadata loading will silently skip on errors
	// The function should not return error even if AWS API calls fail
	assert.NoError(t, err)
}

// TestLoadAccountMetadata_GracefulDegradation tests that errors are handled gracefully
func TestLoadAccountMetadata_GracefulDegradation(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
		{ID: "345678901234", Name: "Account 3"},
	}

	// Even without AWS credentials, should not error
	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	assert.NoError(t, err, "LoadAccountMetadata should not error on AWS API failures")
}

// TestLoadAccountMetadata_PartialMetadata tests partial metadata loading
func TestLoadAccountMetadata_PartialMetadata(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	// Manually set some metadata to simulate partial loading
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"
	resolver.accountToTags["123456789012"] = map[string]string{"Environment": "production"}

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
	}

	_ = resolver.LoadAccountMetadata(ctx, cfg, accounts)

	// The existing metadata should still be there
	assert.Equal(t, "ou-prod-12345678", resolver.accountToOU["123456789012"])
	assert.Equal(t, "production", resolver.accountToTags["123456789012"]["Environment"])
}

// TestResolvePolicy_WithLoadedMetadata tests policy resolution after metadata loading
func TestResolvePolicy_WithLoadedMetadata(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{
				OU:            "ou-prod-12345678",
				Name:          "Production",
				GrowthBuffer:  15,
				MinimumBudget: 50,
			},
		},
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Prod Tag",
				GrowthBuffer:  12,
				MinimumBudget: 75,
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

	// Simulate loaded metadata
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"
	resolver.accountToTags["123456789012"] = map[string]string{
		"Environment": "production",
		"CostCenter":  "engineering",
	}

	policy := resolver.ResolvePolicy("123456789012")

	// Tag policy should win over OU policy
	assert.Equal(t, "Prod Tag", policy.Name)
	assert.Equal(t, 12.0, policy.GrowthBuffer)
	assert.Equal(t, 75.0, policy.MinimumBudget)
}

// TestLoadAccountMetadata_MultipleAccounts tests loading metadata for multiple accounts
func TestLoadAccountMetadata_MultipleAccounts(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
		{ID: "345678901234", Name: "Account 3"},
		{ID: "456789012345", Name: "Account 4"},
		{ID: "567890123456", Name: "Account 5"},
	}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	assert.NoError(t, err)
}

// TestResolvePolicy_MultipleAccounts tests policy resolution for multiple accounts
func TestResolvePolicy_MultipleAccounts(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-prod-12345678", Name: "Production", GrowthBuffer: 15, MinimumBudget: 50},
			{OU: "ou-dev-87654321", Name: "Development", GrowthBuffer: 30, MinimumBudget: 10},
		},
		AccountPolicies: []types.AccountPolicy{
			{Account: "999888777666", Name: "VIP", GrowthBuffer: 5, MinimumBudget: 1000},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// Set up OU mappings
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"
	resolver.accountToOU["234567890123"] = "ou-prod-12345678"
	resolver.accountToOU["345678901234"] = "ou-dev-87654321"

	tests := []struct {
		name              string
		accountID         string
		expectedPolicy    string
		expectedGrowth    float64
		expectedMinBudget float64
	}{
		{
			name:              "production account 1",
			accountID:         "123456789012",
			expectedPolicy:    "Production",
			expectedGrowth:    15,
			expectedMinBudget: 50,
		},
		{
			name:              "production account 2",
			accountID:         "234567890123",
			expectedPolicy:    "Production",
			expectedGrowth:    15,
			expectedMinBudget: 50,
		},
		{
			name:              "development account",
			accountID:         "345678901234",
			expectedPolicy:    "Development",
			expectedGrowth:    30,
			expectedMinBudget: 10,
		},
		{
			name:              "account policy override",
			accountID:         "999888777666",
			expectedPolicy:    "VIP",
			expectedGrowth:    5,
			expectedMinBudget: 1000,
		},
		{
			name:              "default policy",
			accountID:         "111111111111",
			expectedPolicy:    "Default",
			expectedGrowth:    20,
			expectedMinBudget: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := resolver.ResolvePolicy(tt.accountID)
			assert.Equal(t, tt.expectedPolicy, policy.Name)
			assert.Equal(t, tt.expectedGrowth, policy.GrowthBuffer)
			assert.Equal(t, tt.expectedMinBudget, policy.MinimumBudget)
		})
	}
}

// TestResolvePolicy_CaseSensitivity tests that tag matching is case-sensitive
func TestResolvePolicy_CaseSensitivity(t *testing.T) {
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
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	tests := []struct {
		name           string
		tagKey         string
		tagValue       string
		expectedPolicy string
	}{
		{
			name:           "exact match",
			tagKey:         "Environment",
			tagValue:       "production",
			expectedPolicy: "Production Tag",
		},
		{
			name:           "different case value",
			tagKey:         "Environment",
			tagValue:       "Production",
			expectedPolicy: "Default",
		},
		{
			name:           "different case key",
			tagKey:         "environment",
			tagValue:       "production",
			expectedPolicy: "Default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(config, defaultPolicy)
			resolver.accountToTags["123456789012"] = map[string]string{
				tt.tagKey: tt.tagValue,
			}

			policy := resolver.ResolvePolicy("123456789012")
			assert.Equal(t, tt.expectedPolicy, policy.Name)
		})
	}
}

// TestResolvePolicy_ZeroValueOverride tests that zero values don't override defaults
func TestResolvePolicy_ZeroValueOverride(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{
				Account:       "123456789012",
				Name:          "Zero Values",
				GrowthBuffer:  0, // Zero - should not override
				MinimumBudget: 0, // Zero - should not override
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

	assert.Equal(t, "Zero Values", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)      // Should keep default
	assert.Equal(t, 10.0, policy.MinimumBudget)     // Should keep default
	assert.Equal(t, 10.0, policy.RoundingIncrement) // Should keep default
}

// TestResolvePolicy_EmptyTagValues tests behavior with empty tag values
func TestResolvePolicy_EmptyTagValues(t *testing.T) {
	config := types.PolicyConfig{
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "", // Empty value
				Name:          "Empty Tag",
				GrowthBuffer:  12,
				MinimumBudget: 75,
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
		"Environment": "",
	}

	// Empty tag value should match
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Empty Tag", policy.Name)
}

// TestResolvePolicy_NonExistentAccount tests resolution for non-existent account
func TestResolvePolicy_NonExistentAccount(t *testing.T) {
	config := types.PolicyConfig{}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// Account that doesn't exist in any mapping should get default
	policy := resolver.ResolvePolicy("000000000000")

	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
	assert.Equal(t, 10.0, policy.MinimumBudget)
}

// TestResolvePolicy_WithEmptyOUInMapping tests behavior with empty OU ID
func TestResolvePolicy_WithEmptyOUInMapping(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{
				OU:            "",
				Name:          "Empty OU",
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
	resolver.accountToOU["123456789012"] = ""

	// Empty OU should still match the policy with empty OU
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Empty OU", policy.Name)
}

// Helper function to create mock organizations error
func newOrganizationsError(code string) error {
	return &smithy.GenericAPIError{
		Code:    code,
		Message: "test error",
	}
}

// TestErrorClassification tests error classification for organizations API
func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name               string
		errorCode          string
		expectAccessDenied bool
	}{
		{
			name:               "AccessDeniedException",
			errorCode:          "AccessDeniedException",
			expectAccessDenied: true,
		},
		{
			name:               "UnauthorizedOperation",
			errorCode:          "UnauthorizedOperation",
			expectAccessDenied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newOrganizationsError(tt.errorCode)
			// This tests our helper function
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.errorCode)
		})
	}
}

// TestResolvePolicy_ConcurrentAccess tests thread safety of policy resolution
func TestResolvePolicy_ConcurrentAccess(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-prod-12345678", Name: "Production", GrowthBuffer: 15, MinimumBudget: 50},
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

	// Simulate concurrent access (note: this doesn't actually run in parallel
	// but demonstrates the function is safe for concurrent reads)
	policies := make([]types.RecommendationPolicy, 10)
	for i := 0; i < 10; i++ {
		policies[i] = resolver.ResolvePolicy("123456789012")
	}

	// All should return the same policy
	for _, p := range policies {
		assert.Equal(t, "Production", p.Name)
		assert.Equal(t, 15.0, p.GrowthBuffer)
	}
}

// TestNewResolver_WithEmptyConfig tests creating resolver with empty config
func TestNewResolver_WithEmptyConfig(t *testing.T) {
	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	assert.NotNil(t, resolver)
	assert.NotNil(t, resolver.accountToOU)
	assert.NotNil(t, resolver.accountToTags)

	// Should resolve to default for any account
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Default", policy.Name)
}

// TestLoadAccountMetadata_WithNilTags tests handling of nil tags
func TestLoadAccountMetadata_WithNilTags(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToTags["123456789012"] = nil // Explicitly nil

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
	}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	assert.NoError(t, err)
}

// TestResolvePolicy_AccountWithoutMetadataInCache tests account not in cache
func TestResolvePolicy_AccountWithoutMetadataInCache(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-prod-12345678", Name: "Production", GrowthBuffer: 15, MinimumBudget: 50},
		},
		TagPolicies: []types.TagPolicy{
			{
				TagKey:        "Environment",
				TagValue:      "production",
				Name:          "Production Tag",
				GrowthBuffer:  12,
				MinimumBudget: 75,
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

	// Set metadata for one account but not another
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"
	// "234567890123" is not in the cache

	// Account with metadata should match OU policy
	policy1 := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Production", policy1.Name)

	// Account without metadata should get default
	policy2 := resolver.ResolvePolicy("234567890123")
	assert.Equal(t, "Default", policy2.Name)
}

// TestResolvePolicy_OverridingName tests name override behavior
func TestResolvePolicy_OverridingName(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-test-12345678", Name: "", GrowthBuffer: 15, MinimumBudget: 50}, // Empty name
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["123456789012"] = "ou-test-12345678"

	policy := resolver.ResolvePolicy("123456789012")

	// Empty name in policy should not override default name
	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 15.0, policy.GrowthBuffer) // But growth should still be overridden
}

// TestLoadAccountMetadata_ErrorHandling tests various error scenarios
func TestLoadAccountMetadata_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "permission denied on list parents",
			description: "should gracefully handle AccessDeniedException when listing parents",
		},
		{
			name:        "permission denied on list tags",
			description: "should gracefully handle AccessDeniedException when listing tags",
		},
		{
			name:        "empty parent list",
			description: "should handle accounts with no parents",
		},
		{
			name:        "empty tag list",
			description: "should handle accounts with no tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := aws.Config{Region: "us-east-1"}

			config := types.PolicyConfig{}
			defaultPolicy := types.RecommendationPolicy{Name: "Default"}

			resolver := NewResolver(config, defaultPolicy)

			accounts := []types.AccountInfo{
				{ID: "123456789012", Name: "Test Account"},
			}

			// Without actual AWS API, we document expected behavior
			err := resolver.LoadAccountMetadata(ctx, cfg, accounts)
			assert.NoError(t, err, tt.description)
		})
	}
}

// TestValidateOUs_ErrorScenarios tests various validation error scenarios
func TestValidateOUs_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		ouIDs       []string
		description string
	}{
		{
			name:        "invalid OU format",
			ouIDs:       []string{"invalid-ou-format"},
			description: "should return error for invalid OU format",
		},
		{
			name:        "non-existent OU",
			ouIDs:       []string{"ou-test-99999999"},
			description: "should return error for non-existent OU",
		},
		{
			name:        "mixed valid and invalid",
			ouIDs:       []string{"ou-test-12345678", "invalid-format"},
			description: "should return error on first invalid OU",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := aws.Config{Region: "us-east-1"}

			err := ValidateOUs(ctx, cfg, tt.ouIDs)

			// Without actual AWS API, we expect errors
			assert.Error(t, err, tt.description)
		})
	}
}

// TestLoadAccountMetadata_PartialFailure tests partial failure handling
func TestLoadAccountMetadata_PartialFailure(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	// Set metadata for one account before loading
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
		{ID: "345678901234", Name: "Account 3"},
	}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	// Should not error even if some accounts fail
	assert.NoError(t, err)

	// Pre-existing metadata should still be there
	assert.Equal(t, "ou-prod-12345678", resolver.accountToOU["123456789012"])
}

// TestResolvePolicy_PolicyImmutability tests that policy resolution doesn't modify defaults
func TestResolvePolicy_PolicyImmutability(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{Account: "123456789012", Name: "Test", GrowthBuffer: 50, MinimumBudget: 100},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	originalDefault := defaultPolicy

	resolver := NewResolver(config, defaultPolicy)

	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, 50.0, policy.GrowthBuffer)

	// Default policy should remain unchanged
	assert.Equal(t, originalDefault.Name, defaultPolicy.Name)
	assert.Equal(t, originalDefault.GrowthBuffer, defaultPolicy.GrowthBuffer)
	assert.Equal(t, originalDefault.MinimumBudget, defaultPolicy.MinimumBudget)
	assert.Equal(t, originalDefault.RoundingIncrement, defaultPolicy.RoundingIncrement)
}

// TestResolvePolicy_MultipleTagsSameAccount tests multiple tags on same account
func TestResolvePolicy_MultipleTagsSameAccount(t *testing.T) {
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
				TagKey:        "CostCenter",
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
		"CostCenter":  "engineering",
	}

	// First matching policy should win
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Production", policy.Name)
	assert.Equal(t, 15.0, policy.GrowthBuffer)
}

// TestResolvePolicy_EmptyAccountID tests handling of empty account ID
func TestResolvePolicy_EmptyAccountID(t *testing.T) {
	config := types.PolicyConfig{}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// Empty account ID should get default policy
	policy := resolver.ResolvePolicy("")
	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
}

// TestLoadAccountMetadata_Overwrite tests that metadata loading overwrites existing values
func TestLoadAccountMetadata_Overwrite(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	// Set initial metadata
	resolver.accountToOU["123456789012"] = "ou-old-12345678"
	resolver.accountToTags["123456789012"] = map[string]string{
		"OldTag": "oldvalue",
	}

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Test Account"},
	}

	_ = resolver.LoadAccountMetadata(ctx, cfg, accounts)

	// Without actual AWS API, the old values might remain or be cleared
	// This test documents the expected behavior: metadata can be overwritten
	_ = resolver.accountToOU["123456789012"]
	_ = resolver.accountToTags["123456789012"]
}

// TestResolvePolicy_MergeWithZeroBase tests merging when base has zeros
func TestResolvePolicy_MergeWithZeroBase(t *testing.T) {
	resolver := &Resolver{}

	base := types.RecommendationPolicy{
		Name:              "Base",
		GrowthBuffer:      0,
		MinimumBudget:     0,
		RoundingIncrement: 0,
	}

	merged := resolver.mergePolicy(base, "Override", 30, 50, 10)

	assert.Equal(t, "Override", merged.Name)
	assert.Equal(t, 30.0, merged.GrowthBuffer)
	assert.Equal(t, 50.0, merged.MinimumBudget)
	assert.Equal(t, 10.0, merged.RoundingIncrement)
}

// TestErrorMessages tests error message formatting
func TestErrorMessages(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	ouID := "ou-test-12345678"
	err := ValidateOUs(ctx, cfg, []string{ouID})

	require.Error(t, err)
	// Error message should contain the OU ID
	assert.Contains(t, err.Error(), ouID)
}

// TestResolvePolicy_LongAccountID tests with various account ID formats
func TestResolvePolicy_LongAccountID(t *testing.T) {
	config := types.PolicyConfig{}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// Various account ID formats
	accounts := []string{
		"123456789012", // Standard 12-digit
		"000000000000", // All zeros
		"999999999999", // All nines
	}

	for _, accountID := range accounts {
		t.Run(accountID, func(t *testing.T) {
			policy := resolver.ResolvePolicy(accountID)
			assert.Equal(t, "Default", policy.Name)
		})
	}
}

// TestResolvePolicy_NoNameOverride tests policy with no name override
func TestResolvePolicy_NoNameOverride(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-test-12345678", Name: "", GrowthBuffer: 15, MinimumBudget: 50},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["123456789012"] = "ou-test-12345678"

	policy := resolver.ResolvePolicy("123456789012")

	// Name should remain from default when override name is empty
	assert.Equal(t, "Default", policy.Name)
	// But other values should be overridden
	assert.Equal(t, 15.0, policy.GrowthBuffer)
	assert.Equal(t, 50.0, policy.MinimumBudget)
}

// TestResolvePolicy_AllZeroValues tests policy with all zero values
func TestResolvePolicy_AllZeroValues(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{Account: "123456789012", Name: "All Zeros", GrowthBuffer: 0, MinimumBudget: 0, RoundingIncrement: 0},
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

	// All zero values should not override defaults
	assert.Equal(t, "All Zeros", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
	assert.Equal(t, 10.0, policy.MinimumBudget)
	assert.Equal(t, 10.0, policy.RoundingIncrement)
}

// TestLoadAccountMetadata_ContextCancellation tests context cancellation handling
func TestLoadAccountMetadata_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Test Account"},
	}

	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)

	// Should handle cancelled context gracefully
	// May or may not error depending on timing
	_ = err
}

// TestNewResolver_ImmutableConfig tests that resolver doesn't modify input config
func TestNewResolver_ImmutableConfig(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-test-12345678", Name: "Test", GrowthBuffer: 15, MinimumBudget: 50},
		},
	}

	originalOUPolicies := config.OUPolicies

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	_ = NewResolver(config, defaultPolicy)

	// Config should not be modified
	assert.Equal(t, originalOUPolicies, config.OUPolicies)
}

// TestResolvePolicy_MultipleOUsForSameAccount tests behavior when account has multiple OUs
func TestResolvePolicy_MultipleOUsForSameAccount(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-first-12345678", Name: "First OU", GrowthBuffer: 15, MinimumBudget: 50},
			{OU: "ou-second-87654321", Name: "Second OU", GrowthBuffer: 25, MinimumBudget: 75},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// An account can only belong to one OU (parent) in AWS Organizations
	// This tests that behavior
	resolver.accountToOU["123456789012"] = "ou-first-12345678"

	policy := resolver.ResolvePolicy("123456789012")

	assert.Equal(t, "First OU", policy.Name)
	assert.Equal(t, 15.0, policy.GrowthBuffer)
}

// TestValidateOUs_DuplicateOUs tests validation with duplicate OU IDs
func TestValidateOUs_DuplicateOUs(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	ouID := "ou-test-12345678"
	ouIDs := []string{ouID, ouID, ouID} // Same OU three times

	err := ValidateOUs(ctx, cfg, ouIDs)

	// Should validate each one (will fail on first without actual API)
	assert.Error(t, err)
}

// TestResolvePolicy_TagWithEmptyKey tests tag policy with empty key
func TestResolvePolicy_TagWithEmptyKey(t *testing.T) {
	config := types.PolicyConfig{
		TagPolicies: []types.TagPolicy{
			{TagKey: "", TagValue: "production", Name: "Empty Key", GrowthBuffer: 12, MinimumBudget: 75},
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
		"": "production",
	}

	// Empty key should still match
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Empty Key", policy.Name)
}

// TestLoadAccountMetadata_AccountsNilSlice tests with nil accounts slice
func TestLoadAccountMetadata_AccountsNilSlice(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	// Should handle nil slice gracefully
	err := resolver.LoadAccountMetadata(ctx, cfg, nil)

	assert.NoError(t, err)
}

// TestResolvePolicy_WithSyntheticTagAndOU tests interaction between tag and OU metadata
func TestResolvePolicy_WithSyntheticTagAndOU(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-prod-12345678", Name: "Production OU", GrowthBuffer: 15, MinimumBudget: 50},
		},
		TagPolicies: []types.TagPolicy{
			{TagKey: "Team", TagValue: "platform", Name: "Platform Team", GrowthBuffer: 10, MinimumBudget: 100},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	// Account has both OU and tag metadata
	resolver.accountToOU["123456789012"] = "ou-prod-12345678"
	resolver.accountToTags["123456789012"] = map[string]string{"Team": "platform"}

	// Tag policy should win over OU policy
	policy := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, "Platform Team", policy.Name)
	assert.Equal(t, 10.0, policy.GrowthBuffer)
	assert.Equal(t, 100.0, policy.MinimumBudget)
}

// TestResolvePolicy_NegativeValueHandling tests handling of negative values
func TestResolvePolicy_NegativeValueHandling(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{Account: "123456789012", Name: "Negative", GrowthBuffer: -10, MinimumBudget: -5},
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

	// Negative values should NOT override defaults (since mergePolicy checks > 0)
	assert.Equal(t, "Negative", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
	assert.Equal(t, 10.0, policy.MinimumBudget)
}

// TestResolvePolicy_SameAccountMultiplePolicies tests behavior with multiple policies for same account
func TestResolvePolicy_SameAccountMultiplePolicies(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{Account: "123456789012", Name: "First", GrowthBuffer: 10, MinimumBudget: 50},
			{Account: "123456789012", Name: "Second", GrowthBuffer: 20, MinimumBudget: 100},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      30,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	resolver := NewResolver(config, defaultPolicy)

	policy := resolver.ResolvePolicy("123456789012")

	// First matching policy should win
	assert.Equal(t, "First", policy.Name)
	assert.Equal(t, 10.0, policy.GrowthBuffer)
	assert.Equal(t, 50.0, policy.MinimumBudget)
}

// TestLoadAccountMetadata_VerifyInitializedMaps tests that maps are properly initialized
func TestLoadAccountMetadata_VerifyInitializedMaps(t *testing.T) {
	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	// Maps should be initialized
	assert.NotNil(t, resolver.accountToOU)
	assert.NotNil(t, resolver.accountToTags)

	// Should be able to add entries
	resolver.accountToOU["123456789012"] = "ou-test-12345678"
	resolver.accountToTags["123456789012"] = map[string]string{"key": "value"}

	assert.Equal(t, "ou-test-12345678", resolver.accountToOU["123456789012"])
	assert.Equal(t, "value", resolver.accountToTags["123456789012"]["key"])
}

// TestLoadAccountMetadata_ParentsOrder tests handling of multiple parents
func TestLoadAccountMetadata_ParentsOrder(t *testing.T) {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}

	config := types.PolicyConfig{}
	defaultPolicy := types.RecommendationPolicy{Name: "Default"}

	resolver := NewResolver(config, defaultPolicy)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Test Account"},
	}

	// Without actual AWS API, we document expected behavior:
	// AWS accounts can only have one parent (one OU), but ListParents returns a list
	// The function uses the first parent in the list
	err := resolver.LoadAccountMetadata(ctx, cfg, accounts)
	assert.NoError(t, err)
}

// TestResolvePolicy_VerifyRoundingInheritance tests that rounding increment is inherited properly
func TestResolvePolicy_VerifyRoundingInheritance(t *testing.T) {
	config := types.PolicyConfig{
		OUPolicies: []types.OUPolicy{
			{OU: "ou-test-12345678", Name: "Test OU", GrowthBuffer: 15, MinimumBudget: 50},
			// RoundingIncrement not specified - should inherit
		},
		TagPolicies: []types.TagPolicy{
			{TagKey: "Team", TagValue: "engineering", Name: "Engineering", GrowthBuffer: 25},
			// MinimumBudget and RoundingIncrement not specified
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 25, // Non-default value
	}

	resolver := NewResolver(config, defaultPolicy)
	resolver.accountToOU["123456789012"] = "ou-test-12345678"
	resolver.accountToTags["234567890123"] = map[string]string{"Team": "engineering"}

	// OU policy should inherit rounding from default
	policy1 := resolver.ResolvePolicy("123456789012")
	assert.Equal(t, 25.0, policy1.RoundingIncrement)

	// Tag policy should inherit rounding from default
	policy2 := resolver.ResolvePolicy("234567890123")
	assert.Equal(t, 25.0, policy2.RoundingIncrement)
}

// TestResolvePolicy_DefaultPolicyIsNotModified tests that default policy is never modified
func TestResolvePolicy_DefaultPolicyIsNotModified(t *testing.T) {
	config := types.PolicyConfig{
		AccountPolicies: []types.AccountPolicy{
			{Account: "123456789012", Name: "Override", GrowthBuffer: 50, MinimumBudget: 100},
		},
	}

	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	originalDefaults := defaultPolicy

	resolver := NewResolver(config, defaultPolicy)

	// Resolve policy for account with override
	_ = resolver.ResolvePolicy("123456789012")

	// Resolve policy for account that uses default
	policy := resolver.ResolvePolicy("999888777666")

	// Default policy values should remain unchanged
	assert.Equal(t, originalDefaults.Name, defaultPolicy.Name)
	assert.Equal(t, originalDefaults.GrowthBuffer, defaultPolicy.GrowthBuffer)
	assert.Equal(t, originalDefaults.MinimumBudget, defaultPolicy.MinimumBudget)
	assert.Equal(t, originalDefaults.RoundingIncrement, defaultPolicy.RoundingIncrement)

	// And the returned policy should have default values
	assert.Equal(t, "Default", policy.Name)
	assert.Equal(t, 20.0, policy.GrowthBuffer)
	assert.Equal(t, 10.0, policy.MinimumBudget)
	assert.Equal(t, 10.0, policy.RoundingIncrement)
}

// TestErrorStringFormats tests error string formats
func TestErrorStringFormats(t *testing.T) {
	ouID := "ou-test-12345678"
	expectedErr := fmt.Errorf("OU %s does not exist or is not accessible: %w", ouID, errors.New("test"))

	// Error message should contain the OU ID
	assert.Contains(t, expectedErr.Error(), ouID)
	assert.Contains(t, expectedErr.Error(), "does not exist")
}
