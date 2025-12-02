package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/mskutin/bud/internal/analyzer"
	"github.com/mskutin/bud/internal/recommender"
	"github.com/mskutin/bud/internal/reporter"
	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndWorkflow tests the complete analysis workflow with mock data
func TestEndToEndWorkflow(t *testing.T) {
	_ = context.Background()

	// Create sample accounts (for reference, not used in this test)
	_ = []types.AccountInfo{
		{ID: "123456789012", Name: "Test Account 1", Email: "test1@example.com"},
		{ID: "234567890123", Name: "Test Account 2", Email: "test2@example.com"},
		{ID: "345678901234", Name: "Test Account 3", Email: "test3@example.com"},
	}

	// Create mock cost data
	costData := []*types.AccountCostData{
		{
			AccountID:   "123456789012",
			AccountName: "Test Account 1",
			MonthlyCosts: []types.MonthlyCost{
				{Month: "2024-09", Amount: 450.00},
				{Month: "2024-10", Amount: 520.00},
				{Month: "2024-11", Amount: 580.00},
			},
		},
		{
			AccountID:   "234567890123",
			AccountName: "Test Account 2",
			MonthlyCosts: []types.MonthlyCost{
				{Month: "2024-09", Amount: 150.00},
				{Month: "2024-10", Amount: 160.00},
				{Month: "2024-11", Amount: 155.00},
			},
		},
		{
			AccountID:   "345678901234",
			AccountName: "Test Account 3",
			MonthlyCosts: []types.MonthlyCost{
				{Month: "2024-09", Amount: 25.00},
				{Month: "2024-10", Amount: 30.00},
				{Month: "2024-11", Amount: 28.00},
			},
		},
	}

	// Create mock budget data
	budgetData := map[string][]*types.BudgetConfig{
		"123456789012": {
			{
				AccountID:     "123456789012",
				AccountName:   "Test Account 1",
				BudgetName:    "monthly-budget",
				LimitAmount:   430.00,
				TimeUnit:      "MONTHLY",
				HasForecasted: true,
				HasActual:     true,
				Subscribers:   []string{"test1@example.com"},
			},
		},
		"234567890123": {
			{
				AccountID:     "234567890123",
				AccountName:   "Test Account 2",
				BudgetName:    "monthly-budget",
				LimitAmount:   340.00,
				TimeUnit:      "MONTHLY",
				HasForecasted: true,
				HasActual:     true,
				Subscribers:   []string{"test2@example.com"},
			},
		},
		// Account 3 has no budget
	}

	// Initialize components
	analyzer := &analyzer.Analyzer{}
	recommender := recommender.NewRecommender(types.RecommendationPolicy{
		GrowthBuffer:      20.0,
		MinimumBudget:     10.0,
		RoundingIncrement: 10.0,
	})

	// Build analysis result
	result := &types.AnalysisResult{
		Timestamp: time.Now(),
		Config: types.AnalysisConfig{
			AnalysisMonths:    3,
			GrowthBuffer:      20.0,
			MinimumBudget:     10.0,
			RoundingIncrement: 10.0,
		},
		Recommendations: make([]*types.BudgetRecommendation, 0),
		Errors:          make([]types.AnalysisError, 0),
	}

	// Process each account
	for _, cost := range costData {
		// Calculate statistics
		stats, err := analyzer.CalculateStatistics(cost)
		require.NoError(t, err)
		assert.NotNil(t, stats)

		// Get budget for this account
		var budgetConfig *types.BudgetConfig
		if budgets, ok := budgetData[cost.AccountID]; ok && len(budgets) > 0 {
			budgetConfig = budgets[0]
			result.AccountsWithBudgets++
		} else {
			result.AccountsWithoutBudgets++
		}

		// Compare to budget
		comparison, err := analyzer.CompareToBudget(stats, budgetConfig)
		require.NoError(t, err)
		assert.NotNil(t, comparison)

		// Generate recommendation
		recommendation, err := recommender.GenerateRecommendation(comparison, stats)
		require.NoError(t, err)
		assert.NotNil(t, recommendation)

		result.Recommendations = append(result.Recommendations, recommendation)
		result.AccountsAnalyzed++
	}

	// Prioritize recommendations
	result.Recommendations = recommender.PrioritizeRecommendations(result.Recommendations)

	// Verify results
	assert.Equal(t, 3, result.AccountsAnalyzed, "Should analyze all 3 accounts")
	assert.Equal(t, 2, result.AccountsWithBudgets, "Should find 2 accounts with budgets")
	assert.Equal(t, 1, result.AccountsWithoutBudgets, "Should find 1 account without budget")
	assert.Equal(t, 3, len(result.Recommendations), "Should generate 3 recommendations")
	assert.Equal(t, 0, len(result.Errors), "Should have no errors")

	// Verify specific recommendations
	for _, rec := range result.Recommendations {
		assert.NotEmpty(t, rec.AccountID)
		assert.NotEmpty(t, rec.AccountName)
		assert.Greater(t, rec.RecommendedBudget, 0.0)
		assert.NotEmpty(t, rec.Justification)
		assert.NotEmpty(t, rec.Priority)
	}

	// Test report generation
	t.Run("generate table report", func(t *testing.T) {
		rep := reporter.NewReporter(nil)
		tableReport, err := rep.GenerateTableReport(result.Recommendations)
		require.NoError(t, err)
		assert.NotEmpty(t, tableReport)
		assert.Contains(t, tableReport, "Test Account 1")
		assert.Contains(t, tableReport, "Test Account 2")
		assert.Contains(t, tableReport, "Test Account 3")
	})

	t.Run("generate JSON report", func(t *testing.T) {
		rep := reporter.NewReporter(nil)
		jsonReport, err := rep.GenerateJSONReport(result.Recommendations)
		require.NoError(t, err)
		assert.NotEmpty(t, jsonReport)
		assert.Contains(t, jsonReport, "123456789012")
		assert.Contains(t, jsonReport, "234567890123")
		assert.Contains(t, jsonReport, "345678901234")
	})
}

// TestEndToEndWorkflowWithErrors tests the workflow with partial failures
func TestEndToEndWorkflowWithErrors(t *testing.T) {
	_ = context.Background()

	// Create mock cost data with one error
	costData := []*types.AccountCostData{
		{
			AccountID:   "123456789012",
			AccountName: "Test Account 1",
			MonthlyCosts: []types.MonthlyCost{
				{Month: "2024-09", Amount: 450.00},
				{Month: "2024-10", Amount: 520.00},
			},
		},
		{
			AccountID:   "234567890123",
			AccountName: "Test Account 2",
			Error:       assert.AnError, // Simulate error
		},
		{
			AccountID:   "345678901234",
			AccountName: "Test Account 3",
			MonthlyCosts: []types.MonthlyCost{
				{Month: "2024-09", Amount: 25.00},
				{Month: "2024-10", Amount: 30.00},
			},
		},
	}

	// Initialize components
	analyzer := &analyzer.Analyzer{}
	recommender := recommender.NewRecommender(types.RecommendationPolicy{
		GrowthBuffer:      20.0,
		MinimumBudget:     10.0,
		RoundingIncrement: 10.0,
	})

	// Build analysis result
	result := &types.AnalysisResult{
		Timestamp:       time.Now(),
		Recommendations: make([]*types.BudgetRecommendation, 0),
		Errors:          make([]types.AnalysisError, 0),
	}

	// Process each account
	for _, cost := range costData {
		// Handle errors in cost data
		if cost.Error != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       cost.Error,
			})
			continue
		}

		// Calculate statistics
		stats, err := analyzer.CalculateStatistics(cost)
		if err != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       err,
			})
			continue
		}

		// Compare to budget (no budget for this test)
		comparison, err := analyzer.CompareToBudget(stats, nil)
		if err != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       err,
			})
			continue
		}

		// Generate recommendation
		recommendation, err := recommender.GenerateRecommendation(comparison, stats)
		if err != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       err,
			})
			continue
		}

		result.Recommendations = append(result.Recommendations, recommendation)
		result.AccountsAnalyzed++
	}

	// Verify partial failure handling
	assert.Equal(t, 2, result.AccountsAnalyzed, "Should successfully analyze 2 accounts")
	assert.Equal(t, 2, len(result.Recommendations), "Should generate 2 recommendations")
	assert.Equal(t, 1, len(result.Errors), "Should have 1 error")

	// Verify error details
	assert.Equal(t, "234567890123", result.Errors[0].AccountID)
	assert.Equal(t, "Test Account 2", result.Errors[0].AccountName)
	assert.NotNil(t, result.Errors[0].Error)

	// Verify total entries match input
	totalEntries := len(result.Recommendations) + len(result.Errors)
	assert.Equal(t, 3, totalEntries, "Total entries should equal input accounts")
}

// TestFilterAccountsIntegration tests account filtering in integration context
func TestFilterAccountsIntegration(t *testing.T) {
	allAccounts := []types.AccountInfo{
		{ID: "123456789012", Name: "Account 1"},
		{ID: "234567890123", Name: "Account 2"},
		{ID: "345678901234", Name: "Account 3"},
		{ID: "456789012345", Name: "Account 4"},
		{ID: "567890123456", Name: "Account 5"},
	}

	testCases := []struct {
		name          string
		filter        []string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "no filter returns all",
			filter:        []string{},
			expectedCount: 5,
			expectedIDs:   []string{"123456789012", "234567890123", "345678901234", "456789012345", "567890123456"},
		},
		{
			name:          "single account filter",
			filter:        []string{"123456789012"},
			expectedCount: 1,
			expectedIDs:   []string{"123456789012"},
		},
		{
			name:          "multiple account filter",
			filter:        []string{"123456789012", "345678901234", "567890123456"},
			expectedCount: 3,
			expectedIDs:   []string{"123456789012", "345678901234", "567890123456"},
		},
		{
			name:          "non-existent account",
			filter:        []string{"999999999999"},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name:          "mix of existing and non-existing",
			filter:        []string{"123456789012", "999999999999", "345678901234"},
			expectedCount: 2,
			expectedIDs:   []string{"123456789012", "345678901234"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtered := filterAccounts(allAccounts, tc.filter)
			assert.Equal(t, tc.expectedCount, len(filtered))

			// Verify expected IDs are present
			filteredIDs := make(map[string]bool)
			for _, acc := range filtered {
				filteredIDs[acc.ID] = true
			}

			for _, expectedID := range tc.expectedIDs {
				assert.True(t, filteredIDs[expectedID], "Expected account %s to be in filtered results", expectedID)
			}
		})
	}
}
