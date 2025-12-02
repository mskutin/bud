package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReporter(t *testing.T) {
	t.Run("with writer", func(t *testing.T) {
		var buf bytes.Buffer
		reporter := NewReporter(&buf)
		assert.NotNil(t, reporter)
		assert.Equal(t, &buf, reporter.writer)
	})

	t.Run("with nil writer", func(t *testing.T) {
		reporter := NewReporter(nil)
		assert.NotNil(t, reporter)
		assert.Equal(t, os.Stdout, reporter.writer)
	})
}

func TestGenerateTableReport_Empty(t *testing.T) {
	reporter := NewReporter(nil)

	output, err := reporter.GenerateTableReport([]*types.BudgetRecommendation{})

	require.NoError(t, err)
	assert.Contains(t, output, "No recommendations")
}

func TestGenerateTableReport_WithData(t *testing.T) {
	reporter := NewReporter(nil)

	currentBudget := 500.0
	recommendations := []*types.BudgetRecommendation{
		{
			AccountID:         "123456789012",
			AccountName:       "test-account",
			CurrentBudget:     &currentBudget,
			RecommendedBudget: 600,
			AverageSpend:      450,
			PeakSpend:         550,
			AdjustmentPercent: 20,
			Priority:          types.PriorityMedium,
		},
	}

	output, err := reporter.GenerateTableReport(recommendations)

	require.NoError(t, err)
	assert.Contains(t, output, "AWS Budget Optimization Report")
	assert.Contains(t, output, "test-account")
	assert.Contains(t, output, "123456789012")
	assert.Contains(t, output, "$500")
	assert.Contains(t, output, "$600")
	assert.Contains(t, output, "Summary")
}

func TestGenerateJSONReport(t *testing.T) {
	reporter := NewReporter(nil)

	currentBudget := 500.0
	recommendations := []*types.BudgetRecommendation{
		{
			AccountID:         "123456789012",
			AccountName:       "test-account",
			CurrentBudget:     &currentBudget,
			RecommendedBudget: 600,
			AverageSpend:      450,
			PeakSpend:         550,
			AdjustmentPercent: 20,
			Priority:          types.PriorityMedium,
			Justification:     "Test justification",
		},
	}

	output, err := reporter.GenerateJSONReport(recommendations)

	require.NoError(t, err)

	// Parse JSON to verify structure
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	assert.Contains(t, result, "timestamp")
	assert.Contains(t, result, "recommendations")
	assert.Contains(t, result, "summary")

	summary := result["summary"].(map[string]interface{})
	assert.Equal(t, float64(1), summary["total"])
}

func TestFormatCurrency(t *testing.T) {
	reporter := &Reporter{}

	tests := []struct {
		name     string
		value    *float64
		expected string
	}{
		{"nil value", nil, "-"},
		{"zero", ptr(0.0), "$0"},
		{"positive", ptr(123.45), "$123"},
		{"large", ptr(1234567.89), "$1234568"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reporter.formatCurrency(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncate(t *testing.T) {
	reporter := &Reporter{}

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very long", "this is a very long string", 10, "this is..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reporter.truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountByPriority(t *testing.T) {
	reporter := &Reporter{}

	recommendations := []*types.BudgetRecommendation{
		{Priority: types.PriorityHigh},
		{Priority: types.PriorityHigh},
		{Priority: types.PriorityMedium},
		{Priority: types.PriorityLow},
	}

	assert.Equal(t, 2, reporter.countByPriority(recommendations, types.PriorityHigh))
	assert.Equal(t, 1, reporter.countByPriority(recommendations, types.PriorityMedium))
	assert.Equal(t, 1, reporter.countByPriority(recommendations, types.PriorityLow))
}

func TestSumCurrentBudgets(t *testing.T) {
	reporter := &Reporter{}

	recommendations := []*types.BudgetRecommendation{
		{CurrentBudget: ptr(100.0)},
		{CurrentBudget: ptr(200.0)},
		{CurrentBudget: nil}, // Should be skipped
		{CurrentBudget: ptr(300.0)},
	}

	sum := reporter.sumCurrentBudgets(recommendations)
	assert.Equal(t, 600.0, sum)
}

func TestSumRecommendedBudgets(t *testing.T) {
	reporter := &Reporter{}

	recommendations := []*types.BudgetRecommendation{
		{RecommendedBudget: 100.0},
		{RecommendedBudget: 200.0},
		{RecommendedBudget: 300.0},
	}

	sum := reporter.sumRecommendedBudgets(recommendations)
	assert.Equal(t, 600.0, sum)
}

func TestSortRecommendations(t *testing.T) {
	reporter := &Reporter{}

	recommendations := []*types.BudgetRecommendation{
		{AccountID: "1", AccountName: "charlie", Priority: types.PriorityLow, AdjustmentPercent: 10},
		{AccountID: "2", AccountName: "alice", Priority: types.PriorityHigh, AdjustmentPercent: 50},
		{AccountID: "3", AccountName: "bob", Priority: types.PriorityMedium, AdjustmentPercent: 30},
	}

	t.Run("sort by priority", func(t *testing.T) {
		sorted := reporter.sortRecommendations(recommendations, types.SortByPriority)
		assert.Equal(t, "2", sorted[0].AccountID) // High
		assert.Equal(t, "3", sorted[1].AccountID) // Medium
		assert.Equal(t, "1", sorted[2].AccountID) // Low
	})

	t.Run("sort by adjustment", func(t *testing.T) {
		sorted := reporter.sortRecommendations(recommendations, types.SortByAdjustment)
		assert.Equal(t, "2", sorted[0].AccountID) // 50%
		assert.Equal(t, "3", sorted[1].AccountID) // 30%
		assert.Equal(t, "1", sorted[2].AccountID) // 10%
	})

	t.Run("sort by account", func(t *testing.T) {
		sorted := reporter.sortRecommendations(recommendations, types.SortByAccount)
		assert.Equal(t, "alice", sorted[0].AccountName)
		assert.Equal(t, "bob", sorted[1].AccountName)
		assert.Equal(t, "charlie", sorted[2].AccountName)
	})
}

func TestPriorityValue(t *testing.T) {
	reporter := &Reporter{}

	assert.Equal(t, 3, reporter.priorityValue(types.PriorityHigh))
	assert.Equal(t, 2, reporter.priorityValue(types.PriorityMedium))
	assert.Equal(t, 1, reporter.priorityValue(types.PriorityLow))
	assert.Equal(t, 0, reporter.priorityValue("unknown"))
}

func TestOutputReport_Table(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf)

	currentBudget := 500.0
	recommendations := []*types.BudgetRecommendation{
		{
			AccountID:         "123456789012",
			AccountName:       "test-account",
			CurrentBudget:     &currentBudget,
			RecommendedBudget: 600,
			AverageSpend:      450,
			PeakSpend:         550,
			AdjustmentPercent: 20,
			Priority:          types.PriorityMedium,
		},
	}

	options := types.ReportOptions{
		Format: types.FormatTable,
		SortBy: types.SortByPriority,
	}

	err := reporter.OutputReport(recommendations, options)

	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-account")
	assert.Contains(t, output, "Summary")
}

func TestOutputReport_JSON(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf)

	currentBudget := 500.0
	recommendations := []*types.BudgetRecommendation{
		{
			AccountID:         "123456789012",
			AccountName:       "test-account",
			CurrentBudget:     &currentBudget,
			RecommendedBudget: 600,
			AverageSpend:      450,
			PeakSpend:         550,
			AdjustmentPercent: 20,
			Priority:          types.PriorityMedium,
		},
	}

	options := types.ReportOptions{
		Format: types.FormatJSON,
		SortBy: types.SortByPriority,
	}

	err := reporter.OutputReport(recommendations, options)

	require.NoError(t, err)
	output := buf.String()

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)
}

func TestGenerateSummary(t *testing.T) {
	reporter := &Reporter{}

	recommendations := []*types.BudgetRecommendation{
		{Priority: types.PriorityHigh, CurrentBudget: ptr(100.0), RecommendedBudget: 150},
		{Priority: types.PriorityMedium, CurrentBudget: ptr(200.0), RecommendedBudget: 250},
		{Priority: types.PriorityLow, CurrentBudget: ptr(300.0), RecommendedBudget: 320},
	}

	summary := reporter.generateSummary(recommendations)

	assert.Contains(t, summary, "Total accounts analyzed: 3")
	assert.Contains(t, summary, "High priority: 1")
	assert.Contains(t, summary, "Medium priority: 1")
	assert.Contains(t, summary, "Low priority: 1")
	assert.Contains(t, summary, "Total current budgets")
	assert.Contains(t, summary, "Total recommended budgets")
}

func TestFormatChange(t *testing.T) {
	reporter := &Reporter{}

	tests := []struct {
		name    string
		percent float64
	}{
		{"large positive", 60},
		{"medium positive", 30},
		{"small positive", 10},
		{"small negative", -10},
		{"large negative", -30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reporter.formatChange(tt.percent)
			// Just verify it contains the percentage
			assert.Contains(t, strings.ToLower(result), fmt.Sprintf("%.1f", math.Abs(tt.percent)))
		})
	}
}

// Helper function to create pointer to float64
func ptr(f float64) *float64 {
	return &f
}
