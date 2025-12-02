package analyzer

import (
	"fmt"
	"math"

	"github.com/mskutin/bud/pkg/types"
)

// Analyzer calculates spending statistics and compares against budgets
type Analyzer struct{}

// NewAnalyzer creates a new Analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// CalculateStatistics computes spending statistics from cost data
func (a *Analyzer) CalculateStatistics(costData *types.AccountCostData) (*types.SpendStatistics, error) {
	if costData == nil {
		return nil, fmt.Errorf("cost data cannot be nil")
	}

	if costData.Error != nil {
		return nil, fmt.Errorf("cost data contains error: %w", costData.Error)
	}

	stats := &types.SpendStatistics{
		AccountID:   costData.AccountID,
		AccountName: costData.AccountName,
	}

	if len(costData.MonthlyCosts) == 0 {
		// No cost data available
		stats.MonthsAnalyzed = 0
		stats.Trend = types.TrendStable
		return stats, nil
	}

	// Calculate average, peak, and min
	var sum float64
	peak := costData.MonthlyCosts[0].Amount
	min := costData.MonthlyCosts[0].Amount

	for _, cost := range costData.MonthlyCosts {
		sum += cost.Amount
		if cost.Amount > peak {
			peak = cost.Amount
		}
		if cost.Amount < min {
			min = cost.Amount
		}
	}

	count := len(costData.MonthlyCosts)
	stats.AverageMonthlySpend = sum / float64(count)
	stats.PeakMonthlySpend = peak
	stats.MinMonthlySpend = min
	stats.MonthsAnalyzed = count

	// Set current month spend (last month in the data)
	if count > 0 {
		currentSpend := costData.MonthlyCosts[count-1].Amount
		stats.CurrentMonthSpend = &currentSpend
	}

	// Calculate trend
	stats.Trend = a.calculateTrend(costData.MonthlyCosts)

	return stats, nil
}

// CompareToBudget compares spending statistics against budget configuration
func (a *Analyzer) CompareToBudget(
	statistics *types.SpendStatistics,
	budgetConfig *types.BudgetConfig,
) (*types.BudgetComparison, error) {
	if statistics == nil {
		return nil, fmt.Errorf("statistics cannot be nil")
	}

	comparison := &types.BudgetComparison{
		AccountID:    statistics.AccountID,
		AccountName:  statistics.AccountName,
		AverageSpend: statistics.AverageMonthlySpend,
		PeakSpend:    statistics.PeakMonthlySpend,
	}

	// If no budget is configured
	if budgetConfig == nil {
		comparison.Status = types.StatusNoBudget
		return comparison, nil
	}

	// Set current budget
	comparison.CurrentBudget = &budgetConfig.LimitAmount

	// Calculate utilization percentage
	if budgetConfig.LimitAmount > 0 {
		utilization := (statistics.AverageMonthlySpend / budgetConfig.LimitAmount) * 100
		comparison.UtilizationPercent = &utilization

		// Determine status based on utilization
		if utilization > 100 {
			comparison.Status = types.StatusOverBudget
		} else if utilization < 50 {
			comparison.Status = types.StatusUnderUtilized
		} else {
			comparison.Status = types.StatusAppropriate
		}
	} else {
		// Budget is zero or negative - treat as no budget
		comparison.Status = types.StatusNoBudget
	}

	return comparison, nil
}

// calculateTrend determines the spending trend from monthly costs
func (a *Analyzer) calculateTrend(monthlyCosts []types.MonthlyCost) types.Trend {
	if len(monthlyCosts) < 2 {
		return types.TrendStable
	}

	// Use simple linear regression to determine trend
	// Calculate slope of the best-fit line
	n := float64(len(monthlyCosts))
	var sumX, sumY, sumXY, sumX2 float64

	for i, cost := range monthlyCosts {
		x := float64(i)
		y := cost.Amount
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope: m = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	numerator := n*sumXY - sumX*sumY
	denominator := n*sumX2 - sumX*sumX

	if denominator == 0 {
		return types.TrendStable
	}

	slope := numerator / denominator

	// Determine trend based on slope
	// Use a threshold to avoid classifying small changes as trends
	threshold := 0.05 * (sumY / n) // 5% of average spend

	if math.Abs(slope) < threshold {
		return types.TrendStable
	} else if slope > 0 {
		return types.TrendIncreasing
	} else {
		return types.TrendDecreasing
	}
}
