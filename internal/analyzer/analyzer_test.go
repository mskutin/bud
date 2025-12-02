package analyzer

import (
	"errors"
	"testing"

	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyzer(t *testing.T) {
	analyzer := NewAnalyzer()
	assert.NotNil(t, analyzer)
}

func TestCalculateStatistics_NilCostData(t *testing.T) {
	analyzer := NewAnalyzer()

	stats, err := analyzer.CalculateStatistics(nil)

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestCalculateStatistics_WithError(t *testing.T) {
	analyzer := NewAnalyzer()

	costData := &types.AccountCostData{
		AccountID:   "123456789012",
		AccountName: "test-account",
		Error:       errors.New("API error"),
	}

	stats, err := analyzer.CalculateStatistics(costData)

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "API error")
}

func TestCalculateStatistics_EmptyCosts(t *testing.T) {
	analyzer := NewAnalyzer()

	costData := &types.AccountCostData{
		AccountID:    "123456789012",
		AccountName:  "test-account",
		MonthlyCosts: []types.MonthlyCost{},
	}

	stats, err := analyzer.CalculateStatistics(costData)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "123456789012", stats.AccountID)
	assert.Equal(t, "test-account", stats.AccountName)
	assert.Equal(t, 0, stats.MonthsAnalyzed)
	assert.Equal(t, types.TrendStable, stats.Trend)
}

func TestCalculateStatistics_SingleMonth(t *testing.T) {
	analyzer := NewAnalyzer()

	costData := &types.AccountCostData{
		AccountID:   "123456789012",
		AccountName: "test-account",
		MonthlyCosts: []types.MonthlyCost{
			{Month: "2024-01", Amount: 100.0},
		},
	}

	stats, err := analyzer.CalculateStatistics(costData)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 100.0, stats.AverageMonthlySpend)
	assert.Equal(t, 100.0, stats.PeakMonthlySpend)
	assert.Equal(t, 100.0, stats.MinMonthlySpend)
	assert.Equal(t, 1, stats.MonthsAnalyzed)
	assert.NotNil(t, stats.CurrentMonthSpend)
	assert.Equal(t, 100.0, *stats.CurrentMonthSpend)
	assert.Equal(t, types.TrendStable, stats.Trend)
}

func TestCalculateStatistics_MultipleMonths(t *testing.T) {
	analyzer := NewAnalyzer()

	costData := &types.AccountCostData{
		AccountID:   "123456789012",
		AccountName: "test-account",
		MonthlyCosts: []types.MonthlyCost{
			{Month: "2024-01", Amount: 100.0},
			{Month: "2024-02", Amount: 150.0},
			{Month: "2024-03", Amount: 200.0},
		},
	}

	stats, err := analyzer.CalculateStatistics(costData)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 150.0, stats.AverageMonthlySpend) // (100+150+200)/3
	assert.Equal(t, 200.0, stats.PeakMonthlySpend)
	assert.Equal(t, 100.0, stats.MinMonthlySpend)
	assert.Equal(t, 3, stats.MonthsAnalyzed)
	assert.NotNil(t, stats.CurrentMonthSpend)
	assert.Equal(t, 200.0, *stats.CurrentMonthSpend)
	assert.Equal(t, types.TrendIncreasing, stats.Trend)
}

func TestCompareToBudget_NilStatistics(t *testing.T) {
	analyzer := NewAnalyzer()

	comparison, err := analyzer.CompareToBudget(nil, nil)

	assert.Error(t, err)
	assert.Nil(t, comparison)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestCompareToBudget_NoBudget(t *testing.T) {
	analyzer := NewAnalyzer()

	stats := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 150.0,
		PeakMonthlySpend:    200.0,
	}

	comparison, err := analyzer.CompareToBudget(stats, nil)

	require.NoError(t, err)
	assert.NotNil(t, comparison)
	assert.Equal(t, "123456789012", comparison.AccountID)
	assert.Equal(t, "test-account", comparison.AccountName)
	assert.Equal(t, 150.0, comparison.AverageSpend)
	assert.Equal(t, 200.0, comparison.PeakSpend)
	assert.Nil(t, comparison.CurrentBudget)
	assert.Nil(t, comparison.UtilizationPercent)
	assert.Equal(t, types.StatusNoBudget, comparison.Status)
}

func TestCompareToBudget_OverBudget(t *testing.T) {
	analyzer := NewAnalyzer()

	stats := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 550.0,
		PeakMonthlySpend:    600.0,
	}

	budget := &types.BudgetConfig{
		AccountID:   "123456789012",
		LimitAmount: 500.0,
	}

	comparison, err := analyzer.CompareToBudget(stats, budget)

	require.NoError(t, err)
	assert.NotNil(t, comparison)
	assert.NotNil(t, comparison.CurrentBudget)
	assert.Equal(t, 500.0, *comparison.CurrentBudget)
	assert.NotNil(t, comparison.UtilizationPercent)
	assert.InDelta(t, 110.0, *comparison.UtilizationPercent, 0.01) // 550/500 * 100
	assert.Equal(t, types.StatusOverBudget, comparison.Status)
}

func TestCompareToBudget_UnderUtilized(t *testing.T) {
	analyzer := NewAnalyzer()

	stats := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 200.0,
		PeakMonthlySpend:    250.0,
	}

	budget := &types.BudgetConfig{
		AccountID:   "123456789012",
		LimitAmount: 500.0,
	}

	comparison, err := analyzer.CompareToBudget(stats, budget)

	require.NoError(t, err)
	assert.NotNil(t, comparison)
	assert.NotNil(t, comparison.UtilizationPercent)
	assert.Equal(t, 40.0, *comparison.UtilizationPercent) // 200/500 * 100
	assert.Equal(t, types.StatusUnderUtilized, comparison.Status)
}

func TestCompareToBudget_Appropriate(t *testing.T) {
	analyzer := NewAnalyzer()

	stats := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 350.0,
		PeakMonthlySpend:    400.0,
	}

	budget := &types.BudgetConfig{
		AccountID:   "123456789012",
		LimitAmount: 500.0,
	}

	comparison, err := analyzer.CompareToBudget(stats, budget)

	require.NoError(t, err)
	assert.NotNil(t, comparison)
	assert.NotNil(t, comparison.UtilizationPercent)
	assert.Equal(t, 70.0, *comparison.UtilizationPercent) // 350/500 * 100
	assert.Equal(t, types.StatusAppropriate, comparison.Status)
}

func TestCompareToBudget_ZeroBudget(t *testing.T) {
	analyzer := NewAnalyzer()

	stats := &types.SpendStatistics{
		AccountID:           "123456789012",
		AccountName:         "test-account",
		AverageMonthlySpend: 100.0,
		PeakMonthlySpend:    150.0,
	}

	budget := &types.BudgetConfig{
		AccountID:   "123456789012",
		LimitAmount: 0.0,
	}

	comparison, err := analyzer.CompareToBudget(stats, budget)

	require.NoError(t, err)
	assert.NotNil(t, comparison)
	assert.Equal(t, types.StatusNoBudget, comparison.Status)
}

func TestCalculateTrend_Increasing(t *testing.T) {
	analyzer := NewAnalyzer()

	costs := []types.MonthlyCost{
		{Month: "2024-01", Amount: 100.0},
		{Month: "2024-02", Amount: 150.0},
		{Month: "2024-03", Amount: 200.0},
		{Month: "2024-04", Amount: 250.0},
	}

	trend := analyzer.calculateTrend(costs)
	assert.Equal(t, types.TrendIncreasing, trend)
}

func TestCalculateTrend_Decreasing(t *testing.T) {
	analyzer := NewAnalyzer()

	costs := []types.MonthlyCost{
		{Month: "2024-01", Amount: 250.0},
		{Month: "2024-02", Amount: 200.0},
		{Month: "2024-03", Amount: 150.0},
		{Month: "2024-04", Amount: 100.0},
	}

	trend := analyzer.calculateTrend(costs)
	assert.Equal(t, types.TrendDecreasing, trend)
}

func TestCalculateTrend_Stable(t *testing.T) {
	analyzer := NewAnalyzer()

	costs := []types.MonthlyCost{
		{Month: "2024-01", Amount: 100.0},
		{Month: "2024-02", Amount: 102.0},
		{Month: "2024-03", Amount: 98.0},
		{Month: "2024-04", Amount: 101.0},
	}

	trend := analyzer.calculateTrend(costs)
	assert.Equal(t, types.TrendStable, trend)
}

func TestCalculateTrend_SingleMonth(t *testing.T) {
	analyzer := NewAnalyzer()

	costs := []types.MonthlyCost{
		{Month: "2024-01", Amount: 100.0},
	}

	trend := analyzer.calculateTrend(costs)
	assert.Equal(t, types.TrendStable, trend)
}
