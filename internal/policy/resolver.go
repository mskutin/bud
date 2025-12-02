package policy

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/mskutin/bud/pkg/types"
)

// Resolver resolves which policy applies to an account
type Resolver struct {
	config        types.PolicyConfig
	defaultPolicy types.RecommendationPolicy
	accountToOU   map[string]string            // Cache: accountID -> ouID
	accountToTags map[string]map[string]string // Cache: accountID -> tags
}

// NewResolver creates a new policy resolver
func NewResolver(config types.PolicyConfig, defaultPolicy types.RecommendationPolicy) *Resolver {
	return &Resolver{
		config:        config,
		defaultPolicy: defaultPolicy,
		accountToOU:   make(map[string]string),
		accountToTags: make(map[string]map[string]string),
	}
}

// LoadAccountMetadata loads OU and tag information for accounts
func (r *Resolver) LoadAccountMetadata(ctx context.Context, cfg aws.Config, accounts []types.AccountInfo) error {
	client := organizations.NewFromConfig(cfg)

	for _, account := range accounts {
		// Get OU for account
		parentsInput := &organizations.ListParentsInput{
			ChildId: aws.String(account.ID),
		}

		parentsOutput, err := client.ListParents(ctx, parentsInput)
		if err != nil {
			// Non-fatal: continue without OU info
			continue
		}

		if len(parentsOutput.Parents) > 0 && parentsOutput.Parents[0].Id != nil {
			r.accountToOU[account.ID] = *parentsOutput.Parents[0].Id
		}

		// Get tags for account
		tagsInput := &organizations.ListTagsForResourceInput{
			ResourceId: aws.String(account.ID),
		}

		tagsOutput, err := client.ListTagsForResource(ctx, tagsInput)
		if err != nil {
			// Non-fatal: continue without tag info
			continue
		}

		tags := make(map[string]string)
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		r.accountToTags[account.ID] = tags
	}

	return nil
}

// ResolvePolicy determines which policy applies to an account
// Priority: Account > Tag > OU > Default
func (r *Resolver) ResolvePolicy(accountID string) types.RecommendationPolicy {
	// 1. Check account-specific policy
	for _, accountPolicy := range r.config.AccountPolicies {
		if accountPolicy.Account == accountID {
			return r.mergePolicy(r.defaultPolicy, accountPolicy.Name, accountPolicy.GrowthBuffer, accountPolicy.MinimumBudget, accountPolicy.RoundingIncrement)
		}
	}

	// 2. Check tag-based policy
	if tags, ok := r.accountToTags[accountID]; ok {
		for _, tagPolicy := range r.config.TagPolicies {
			if tagValue, exists := tags[tagPolicy.TagKey]; exists && tagValue == tagPolicy.TagValue {
				return r.mergePolicy(r.defaultPolicy, tagPolicy.Name, tagPolicy.GrowthBuffer, tagPolicy.MinimumBudget, tagPolicy.RoundingIncrement)
			}
		}
	}

	// 3. Check OU-based policy
	if ouID, ok := r.accountToOU[accountID]; ok {
		for _, ouPolicy := range r.config.OUPolicies {
			if ouPolicy.OU == ouID {
				return r.mergePolicy(r.defaultPolicy, ouPolicy.Name, ouPolicy.GrowthBuffer, ouPolicy.MinimumBudget, ouPolicy.RoundingIncrement)
			}
		}
	}

	// 4. Return default policy
	return r.defaultPolicy
}

// mergePolicy merges policy values with defaults (inheritance)
func (r *Resolver) mergePolicy(base types.RecommendationPolicy, name string, growthBuffer, minimumBudget, roundingIncrement float64) types.RecommendationPolicy {
	policy := base

	if name != "" {
		policy.Name = name
	}

	if growthBuffer > 0 {
		policy.GrowthBuffer = growthBuffer
	}

	if minimumBudget > 0 {
		policy.MinimumBudget = minimumBudget
	}

	if roundingIncrement > 0 {
		policy.RoundingIncrement = roundingIncrement
	}

	return policy
}

// ValidateOUs checks that all configured OUs exist
func ValidateOUs(ctx context.Context, cfg aws.Config, ouIDs []string) error {
	if len(ouIDs) == 0 {
		return nil
	}

	client := organizations.NewFromConfig(cfg)

	for _, ouID := range ouIDs {
		input := &organizations.DescribeOrganizationalUnitInput{
			OrganizationalUnitId: aws.String(ouID),
		}

		_, err := client.DescribeOrganizationalUnit(ctx, input)
		if err != nil {
			return fmt.Errorf("OU %s does not exist or is not accessible: %w", ouID, err)
		}
	}

	return nil
}
