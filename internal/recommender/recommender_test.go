package recommender

import (
	"testing"

	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecommender(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}

	recommender := NewRecommender(policy)

	assert.NotNil(t, recommender)
	assert.Equal(t, policy, recommender.policy)
}

func TestGenerateRecommendation_NilInputs(t *testing.T) {
	recommender := NewRecommender(types.RecommendationPolicy{})

	t.Run("nil comparison", func(t *testing.T) {
		stats := &types.SpendStatistics{}
		rec, err := recommender.GenerateRecommendation(nil, stats)
		assert.Error(t, err)
		assert.Nil(t, rec)
	})

	t.Run("nil statistics", func(t *testing.T) {
		comparison := &types.BudgetComparison{}
		rec, err := recommender.GenerateRecommendation(comparison, nil)
		assert.Error(t, err)
		assert.Nil(t, rec)
	})
}

func TestGenerateRecommendation_BasicCalculation(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}
	recommender := NewRecommender(policy)

	comparison := &types.BudgetComparison{
		AccountID:    "123456789012",
		AccountName:  "test-account",
		AverageSpend: 400,
		PeakSpend:    500,
	}

	statistics := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 400,
		PeakMonthlySpend:    500,
		MonthsAnalyzed:      3,
		Trend:               types.TrendStable,
	}

	rec, err := recommender.GenerateRecommendation(comparison, statistics)

	require.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, "123456789012", rec.AccountID)
	assert.Equal(t, "test-account", rec.AccountName)
	assert.Equal(t, 400.0, rec.AverageSpend)
	assert.Equal(t, 500.0, rec.PeakSpend)
	// 500 * 1.20 = 600, rounded to 600
	assert.Equal(t, 600.0, rec.RecommendedBudget)
}

func TestGenerateRecommendation_WithCurrentBudget(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}
	recommender := NewRecommender(policy)

	currentBudget := 450.0
	comparison := &types.BudgetComparison{
		AccountID:     "123456789012",
		AccountName:   "test-account",
		CurrentBudget: &currentBudget,
		AverageSpend:  400,
		PeakSpend:     500,
	}

	statistics := &types.SpendStatistics{
		PeakMonthlySpend: 500,
		MonthsAnalyzed:   3,
	}

	rec, err := recommender.GenerateRecommendation(comparison, statistics)

	require.NoError(t, err)
	assert.NotNil(t, rec.CurrentBudget)
	assert.Equal(t, 450.0, *rec.CurrentBudget)
	assert.Equal(t, 600.0, rec.RecommendedBudget)
	// (600 - 450) / 450 * 100 = 33.33%
	assert.InDelta(t, 33.33, rec.AdjustmentPercent, 0.01)
}

func TestGenerateRecommendation_MinimumBudget(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      20,
		MinimumBudget:     100,
		RoundingIncrement: 10,
	}
	recommender := NewRecommender(policy)

	comparison := &types.BudgetComparison{
		AccountID:    "123456789012",
		AccountName:  "test-account",
		AverageSpend: 5,
		PeakSpend:    10,
	}

	statistics := &types.SpendStatistics{
		PeakMonthlySpend: 10,
		MonthsAnalyzed:   3,
	}

	rec, err := recommender.GenerateRecommendation(comparison, statistics)

	require.NoError(t, err)
	// 10 * 1.20 = 12, but minimum is 100
	assert.Equal(t, 100.0, rec.RecommendedBudget)
}

func TestGenerateRecommendation_DefaultGrowthBuffer(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      0, // Will use default 20%
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}
	recommender := NewRecommender(policy)

	comparison := &types.BudgetComparison{
		AccountID:    "123456789012",
		AccountName:  "test-account",
		AverageSpend: 400,
		PeakSpend:    500,
	}

	statistics := &types.SpendStatistics{
		PeakMonthlySpend: 500,
		MonthsAnalyzed:   3,
	}

	rec, err := recommender.GenerateRecommendation(comparison, statistics)

	require.NoError(t, err)
	// Should use default 20%: 500 * 1.20 = 600
	assert.Equal(t, 600.0, rec.RecommendedBudget)
}

func TestRoundToIncrement(t *testing.T) {
	recommender := &Recommender{}

	tests := []struct {
		name      string
		value     float64
		increment float64
		expected  float64
	}{
		{"round to 10", 123.0, 10, 120},
		{"round to 10 up", 127.0, 10, 130},
		{"round to 50", 123.0, 50, 100},
		{"round to 50 up", 140.0, 50, 150},
		{"round to 100", 567.0, 100, 600},
		{"exact match", 100.0, 10, 100},
		{"zero increment", 123.0, 0, 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recommender.roundToIncrement(tt.value, tt.increment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeterminePriority(t *testing.T) {
	recommender := &Recommender{}

	tests := []struct {
		name              string
		status            types.BudgetStatus
		adjustmentPercent float64
		expected          types.Priority
	}{
		{"over budget", types.StatusOverBudget, 10, types.PriorityHigh},
		{"large adjustment", types.StatusAppropriate, 60, types.PriorityHigh},
		{"medium adjustment", types.StatusAppropriate, 30, types.PriorityMedium},
		{"small adjustment", types.StatusAppropriate, 10, types.PriorityLow},
		{"negative large", types.StatusUnderUtilized, -60, types.PriorityHigh},
		{"negative medium", types.StatusUnderUtilized, -30, types.PriorityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comparison := &types.BudgetComparison{
				Status: tt.status,
			}
			priority := recommender.determinePriority(comparison, tt.adjustmentPercent)
			assert.Equal(t, tt.expected, priority)
		})
	}
}

func TestGenerateJustification(t *testing.T) {
	recommender := &Recommender{}

	t.Run("with historical data", func(t *testing.T) {
		statistics := &types.SpendStatistics{
			AverageMonthlySpend: 400,
			PeakMonthlySpend:    500,
			MonthsAnalyzed:      3,
			Trend:               types.TrendIncreasing,
		}

		justification := recommender.generateJustification(statistics, 600, 20)

		assert.Contains(t, justification, "3-month analysis")
		assert.Contains(t, justification, "avg=$400")
		assert.Contains(t, justification, "peak=$500")
		assert.Contains(t, justification, "increasing")
	})

	t.Run("no historical data", func(t *testing.T) {
		statistics := &types.SpendStatistics{
			MonthsAnalyzed: 0,
		}

		justification := recommender.generateJustification(statistics, 100, 20)

		assert.Contains(t, justification, "No historical spend data")
		assert.Contains(t, justification, "$100")
	})

	t.Run("decreasing trend", func(t *testing.T) {
		statistics := &types.SpendStatistics{
			AverageMonthlySpend: 400,
			PeakMonthlySpend:    500,
			MonthsAnalyzed:      3,
			Trend:               types.TrendDecreasing,
		}

		justification := recommender.generateJustification(statistics, 600, 20)

		assert.Contains(t, justification, "decreasing")
	})
}

func TestPrioritizeRecommendations(t *testing.T) {
	recommender := &Recommender{}

	recommendations := []*types.BudgetRecommendation{
		{AccountID: "1", AdjustmentPercent: 10},
		{AccountID: "2", AdjustmentPercent: 50},
		{AccountID: "3", AdjustmentPercent: -30},
		{AccountID: "4", AdjustmentPercent: 5},
	}

	sorted := recommender.PrioritizeRecommendations(recommendations)

	require.Len(t, sorted, 4)
	// Should be sorted by absolute value: 50, 30, 10, 5
	assert.Equal(t, "2", sorted[0].AccountID) // 50%
	assert.Equal(t, "3", sorted[1].AccountID) // -30%
	assert.Equal(t, "1", sorted[2].AccountID) // 10%
	assert.Equal(t, "4", sorted[3].AccountID) // 5%

	// Original should be unchanged
	assert.Equal(t, "1", recommendations[0].AccountID)
}

func TestGenerateRecommendation_NoBudgetNewAccount(t *testing.T) {
	policy := types.RecommendationPolicy{
		GrowthBuffer:      20,
		MinimumBudget:     10,
		RoundingIncrement: 10,
	}
	recommender := NewRecommender(policy)

	comparison := &types.BudgetComparison{
		AccountID:     "123456789012",
		AccountName:   "new-account",
		CurrentBudget: nil, // No existing budget
		AverageSpend:  400,
		PeakSpend:     500,
		Status:        types.StatusNoBudget,
	}

	statistics := &types.SpendStatistics{
		PeakMonthlySpend: 500,
		MonthsAnalyzed:   3,
	}

	rec, err := recommender.GenerateRecommendation(comparison, statistics)

	require.NoError(t, err)
	assert.Nil(t, rec.CurrentBudget)
	assert.Equal(t, 600.0, rec.RecommendedBudget)
	assert.Equal(t, 100.0, rec.AdjustmentPercent) // New budget = 100% change
}
