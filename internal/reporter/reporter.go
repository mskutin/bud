package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mskutin/bud/pkg/types"
)

// Reporter generates formatted reports
type Reporter struct {
	writer io.Writer
}

// NewReporter creates a new Reporter
func NewReporter(writer io.Writer) *Reporter {
	if writer == nil {
		writer = os.Stdout
	}
	return &Reporter{
		writer: writer,
	}
}

// GenerateTableReport creates a formatted table report
func (r *Reporter) GenerateTableReport(recommendations []*types.BudgetRecommendation) (string, error) {
	if len(recommendations) == 0 {
		return "No recommendations to display.\n", nil
	}

	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	sb.WriteString(color.New(color.Bold).Sprint("AWS Budget Optimization Report"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Fixed-width columns (to handle ANSI color codes properly)
	// Priority: 8, Account Name: 30, Policy: 15, Account ID: 14, Current: 10, Average: 10, Peak: 10, Recommended: 12, Adjustment: 10
	headerFormat := "%-8s  %-30s  %-15s  %-14s  %-10s  %-10s  %-10s  %-12s  %-10s\n"

	// Table header
	sb.WriteString(fmt.Sprintf(headerFormat,
		"Priority", "Account Name", "Policy", "Account ID", "Current", "Average", "Peak", "Recommended", "Adjustment"))
	sb.WriteString(fmt.Sprintf(headerFormat,
		"--------", strings.Repeat("-", 30), strings.Repeat("-", 15), strings.Repeat("-", 14),
		strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 10),
		strings.Repeat("-", 12), strings.Repeat("-", 10)))

	// Table rows
	for _, rec := range recommendations {
		// Get plain text versions for width calculation
		priorityPlain := r.getPriorityPlain(rec.Priority)
		accountName := r.truncate(rec.AccountName, 30)
		policyName := r.truncate(rec.PolicyName, 15)
		if policyName == "" {
			policyName = "Default"
		}
		accountID := rec.AccountID
		current := r.formatCurrency(rec.CurrentBudget)
		average := r.formatCurrency(&rec.AverageSpend)
		peak := r.formatCurrency(&rec.PeakSpend)
		recommended := r.formatCurrency(&rec.RecommendedBudget)

		// Determine adjustment display based on budget access status
		var changePlain, changeColored string
		if rec.BudgetAccessStatus == types.BudgetAccessDenied {
			changePlain = "UNKNOWN"
			changeColored = color.YellowString("UNKNOWN")
		} else if rec.CurrentBudget == nil || *rec.CurrentBudget == 0 {
			changePlain = "NEW"
			changeColored = color.GreenString("NEW")
		} else {
			changePlain = r.formatChangePlain(rec.AdjustmentPercent)
			changeColored = r.formatChange(rec.AdjustmentPercent)
		}

		// Format with colors
		priorityColored := r.formatPriority(rec.Priority)

		// Calculate padding for colored fields
		priorityPadding := strings.Repeat(" ", max(0, 8-len(priorityPlain)))
		changePadding := strings.Repeat(" ", max(0, 10-len(changePlain)))

		sb.WriteString(fmt.Sprintf("%s%s  %-30s  %-15s  %-14s  %10s  %10s  %10s  %12s  %s%s\n",
			priorityColored, priorityPadding,
			accountName, policyName, accountID, current, average, peak, recommended,
			changeColored, changePadding))
	}

	// Summary
	sb.WriteString("\n")
	sb.WriteString(r.generateSummary(recommendations))
	sb.WriteString("\n")

	return sb.String(), nil
}

// GenerateJSONReport creates a JSON report
func (r *Reporter) GenerateJSONReport(recommendations []*types.BudgetRecommendation) (string, error) {
	result := map[string]interface{}{
		"timestamp":       time.Now().Format(time.RFC3339),
		"recommendations": recommendations,
		"summary": map[string]interface{}{
			"total":            len(recommendations),
			"high":             r.countByPriority(recommendations, types.PriorityHigh),
			"medium":           r.countByPriority(recommendations, types.PriorityMedium),
			"low":              r.countByPriority(recommendations, types.PriorityLow),
			"totalCurrent":     r.sumCurrentBudgets(recommendations),
			"totalRecommended": r.sumRecommendedBudgets(recommendations),
		},
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// OutputReport outputs the report based on options
func (r *Reporter) OutputReport(
	recommendations []*types.BudgetRecommendation,
	options types.ReportOptions,
) error {
	// Sort recommendations
	sorted := r.sortRecommendations(recommendations, options.SortBy)

	var output string
	var err error

	// If output file is specified but format is table, automatically use "both" format
	// This makes --output-file work intuitively without requiring --output-format
	format := options.Format
	if options.OutputFile != "" && format == types.FormatTable {
		format = types.FormatBoth
	}

	switch format {
	case types.FormatTable:
		output, err = r.GenerateTableReport(sorted)
		if err != nil {
			return err
		}
		fmt.Fprint(r.writer, output)

	case types.FormatJSON:
		output, err = r.GenerateJSONReport(sorted)
		if err != nil {
			return err
		}
		if options.OutputFile != "" {
			return r.writeToFile(output, options.OutputFile)
		}
		fmt.Fprint(r.writer, output)

	case types.FormatBoth:
		// Table to console
		tableOutput, err := r.GenerateTableReport(sorted)
		if err != nil {
			return err
		}
		fmt.Fprint(r.writer, tableOutput)

		// JSON to file
		jsonOutput, err := r.GenerateJSONReport(sorted)
		if err != nil {
			return err
		}
		if options.OutputFile != "" {
			return r.writeToFile(jsonOutput, options.OutputFile)
		}
	}

	return nil
}

// sortRecommendations sorts recommendations based on the sort option
func (r *Reporter) sortRecommendations(
	recommendations []*types.BudgetRecommendation,
	sortBy types.SortBy,
) []*types.BudgetRecommendation {
	sorted := make([]*types.BudgetRecommendation, len(recommendations))
	copy(sorted, recommendations)

	switch sortBy {
	case types.SortByPriority:
		sort.Slice(sorted, func(i, j int) bool {
			return r.priorityValue(sorted[i].Priority) > r.priorityValue(sorted[j].Priority)
		})
	case types.SortByAdjustment:
		sort.Slice(sorted, func(i, j int) bool {
			return math.Abs(sorted[i].AdjustmentPercent) > math.Abs(sorted[j].AdjustmentPercent)
		})
	case types.SortByAccount:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].AccountName < sorted[j].AccountName
		})
	}

	return sorted
}

// formatPriority formats priority with color
func (r *Reporter) formatPriority(priority types.Priority) string {
	switch priority {
	case types.PriorityHigh:
		return color.RedString("HIGH")
	case types.PriorityMedium:
		return color.YellowString("MEDIUM")
	case types.PriorityLow:
		return color.GreenString("LOW")
	default:
		return string(priority)
	}
}

// getPriorityPlain returns priority without color codes
func (r *Reporter) getPriorityPlain(priority types.Priority) string {
	switch priority {
	case types.PriorityHigh:
		return "HIGH"
	case types.PriorityMedium:
		return "MEDIUM"
	case types.PriorityLow:
		return "LOW"
	default:
		return string(priority)
	}
}

// formatCurrency formats a currency value
func (r *Reporter) formatCurrency(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("$%.0f", *value)
}

// formatChange formats the adjustment percentage with color
func (r *Reporter) formatChange(percent float64) string {
	sign := ""
	if percent > 0 {
		sign = "+"
	}

	formatted := fmt.Sprintf("%s%.1f%%", sign, percent)

	if percent > 50 {
		return color.RedString(formatted)
	} else if percent > 20 {
		return color.YellowString(formatted)
	} else if percent < -20 {
		return color.CyanString(formatted)
	}

	return formatted
}

// formatChangePlain returns change percentage without color codes
func (r *Reporter) formatChangePlain(percent float64) string {
	sign := ""
	if percent > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.1f%%", sign, percent)
}

// truncate truncates a string to a maximum length
func (r *Reporter) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// generateSummary generates a summary section
func (r *Reporter) generateSummary(recommendations []*types.BudgetRecommendation) string {
	var sb strings.Builder

	sb.WriteString(color.New(color.Bold).Sprint("Summary:"))
	sb.WriteString("\n")

	total := len(recommendations)
	high := r.countByPriority(recommendations, types.PriorityHigh)
	medium := r.countByPriority(recommendations, types.PriorityMedium)
	low := r.countByPriority(recommendations, types.PriorityLow)

	sb.WriteString(fmt.Sprintf("- Total accounts analyzed: %d\n", total))
	sb.WriteString(fmt.Sprintf("- High priority: %d\n", high))
	sb.WriteString(fmt.Sprintf("- Medium priority: %d\n", medium))
	sb.WriteString(fmt.Sprintf("- Low priority: %d\n", low))

	currentTotal := r.sumCurrentBudgets(recommendations)
	recommendedTotal := r.sumRecommendedBudgets(recommendations)

	if currentTotal > 0 {
		sb.WriteString(fmt.Sprintf("- Total current budgets: $%.0f\n", currentTotal))
		sb.WriteString(fmt.Sprintf("- Total recommended budgets: $%.0f\n", recommendedTotal))
		change := ((recommendedTotal - currentTotal) / currentTotal) * 100
		sb.WriteString(fmt.Sprintf("- Overall change: %+.1f%%\n", change))
	}

	return sb.String()
}

// countByPriority counts recommendations by priority
func (r *Reporter) countByPriority(recommendations []*types.BudgetRecommendation, priority types.Priority) int {
	count := 0
	for _, rec := range recommendations {
		if rec.Priority == priority {
			count++
		}
	}
	return count
}

// sumCurrentBudgets sums all current budgets
func (r *Reporter) sumCurrentBudgets(recommendations []*types.BudgetRecommendation) float64 {
	sum := 0.0
	for _, rec := range recommendations {
		if rec.CurrentBudget != nil {
			sum += *rec.CurrentBudget
		}
	}
	return sum
}

// sumRecommendedBudgets sums all recommended budgets
func (r *Reporter) sumRecommendedBudgets(recommendations []*types.BudgetRecommendation) float64 {
	sum := 0.0
	for _, rec := range recommendations {
		sum += rec.RecommendedBudget
	}
	return sum
}

// priorityValue returns a numeric value for priority (for sorting)
func (r *Reporter) priorityValue(priority types.Priority) int {
	switch priority {
	case types.PriorityHigh:
		return 3
	case types.PriorityMedium:
		return 2
	case types.PriorityLow:
		return 1
	default:
		return 0
	}
}

// writeToFile writes content to a file
// #nosec G304 - filename is from CLI flag provided by the user running the tool
func (r *Reporter) writeToFile(content, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filename, err)
	}

	fmt.Fprintf(r.writer, "\nReport written to: %s\n", filename)
	return nil
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
