package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/mskutin/bud/internal/analyzer"
	"github.com/mskutin/bud/internal/budgets"
	"github.com/mskutin/bud/internal/costexplorer"
	"github.com/mskutin/bud/internal/policy"
	"github.com/mskutin/bud/internal/recommender"
	"github.com/mskutin/bud/internal/reporter"
	"github.com/mskutin/bud/pkg/types"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information (set via ldflags during build)
	version = "dev"
	commit  = "none"
	date    = "unknown"

	cfgFile string

	// CLI flags
	analysisMonths    int
	growthBuffer      float64
	outputFormat      string
	outputFile        string
	accountFilter     []string
	ouFilter          []string // Organizational Unit IDs to filter
	awsRegion         string
	awsProfile        string
	minimumBudget     float64
	roundingIncrement float64
	concurrency       int
	assumeRoleName    string // Role name to assume in child accounts
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:     "bud",
	Version: version,
	Short:   "Your AWS budget buddy - smart recommendations based on spending patterns",
	Long: `Bud analyzes historical AWS spending patterns across all accounts in 
an AWS Organization and generates intelligent recommendations for budget 
configurations.

The tool retrieves actual spend data from AWS Cost Explorer and compares 
it against configured budgets to identify accounts with misaligned budget 
settings.`,
	RunE: runAnalysis,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Set custom version template
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
commit: {{.Annotations.commit}}
built: {{.Annotations.date}}
`)
	rootCmd.Annotations = map[string]string{
		"commit": commit,
		"date":   date,
	}

	// Configuration file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .bud.yaml)")

	// Analysis options
	rootCmd.Flags().IntVar(&analysisMonths, "analysis-months", 3, "Number of months to analyze")
	rootCmd.Flags().Float64Var(&growthBuffer, "growth-buffer", 20, "Growth buffer percentage above peak spend")
	rootCmd.Flags().Float64Var(&minimumBudget, "minimum-budget", 10, "Minimum budget for any account (USD)")
	rootCmd.Flags().Float64Var(&roundingIncrement, "rounding-increment", 10, "Round budget to nearest increment (USD)")

	// Output options
	rootCmd.Flags().StringVar(&outputFormat, "output-format", "table", "Output format: table, json, or both")
	rootCmd.Flags().StringVar(&outputFile, "output-file", "", "Output file path for JSON export")

	// AWS options
	rootCmd.Flags().StringVar(&awsRegion, "aws-region", "us-east-1", "AWS region")
	rootCmd.Flags().StringVar(&awsProfile, "aws-profile", "", "AWS profile to use")
	rootCmd.Flags().StringSliceVar(&accountFilter, "accounts", []string{}, "Filter specific account IDs (comma-separated)")
	rootCmd.Flags().StringSliceVar(&ouFilter, "organizational-units", []string{}, "Filter by Organizational Unit IDs (comma-separated, e.g., ou-xxxx-yyyyyyyy)")

	// Performance options
	rootCmd.Flags().IntVar(&concurrency, "concurrency", 5, "Number of concurrent API calls")

	// Cross-account options
	rootCmd.Flags().StringVar(&assumeRoleName, "assume-role-name", "", "Role name to assume in child accounts for budget access (e.g., OrganizationAccountAccessRole)")

	// Bind flags to viper
	// #nosec G104 - BindPFlag errors only occur if flag doesn't exist, which can't happen here
	_ = viper.BindPFlag("analysisMonths", rootCmd.Flags().Lookup("analysis-months"))
	_ = viper.BindPFlag("growthBuffer", rootCmd.Flags().Lookup("growth-buffer"))
	_ = viper.BindPFlag("minimumBudget", rootCmd.Flags().Lookup("minimum-budget"))
	_ = viper.BindPFlag("roundingIncrement", rootCmd.Flags().Lookup("rounding-increment"))
	_ = viper.BindPFlag("outputFormat", rootCmd.Flags().Lookup("output-format"))
	_ = viper.BindPFlag("outputFile", rootCmd.Flags().Lookup("output-file"))
	_ = viper.BindPFlag("awsRegion", rootCmd.Flags().Lookup("aws-region"))
	_ = viper.BindPFlag("awsProfile", rootCmd.Flags().Lookup("aws-profile"))
	_ = viper.BindPFlag("accounts", rootCmd.Flags().Lookup("accounts"))
	_ = viper.BindPFlag("organizationalUnits", rootCmd.Flags().Lookup("organizational-units"))
	_ = viper.BindPFlag("concurrency", rootCmd.Flags().Lookup("concurrency"))
	_ = viper.BindPFlag("assumeRoleName", rootCmd.Flags().Lookup("assume-role-name"))
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".bud")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("BUD")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err != nil {
		// If user explicitly specified a config file, fail
		if cfgFile != "" {
			fmt.Fprintf(os.Stderr, "Error: unable to read config file '%s': %v\n", cfgFile, err)
			os.Exit(1)
		}
		// Otherwise, it's optional - continue without config file
	} else {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// runAnalysis is the main entry point for the analysis
func runAnalysis(cmd *cobra.Command, args []string) error {
	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Print banner
	fmt.Println("Bud - Your AWS Budget Buddy")
	fmt.Println("===========================")
	fmt.Println()

	// Build configuration
	cfg := types.AnalysisConfig{
		AnalysisMonths:        viper.GetInt("analysisMonths"),
		GrowthBuffer:          viper.GetFloat64("growthBuffer"),
		MinimumBudget:         viper.GetFloat64("minimumBudget"),
		RoundingIncrement:     viper.GetFloat64("roundingIncrement"),
		AWSRegion:             viper.GetString("awsRegion"),
		CostExplorerRetries:   3,
		CostExplorerBackoffMs: 1000,
		Concurrency:           viper.GetInt("concurrency"),
	}

	// Display configuration
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Analysis Period: %d months\n", cfg.AnalysisMonths)
	fmt.Printf("  Growth Buffer: %.1f%%\n", cfg.GrowthBuffer)
	fmt.Printf("  Minimum Budget: $%.2f\n", cfg.MinimumBudget)
	fmt.Printf("  Rounding Increment: $%.2f\n", cfg.RoundingIncrement)
	fmt.Printf("  AWS Region: %s\n", cfg.AWSRegion)
	fmt.Printf("  Concurrency: %d\n", cfg.Concurrency)

	// Display cross-account role if configured
	if assumeRoleConfig := viper.GetString("assumeRoleName"); assumeRoleConfig != "" {
		fmt.Printf("  Cross-Account Role: %s\n", assumeRoleConfig)
	}

	// Display account filters if configured
	if accountFilters := viper.GetStringSlice("accounts"); len(accountFilters) > 0 {
		fmt.Printf("  Account Filter: %d account(s)\n", len(accountFilters))
	}

	if ouFilters := viper.GetStringSlice("organizationalUnits"); len(ouFilters) > 0 {
		fmt.Printf("  OU Filter: %d OU(s)\n", len(ouFilters))
	}
	fmt.Println()

	// Load AWS configuration
	awsCfg, err := loadAWSConfig(ctx, cfg.AWSRegion, viper.GetString("awsProfile"))
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Discover accounts
	fmt.Println("Discovering AWS accounts...")
	accounts, err := discoverAccounts(ctx, awsCfg)
	if err != nil {
		return fmt.Errorf("failed to discover accounts: %w", err)
	}

	fmt.Printf("Found %d account(s) in organization\n", len(accounts))

	// Apply OU filter if specified
	ouFilterList := viper.GetStringSlice("organizationalUnits")
	if len(ouFilterList) > 0 {
		accounts, err = filterAccountsByOU(ctx, awsCfg, accounts, ouFilterList)
		if err != nil {
			return fmt.Errorf("failed to filter by OU: %w", err)
		}
		fmt.Printf("After OU filter: %d account(s)\n", len(accounts))
	}

	// Apply account filter if specified
	accountFilterList := viper.GetStringSlice("accounts")
	if len(accountFilterList) > 0 {
		accounts = filterAccounts(accounts, accountFilterList)
		fmt.Printf("After account filter: %d account(s)\n", len(accounts))
	}

	fmt.Printf("Analyzing %d account(s)\n", len(accounts))
	fmt.Println()

	if len(accounts) == 0 {
		return fmt.Errorf("no accounts to analyze")
	}

	// Create policy resolver
	defaultPolicy := types.RecommendationPolicy{
		Name:              "Default",
		GrowthBuffer:      cfg.GrowthBuffer,
		MinimumBudget:     cfg.MinimumBudget,
		RoundingIncrement: cfg.RoundingIncrement,
	}

	// Load policy configuration
	policyConfig := types.PolicyConfig{}
	// #nosec G104 - UnmarshalKey errors are handled by using zero values
	_ = viper.UnmarshalKey("ouPolicies", &policyConfig.OUPolicies)
	_ = viper.UnmarshalKey("accountPolicies", &policyConfig.AccountPolicies)
	_ = viper.UnmarshalKey("tagPolicies", &policyConfig.TagPolicies)

	// Print policy configuration if any policies are defined
	if len(policyConfig.OUPolicies) > 0 {
		fmt.Printf("  OU Policies: %d configured\n", len(policyConfig.OUPolicies))
	}
	if len(policyConfig.AccountPolicies) > 0 {
		fmt.Printf("  Account Policies: %d configured\n", len(policyConfig.AccountPolicies))
	}
	if len(policyConfig.TagPolicies) > 0 {
		fmt.Printf("  Tag Policies: %d configured\n", len(policyConfig.TagPolicies))
	}

	resolver := policy.NewResolver(policyConfig, defaultPolicy)

	// Validate configured OUs exist
	ouIDsToValidate := make([]string, 0)
	for _, ouPolicy := range policyConfig.OUPolicies {
		ouIDsToValidate = append(ouIDsToValidate, ouPolicy.OU)
	}
	if len(ouIDsToValidate) > 0 {
		fmt.Printf("Validating %d configured OU(s)...\n", len(ouIDsToValidate))
		if err := policy.ValidateOUs(ctx, awsCfg, ouIDsToValidate); err != nil {
			return fmt.Errorf("policy configuration error: %w", err)
		}
	}

	// Load account metadata for policy resolution (only if needed)
	needsMetadata := len(policyConfig.OUPolicies) > 0 || len(policyConfig.TagPolicies) > 0
	if needsMetadata {
		metadataTypes := []string{}
		if len(policyConfig.OUPolicies) > 0 {
			metadataTypes = append(metadataTypes, "OU membership")
		}
		if len(policyConfig.TagPolicies) > 0 {
			metadataTypes = append(metadataTypes, "tags")
		}
		fmt.Printf("Loading account metadata (%s)...\n", strings.Join(metadataTypes, ", "))
		if err := resolver.LoadAccountMetadata(ctx, awsCfg, accounts); err != nil {
			return fmt.Errorf("failed to load account metadata: %w", err)
		}
	}
	fmt.Println()

	// Calculate date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, -cfg.AnalysisMonths, 0)

	// Initialize clients
	costClient := costexplorer.NewClient(&awsCfg, cfg.CostExplorerRetries, cfg.CostExplorerBackoffMs)

	// Create budget client with optional role assumption
	var budgetClient *budgets.Client
	assumeRole := viper.GetString("assumeRoleName")
	if assumeRole != "" {
		budgetClient = budgets.NewClientWithAssumeRole(&awsCfg, assumeRole)
	} else {
		budgetClient = budgets.NewClient(&awsCfg)
	}
	analyzer := &analyzer.Analyzer{}
	recommender := recommender.NewRecommender(defaultPolicy)

	// Fetch cost data
	fmt.Println("Fetching cost data from AWS Cost Explorer...")
	costBar := progressbar.Default(int64(len(accounts)), "Fetching costs")
	costData, err := costClient.GetAllAccountsCostsWithProgress(ctx, accounts, startDate, endDate, cfg.Concurrency, func() {
		_ = costBar.Add(1) // #nosec G104 - progress bar errors are cosmetic
	})
	if err != nil {
		return fmt.Errorf("failed to fetch cost data: %w", err)
	}
	_ = costBar.Finish() // #nosec G104 - progress bar errors are cosmetic
	fmt.Println()

	// Fetch budget data
	fmt.Println("Fetching budget configurations from AWS Budgets...")
	budgetBar := progressbar.Default(int64(len(accounts)), "Fetching budgets")
	budgetData, err := budgetClient.GetAllAccountsBudgetsWithProgress(ctx, accounts, cfg.Concurrency, func() {
		_ = budgetBar.Add(1) // #nosec G104 - progress bar errors are cosmetic
	})
	if err != nil {
		return fmt.Errorf("failed to fetch budget data: %w", err)
	}
	_ = budgetBar.Finish() // #nosec G104 - progress bar errors are cosmetic
	fmt.Println()

	// Analyze and generate recommendations
	fmt.Println("Analyzing spending patterns and generating recommendations...")
	result := &types.AnalysisResult{
		Timestamp:       time.Now(),
		Config:          cfg,
		Recommendations: make([]*types.BudgetRecommendation, 0),
		Errors:          make([]types.AnalysisError, 0),
	}

	for _, cost := range costData {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("analysis cancelled")
		default:
		}

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

		// Get budget for this account
		var budgetConfig *types.BudgetConfig
		var budgetAccessStatus types.BudgetAccessStatus = types.BudgetAccessNotFound

		if budgets, ok := budgetData[cost.AccountID]; ok && len(budgets) > 0 {
			budgetConfig = budgets[0] // Use first budget
			budgetAccessStatus = budgetConfig.AccessStatus

			// Only count as "with budget" if we successfully retrieved it
			if budgetAccessStatus == types.BudgetAccessSuccess {
				result.AccountsWithBudgets++
			} else {
				result.AccountsWithoutBudgets++
			}
		} else {
			result.AccountsWithoutBudgets++
		}

		// Compare to budget
		comparison, err := analyzer.CompareToBudget(stats, budgetConfig)
		if err != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       err,
			})
			continue
		}

		// Resolve policy for this account
		accountPolicy := resolver.ResolvePolicy(cost.AccountID)

		// Generate recommendation with account-specific policy
		recommendation, err := recommender.GenerateRecommendationWithPolicy(comparison, stats, accountPolicy)
		if err != nil {
			result.Errors = append(result.Errors, types.AnalysisError{
				AccountID:   cost.AccountID,
				AccountName: cost.AccountName,
				Error:       err,
			})
			continue
		}

		// Set the budget access status
		recommendation.BudgetAccessStatus = budgetAccessStatus

		result.Recommendations = append(result.Recommendations, recommendation)
		result.AccountsAnalyzed++
	}

	// Prioritize recommendations
	result.Recommendations = recommender.PrioritizeRecommendations(result.Recommendations)

	fmt.Printf("Analysis complete: %d accounts analyzed, %d errors\n", result.AccountsAnalyzed, len(result.Errors))
	fmt.Println()

	// Generate and output report
	outputFormat := types.ReportFormat(viper.GetString("outputFormat"))
	reportOptions := types.ReportOptions{
		Format:     outputFormat,
		OutputFile: viper.GetString("outputFile"),
		SortBy:     types.SortByAdjustment,
	}

	rep := reporter.NewReporter(os.Stdout)
	if err := rep.OutputReport(result.Recommendations, reportOptions); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors encountered:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s (%s): %v\n", e.AccountName, e.AccountID, e.Error)
		}
	}

	return nil
}

// loadAWSConfig loads AWS SDK configuration
func loadAWSConfig(ctx context.Context, region, profile string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	return cfg, nil
}

// discoverAccounts discovers all active accounts in the AWS Organization
func discoverAccounts(ctx context.Context, cfg aws.Config) ([]types.AccountInfo, error) {
	client := organizations.NewFromConfig(cfg)

	input := &organizations.ListAccountsInput{}
	accounts := make([]types.AccountInfo, 0)

	paginator := organizations.NewListAccountsPaginator(client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}

		for _, account := range output.Accounts {
			// Only include active accounts
			if account.Status == "ACTIVE" {
				name := ""
				if account.Name != nil {
					name = *account.Name
				}
				email := ""
				if account.Email != nil {
					email = *account.Email
				}
				id := ""
				if account.Id != nil {
					id = *account.Id
				}

				accounts = append(accounts, types.AccountInfo{
					ID:    id,
					Name:  name,
					Email: email,
					Alias: name, // Use name as alias
				})
			}
		}
	}

	return accounts, nil
}

// filterAccounts filters accounts by ID
func filterAccounts(accounts []types.AccountInfo, filter []string) []types.AccountInfo {
	if len(filter) == 0 {
		return accounts
	}

	filterMap := make(map[string]bool)
	for _, id := range filter {
		filterMap[id] = true
	}

	filtered := make([]types.AccountInfo, 0)
	for _, account := range accounts {
		if filterMap[account.ID] {
			filtered = append(filtered, account)
		}
	}

	return filtered
}

// filterAccountsByOU filters accounts by Organizational Unit
func filterAccountsByOU(ctx context.Context, cfg aws.Config, accounts []types.AccountInfo, ouIDs []string) ([]types.AccountInfo, error) {
	if len(ouIDs) == 0 {
		return accounts, nil
	}

	client := organizations.NewFromConfig(cfg)

	// Get all accounts in the specified OUs
	accountsInOUs := make(map[string]bool)

	for _, ouID := range ouIDs {
		// List accounts for this OU (non-recursive)
		input := &organizations.ListAccountsForParentInput{
			ParentId: aws.String(ouID),
		}

		paginator := organizations.NewListAccountsForParentPaginator(client, input)
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list accounts for OU %s: %w", ouID, err)
			}

			for _, account := range output.Accounts {
				if account.Id != nil && account.Status == "ACTIVE" {
					accountsInOUs[*account.Id] = true
				}
			}
		}
	}

	// Filter accounts to only those in the specified OUs
	filtered := make([]types.AccountInfo, 0)
	for _, account := range accounts {
		if accountsInOUs[account.ID] {
			filtered = append(filtered, account)
		}
	}

	return filtered, nil
}
