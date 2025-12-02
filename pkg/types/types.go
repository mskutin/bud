package types

import "time"

// AccountInfo represents an AWS account
type AccountInfo struct {
	ID    string
	Alias string
	Email string
	Name  string
}

// MonthlyCost represents cost for a specific month
type MonthlyCost struct {
	Month  string
	Amount float64
}

// AccountCostData represents cost data for an account
type AccountCostData struct {
	AccountID    string
	AccountName  string
	MonthlyCosts []MonthlyCost
	Error        error
}

// BudgetAccessStatus represents the status of budget access
type BudgetAccessStatus string

const (
	BudgetAccessSuccess  BudgetAccessStatus = "success"       // Budget retrieved successfully
	BudgetAccessNotFound BudgetAccessStatus = "not_found"     // No budget exists
	BudgetAccessDenied   BudgetAccessStatus = "access_denied" // Access denied to budget
	BudgetAccessError    BudgetAccessStatus = "error"         // Other error
)

// BudgetConfig represents a budget configuration from AWS
type BudgetConfig struct {
	AccountID     string
	AccountName   string
	BudgetName    string
	LimitAmount   float64
	TimeUnit      string
	HasForecasted bool
	HasActual     bool
	Subscribers   []string
	AccessStatus  BudgetAccessStatus // Status of budget retrieval
	AccessError   error              // Error if retrieval failed
}

// Trend represents spending trend
type Trend string

const (
	TrendIncreasing Trend = "increasing"
	TrendDecreasing Trend = "decreasing"
	TrendStable     Trend = "stable"
)

// SpendStatistics represents calculated spending statistics
type SpendStatistics struct {
	AccountID           string
	AccountName         string
	AverageMonthlySpend float64
	PeakMonthlySpend    float64
	MinMonthlySpend     float64
	CurrentMonthSpend   *float64
	Trend               Trend
	MonthsAnalyzed      int
}

// BudgetStatus represents the status of a budget
type BudgetStatus string

const (
	StatusOverBudget    BudgetStatus = "over-budget"
	StatusUnderUtilized BudgetStatus = "under-utilized"
	StatusAppropriate   BudgetStatus = "appropriate"
	StatusNoBudget      BudgetStatus = "no-budget"
)

// BudgetComparison represents comparison between spend and budget
type BudgetComparison struct {
	AccountID          string
	AccountName        string
	CurrentBudget      *float64
	AverageSpend       float64
	PeakSpend          float64
	UtilizationPercent *float64
	Status             BudgetStatus
}

// Priority represents recommendation priority
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// BudgetRecommendation represents a budget recommendation
type BudgetRecommendation struct {
	AccountID          string
	AccountName        string
	CurrentBudget      *float64
	RecommendedBudget  float64
	AverageSpend       float64
	PeakSpend          float64
	AdjustmentPercent  float64
	Priority           Priority
	Justification      string
	BudgetAccessStatus BudgetAccessStatus // Status of budget access
	PolicyName         string             // Name of policy applied
}

// RecommendationPolicy defines policy for generating recommendations
type RecommendationPolicy struct {
	Name              string // Policy name for identification
	GrowthBuffer      float64
	MinimumBudget     float64
	RoundingIncrement float64
}

// OUPolicy defines budget policy for an Organizational Unit
type OUPolicy struct {
	OU                string  `yaml:"ou"`
	Name              string  `yaml:"name"`
	GrowthBuffer      float64 `yaml:"growthBuffer"`
	MinimumBudget     float64 `yaml:"minimumBudget"`
	RoundingIncrement float64 `yaml:"roundingIncrement"`
}

// AccountPolicy defines budget policy for a specific account
type AccountPolicy struct {
	Account           string  `yaml:"account"`
	Name              string  `yaml:"name"`
	GrowthBuffer      float64 `yaml:"growthBuffer"`
	MinimumBudget     float64 `yaml:"minimumBudget"`
	RoundingIncrement float64 `yaml:"roundingIncrement"`
}

// TagPolicy defines budget policy based on account tags
type TagPolicy struct {
	TagKey            string  `yaml:"tagKey"`
	TagValue          string  `yaml:"tagValue"`
	Name              string  `yaml:"name"`
	GrowthBuffer      float64 `yaml:"growthBuffer"`
	MinimumBudget     float64 `yaml:"minimumBudget"`
	RoundingIncrement float64 `yaml:"roundingIncrement"`
}

// PolicyConfig holds all policy configurations
type PolicyConfig struct {
	OUPolicies      []OUPolicy      `yaml:"ouPolicies"`
	AccountPolicies []AccountPolicy `yaml:"accountPolicies"`
	TagPolicies     []TagPolicy     `yaml:"tagPolicies"`
}

// AnalysisConfig represents configuration for analysis
type AnalysisConfig struct {
	AnalysisMonths        int
	GrowthBuffer          float64
	MinimumBudget         float64
	RoundingIncrement     float64
	AWSRegion             string
	CostExplorerRetries   int
	CostExplorerBackoffMs int
	Concurrency           int
}

// AnalysisError represents an error during analysis
type AnalysisError struct {
	AccountID   string
	AccountName string
	Error       error
}

// AnalysisResult represents the complete analysis result
type AnalysisResult struct {
	Timestamp              time.Time
	Config                 AnalysisConfig
	AccountsAnalyzed       int
	AccountsWithBudgets    int
	AccountsWithoutBudgets int
	Recommendations        []*BudgetRecommendation
	Errors                 []AnalysisError
}

// ReportFormat represents output format
type ReportFormat string

const (
	FormatTable ReportFormat = "table"
	FormatJSON  ReportFormat = "json"
	FormatBoth  ReportFormat = "both"
)

// SortBy represents sorting option
type SortBy string

const (
	SortByPriority   SortBy = "priority"
	SortByAdjustment SortBy = "adjustment"
	SortByAccount    SortBy = "account"
)

// ReportOptions represents options for report generation
type ReportOptions struct {
	Format     ReportFormat
	OutputFile string
	SortBy     SortBy
}
