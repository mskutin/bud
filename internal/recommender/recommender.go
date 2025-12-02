package recommender

import (
	"fmt"
	"math"
	"sort"

	"github.com/mskutin/bud/pkg/types"
)

// Recommender generates budget recommendations based on analysis
type Recommender struct {
	policy types.RecommendationPolicy
}

// NewRecommender creates a new Recommender with the given policy
func NewRecommender(policy types.RecommendationPolicy) *Recommender {
	return &Recommender{
		policy: policy,
	}
}

// GenerateRecommendation creates a budget recommendation based on comparison and statistics
// Uses the recommender's default policy
func (r *Recommender) GenerateRecommendation(
	comparison *types.BudgetComparison,
	statistics *types.SpendStatistics,
) (*types.BudgetRecommendation, error) {
	return r.GenerateRecommendationWithPolicy(comparison, statistics, r.policy)
}

// GenerateRecommendationWithPolicy creates a budget recommendation using a specific policy
func (r *Recommender) GenerateRecommendationWithPolicy(
	comparison *types.BudgetComparison,
	statistics *types.SpendStatistics,
	policy types.RecommendationPolicy,
) (*types.BudgetRecommendation, error) {
	if comparison == nil {
		return nil, fmt.Errorf("comparison cannot be nil")
	}
	if statistics == nil {
		return nil, fmt.Errorf("statistics cannot be nil")
	}

	recommendation := &types.BudgetRecommendation{
		AccountID:     comparison.AccountID,
		AccountName:   comparison.AccountName,
		CurrentBudget: comparison.CurrentBudget,
		AverageSpend:  comparison.AverageSpend,
		PeakSpend:     comparison.PeakSpend,
		PolicyName:    policy.Name, // Set the policy name
	}

	// Calculate recommended budget based on peak spend + growth buffer
	growthBuffer := policy.GrowthBuffer
	if growthBuffer == 0 {
		growthBuffer = 20 // Default 20% if not specified
	}

	recommendedBudget := statistics.PeakMonthlySpend * (1 + growthBuffer/100)

	// Apply minimum budget threshold
	if recommendedBudget < policy.MinimumBudget {
		recommendedBudget = policy.MinimumBudget
	}

	// Round to nearest increment
	if policy.RoundingIncrement > 0 {
		recommendedBudget = r.roundToIncrement(recommendedBudget, policy.RoundingIncrement)
	}

	recommendation.RecommendedBudget = recommendedBudget

	// Calculate adjustment percentage
	if comparison.CurrentBudget != nil && *comparison.CurrentBudget > 0 {
		adjustment := ((recommendedBudget - *comparison.CurrentBudget) / *comparison.CurrentBudget) * 100
		recommendation.AdjustmentPercent = adjustment
	} else {
		// No current budget - this is a new budget
		recommendation.AdjustmentPercent = 100
	}

	// Determine priority
	recommendation.Priority = r.determinePriority(comparison, recommendation.AdjustmentPercent)

	// Generate justification
	recommendation.Justification = r.generateJustification(
		statistics,
		recommendedBudget,
		growthBuffer,
	)

	return recommendation, nil
}

// PrioritizeRecommendations sorts recommendations by adjustment magnitude
func (r *Recommender) PrioritizeRecommendations(
	recommendations []*types.BudgetRecommendation,
) []*types.BudgetRecommendation {
	// Create a copy to avoid modifying the original slice
	sorted := make([]*types.BudgetRecommendation, len(recommendations))
	copy(sorted, recommendations)

	// Sort by absolute adjustment percentage (descending)
	sort.Slice(sorted, func(i, j int) bool {
		absI := math.Abs(sorted[i].AdjustmentPercent)
		absJ := math.Abs(sorted[j].AdjustmentPercent)
		return absI > absJ
	})

	return sorted
}

// roundToIncrement rounds a value to the nearest increment
func (r *Recommender) roundToIncrement(value, increment float64) float64 {
	if increment == 0 {
		return value
	}
	return math.Round(value/increment) * increment
}

// determinePriority determines the priority level based on comparison status and adjustment
func (r *Recommender) determinePriority(
	comparison *types.BudgetComparison,
	adjustmentPercent float64,
) types.Priority {
	absAdjustment := math.Abs(adjustmentPercent)

	// High priority: over-budget or large adjustment needed
	if comparison.Status == types.StatusOverBudget || absAdjustment > 50 {
		return types.PriorityHigh
	}

	// Medium priority: moderate adjustment needed
	if absAdjustment > 20 {
		return types.PriorityMedium
	}

	// Low priority: small adjustment or appropriate budget
	return types.PriorityLow
}

// generateJustification creates a human-readable justification for the recommendation
func (r *Recommender) generateJustification(
	statistics *types.SpendStatistics,
	recommendedBudget float64,
	growthBuffer float64,
) string {
	if statistics.MonthsAnalyzed == 0 {
		return fmt.Sprintf(
			"No historical spend data available. Recommended minimum budget: $%.0f",
			recommendedBudget,
		)
	}

	baseCalculation := statistics.PeakMonthlySpend * (1 + growthBuffer/100)

	justification := fmt.Sprintf(
		"Based on %d-month analysis: avg=$%.0f, peak=$%.0f. "+
			"Recommended budget: $%.0f × %.2f = $%.0f",
		statistics.MonthsAnalyzed,
		statistics.AverageMonthlySpend,
		statistics.PeakMonthlySpend,
		statistics.PeakMonthlySpend,
		1+growthBuffer/100,
		baseCalculation,
	)

	// Add rounding note if applicable
	if math.Abs(baseCalculation-recommendedBudget) > 0.01 {
		justification += fmt.Sprintf(", rounded to $%.0f", recommendedBudget)
	}

	// Add trend information
	switch statistics.Trend {
	case types.TrendIncreasing:
		justification += ". Trend: increasing (consider higher buffer)"
	case types.TrendDecreasing:
		justification += ". Trend: decreasing (may reduce in future)"
	}

	return justification
}
