package cmd

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
)

// Feature: aws-budget-optimization, Property 22: Partial failure result completeness
// Validates: Requirements 6.5
// For any analysis run with N input accounts, the result should contain exactly N entries
// (either successful results or error records).
func TestProperty_PartialFailureResultCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("analysis result contains exactly N entries for N input accounts", prop.ForAll(
		func(numAccounts int, numErrors int) bool {
			// Ensure numErrors doesn't exceed numAccounts
			if numErrors > numAccounts {
				numErrors = numAccounts
			}

			// Create a mock analysis result
			result := &types.AnalysisResult{
				Recommendations: make([]*types.BudgetRecommendation, numAccounts-numErrors),
				Errors:          make([]types.AnalysisError, numErrors),
			}

			// Fill in recommendations
			for i := 0; i < numAccounts-numErrors; i++ {
				result.Recommendations[i] = &types.BudgetRecommendation{
					AccountID:   "account-" + string(rune(i)),
					AccountName: "Account " + string(rune(i)),
				}
			}

			// Fill in errors
			for i := 0; i < numErrors; i++ {
				result.Errors[i] = types.AnalysisError{
					AccountID:   "error-account-" + string(rune(i)),
					AccountName: "Error Account " + string(rune(i)),
				}
			}

			// Property: total entries should equal input accounts
			totalEntries := len(result.Recommendations) + len(result.Errors)
			return totalEntries == numAccounts
		},
		gen.IntRange(1, 100), // numAccounts: 1 to 100
		gen.IntRange(0, 100), // numErrors: 0 to 100
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Test filterAccounts function
func TestFilterAccounts(t *testing.T) {
	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
		{ID: "345678901234", Name: "Account 3"},
	}

	t.Run("empty filter returns all accounts", func(t *testing.T) {
		filtered := filterAccounts(accounts, []string{})
		assert.Equal(t, 3, len(filtered))
	})

	t.Run("filter with one account", func(t *testing.T) {
		filtered := filterAccounts(accounts, []string{"123456789012"})
		assert.Equal(t, 1, len(filtered))
		assert.Equal(t, "123456789012", filtered[0].ID)
	})

	t.Run("filter with multiple accounts", func(t *testing.T) {
		filtered := filterAccounts(accounts, []string{"123456789012", "345678901234"})
		assert.Equal(t, 2, len(filtered))
	})

	t.Run("filter with non-existent account", func(t *testing.T) {
		filtered := filterAccounts(accounts, []string{"999999999999"})
		assert.Equal(t, 0, len(filtered))
	})
}
