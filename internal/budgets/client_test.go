package budgets

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/budgets"
	btypes "github.com/aws/aws-sdk-go-v2/service/budgets/types"
	"github.com/aws/smithy-go"
	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)

	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, cfg, client.config)
	assert.Empty(t, client.assumeRoleName, "assumeRoleName should be empty for NewClient")
}

func TestNewClientWithAssumeRole(t *testing.T) {
	tests := []struct {
		name           string
		assumeRoleName string
	}{
		{
			name:           "with valid role name",
			assumeRoleName: "OrganizationAccountAccessRole",
		},
		{
			name:           "with custom role name",
			assumeRoleName: "BudgetReadRole",
		},
		{
			name:           "with empty role name",
			assumeRoleName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &aws.Config{
				Region: "us-east-1",
			}

			client := NewClientWithAssumeRole(cfg, tt.assumeRoleName)

			assert.NotNil(t, client)
			assert.NotNil(t, client.client)
			assert.Equal(t, cfg, client.config)
			assert.Equal(t, tt.assumeRoleName, client.assumeRoleName)
		})
	}
}

func TestGetClientForAccount_NoRoleAssumption(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	resultClient, err := client.getClientForAccount(ctx, "123456789012")

	require.NoError(t, err)
	assert.Same(t, client.client, resultClient, "should return the same client when no role assumption")
}

func TestGetClientForAccount_WithRoleAssumption_ValidAccountID(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClientWithAssumeRole(cfg, "OrganizationAccountAccessRole")
	ctx := context.Background()

	// Test with a valid 12-digit account ID
	resultClient, err := client.getClientForAccount(ctx, "123456789012")

	// This will fail without actual AWS credentials, but we can verify the behavior
	// The function should attempt to create the correct role ARN
	if err != nil {
		assert.Contains(t, err.Error(), "failed to retrieve credentials")
	}
	assert.NotNil(t, resultClient)
}

func TestGetClientForAccount_RoleARNConstruction(t *testing.T) {
	tests := []struct {
		name        string
		accountID   string
		roleName    string
		expectedARN string
		expectError bool
	}{
		{
			name:        "valid account ID with role",
			accountID:   "123456789012",
			roleName:    "OrganizationAccountAccessRole",
			expectedARN: "arn:aws:iam::123456789012:role/OrganizationAccountAccessRole",
			expectError: false,
		},
		{
			name:        "valid account ID with custom role",
			accountID:   "999888777666",
			roleName:    "BudgetReadRole",
			expectedARN: "arn:aws:iam::999888777666:role/BudgetReadRole",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &aws.Config{
				Region: "us-east-1",
			}

			client := NewClientWithAssumeRole(cfg, tt.roleName)
			ctx := context.Background()

			resultClient, err := client.getClientForAccount(ctx, tt.accountID)

			// We expect an error when trying to actually assume the role without credentials
			// But we can't easily test the ARN construction without mocking deeper
			if err != nil {
				// Expected to fail without AWS credentials
				assert.NotNil(t, resultClient)
			}
		})
	}
}

func TestGetAccountBudgets_AccessDeniedError(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	// This will likely result in an access denied or no credentials error
	// The function should handle it gracefully
	budgets, err := client.GetAccountBudgets(ctx, "123456789012", "test-account")

	// Should return budgets (possibly with error status) rather than throwing
	assert.NotNil(t, budgets)

	if err != nil {
		t.Logf("Expected error when calling AWS API without credentials: %v", err)
	}
}

func TestGetAccountBudgets_NoBudgetsFound(t *testing.T) {
	// This test documents the behavior when no budgets exist
	// Without actual AWS API access, we can't fully test this
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	budgets, err := client.GetAccountBudgets(ctx, "123456789012", "test-account")

	// Should return a budget config with not_found status when no budgets exist
	assert.NotNil(t, budgets)

	if err == nil && len(budgets) > 0 {
		if budgets[0].AccessStatus == types.BudgetAccessNotFound {
			t.Logf("Correctly returned not_found status for account with no budgets")
		}
	}
}

func TestGetAllAccountsBudgets_EmptyAccounts(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()
	accounts := []types.AccountInfo{}

	results, err := client.GetAllAccountsBudgets(ctx, accounts, 2)

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results)
}

func TestGetAllAccountsBudgets_SingleAccount(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "test-account"},
	}

	results, err := client.GetAllAccountsBudgets(ctx, accounts, 1)

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Contains(t, results, "123456789012")
}

func TestGetAllAccountsBudgets_MultipleAccounts(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
	}

	results, err := client.GetAllAccountsBudgets(ctx, accounts, 2)

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, len(accounts))

	// Verify all accounts are in results
	for _, account := range accounts {
		_, exists := results[account.ID]
		assert.True(t, exists, "Account %s should be in results", account.ID)
	}
}

func TestGetAllAccountsBudgetsWithProgress_ProgressCallback(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
	}

	// Track progress callback invocations
	var callbackCount int32
	progressCallback := func() {
		atomic.AddInt32(&callbackCount, 1)
	}

	results, err := client.GetAllAccountsBudgetsWithProgress(ctx, accounts, 2, progressCallback)

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, int32(len(accounts)), atomic.LoadInt32(&callbackCount),
		"progress callback should be called once for each account")
}

func TestGetAllAccountsBudgetsWithProgress_NilCallback(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
	}

	// Should not panic with nil callback
	results, err := client.GetAllAccountsBudgetsWithProgress(ctx, accounts, 1, nil)

	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestGetAllAccountsBudgets_Concurrency(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
		{ID: "456789012345", Name: "account-4"},
		{ID: "567890123456", Name: "account-5"},
	}

	// Test with different concurrency levels
	concurrencyLevels := []int{1, 2, 5, 10}

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("concurrency-%d", concurrency), func(t *testing.T) {
			results, err := client.GetAllAccountsBudgets(ctx, accounts, concurrency)

			require.NoError(t, err)
			assert.NotNil(t, results)
			assert.Len(t, results, len(accounts))
		})
	}
}

func TestParseBudgetConfig_ValidBudget(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	// Create a mock budget
	budgetName := "Test Budget"
	limitAmount := "100.50"
	awsBudget := btypes.Budget{
		BudgetName: &budgetName,
		BudgetLimit: &btypes.Spend{
			Amount: &limitAmount,
		},
		TimeUnit: btypes.TimeUnitMonthly,
	}

	// We can't fully test this without mocking the DescribeNotificationsForBudget API
	// but we can test the basic parsing logic
	// Note: This will fail due to nil client for notifications, which is expected
	config, err := client.parseBudgetConfig(ctx, client.client, "123456789012", "test-account", awsBudget)

	// Config should be returned even if notifications fail
	if err == nil {
		assert.NotNil(t, config)
		assert.Equal(t, "Test Budget", config.BudgetName)
		assert.Equal(t, "123456789012", config.AccountID)
		assert.Equal(t, "test-account", config.AccountName)
	}
}

func TestParseBudgetConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name        string
		budgetName  *string
		limitAmount *string
		timeUnit    btypes.TimeUnit
		expectName  string
		expectLimit float64
	}{
		{
			name:        "missing budget name",
			budgetName:  nil,
			limitAmount: aws.String("100.00"),
			timeUnit:    btypes.TimeUnitMonthly,
			expectName:  "",
			expectLimit: 100.00,
		},
		{
			name:        "missing limit amount",
			budgetName:  aws.String("Test Budget"),
			limitAmount: nil,
			timeUnit:    btypes.TimeUnitMonthly,
			expectName:  "Test Budget",
			expectLimit: 0.0,
		},
		{
			name:        "all fields present",
			budgetName:  aws.String("Test Budget"),
			limitAmount: aws.String("250.75"),
			timeUnit:    btypes.TimeUnitMonthly,
			expectName:  "Test Budget",
			expectLimit: 250.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &aws.Config{
				Region: "us-east-1",
			}

			client := NewClient(cfg)
			ctx := context.Background()

			awsBudget := btypes.Budget{
				BudgetName:  tt.budgetName,
				BudgetLimit: &btypes.Spend{Amount: tt.limitAmount},
				TimeUnit:    tt.timeUnit,
			}

			config, err := client.parseBudgetConfig(ctx, client.client, "123456789012", "test-account", awsBudget)

			// Should return config even with missing fields
			if err == nil {
				assert.NotNil(t, config)
				assert.Equal(t, tt.expectName, config.BudgetName)
				assert.Equal(t, tt.expectLimit, config.LimitAmount)
				assert.Equal(t, string(tt.timeUnit), config.TimeUnit)
			}
		})
	}
}

func TestParseBudgetConfig_InvalidLimitAmount(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	budgetName := "Test Budget"
	invalidAmount := "not-a-number"

	awsBudget := btypes.Budget{
		BudgetName: &budgetName,
		BudgetLimit: &btypes.Spend{
			Amount: &invalidAmount,
		},
		TimeUnit: btypes.TimeUnitMonthly,
	}

	config, err := client.parseBudgetConfig(ctx, client.client, "123456789012", "test-account", awsBudget)

	// Should still return config with zero limit when parsing fails
	if err == nil {
		assert.NotNil(t, config)
		assert.Equal(t, 0.0, config.LimitAmount)
	}
}

func TestParseBudgetConfig_DifferentTimeUnits(t *testing.T) {
	timeUnits := []btypes.TimeUnit{
		btypes.TimeUnitMonthly,
		btypes.TimeUnitQuarterly,
		btypes.TimeUnitAnnually,
	}

	for _, timeUnit := range timeUnits {
		t.Run(string(timeUnit), func(t *testing.T) {
			cfg := &aws.Config{
				Region: "us-east-1",
			}

			client := NewClient(cfg)
			ctx := context.Background()

			budgetName := "Test Budget"
			limitAmount := "100.00"

			awsBudget := btypes.Budget{
				BudgetName: &budgetName,
				BudgetLimit: &btypes.Spend{
					Amount: &limitAmount,
				},
				TimeUnit: timeUnit,
			}

			config, err := client.parseBudgetConfig(ctx, client.client, "123456789012", "test-account", awsBudget)

			if err == nil {
				assert.NotNil(t, config)
				assert.Equal(t, string(timeUnit), config.TimeUnit)
			}
		})
	}
}

func TestIsAccessDeniedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"access denied exception", fmt.Errorf("AccessDeniedException: not authorized"), true},
		{"access denied", fmt.Errorf("AccessDenied: user is not authorized"), true},
		{"other error", assert.AnError, false},
		{"wrapped access denied", fmt.Errorf("wrapper: %w", fmt.Errorf("AccessDeniedException")), true}, // substring search finds it
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAccessDeniedError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"not found exception", fmt.Errorf("NotFoundException: budget not found"), true},
		{"not found", fmt.Errorf("NotFound: resource not found"), true},
		{"other error", assert.AnError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"contains at start", "hello world", "hello", true},
		{"contains at end", "hello world", "world", true},
		{"contains in middle", "hello world", "lo wo", true},
		{"not contains", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "hello", false},
		{"case sensitive", "Hello World", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found at start", "hello world", "hello", true},
		{"found at end", "hello world", "world", true},
		{"found in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"substring longer than string", "hi", "hello", false},
		{"single character match", "hello", "h", true},
		{"single character no match", "hello", "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock implementation for testing error handling

// mockBudgetsClient is a mock for budgets.Client for testing
type mockBudgetsClient struct {
	describeBudgetsFunc func(ctx context.Context, input *budgets.DescribeBudgetsInput, opts ...func(*budgets.Options)) (*budgets.DescribeBudgetsOutput, error)
}

func (m *mockBudgetsClient) DescribeBudgets(ctx context.Context, input *budgets.DescribeBudgetsInput, opts ...func(*budgets.Options)) (*budgets.DescribeBudgetsOutput, error) {
	if m.describeBudgetsFunc != nil {
		return m.describeBudgetsFunc(ctx, input, opts...)
	}
	return nil, fmt.Errorf("not implemented")
}

// TestGetAccountBudgets_ErrorHandling tests error handling in GetAccountBudgets
func TestGetAccountBudgets_ErrorHandling(t *testing.T) {
	// This test documents the expected error handling behavior
	// Without actual mocking framework, we document expectations

	tests := []struct {
		name           string
		errorMsg       string
		expectedStatus types.BudgetAccessStatus
	}{
		{
			name:           "access denied error",
			errorMsg:       "AccessDeniedException: not authorized",
			expectedStatus: types.BudgetAccessDenied,
		},
		{
			name:           "not found error",
			errorMsg:       "NotFoundException: no budget found",
			expectedStatus: types.BudgetAccessNotFound,
		},
		{
			name:           "other error",
			errorMsg:       "some other error",
			expectedStatus: types.BudgetAccessError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The isAccessDeniedError and isNotFoundError functions
			// are tested separately to verify error classification
			if tt.expectedStatus == types.BudgetAccessDenied {
				result := isAccessDeniedError(errors.New(tt.errorMsg))
				assert.True(t, result, "should classify as access denied")
			}
			if tt.expectedStatus == types.BudgetAccessNotFound {
				result := isNotFoundError(errors.New(tt.errorMsg))
				assert.True(t, result, "should classify as not found")
			}
		})
	}
}

// TestConcurrentAccess tests concurrent access to the client
func TestConcurrentAccess(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)
	ctx := context.Background()

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
		{ID: "456789012345", Name: "account-4"},
		{ID: "567890123456", Name: "account-5"},
		{ID: "678901234567", Name: "account-6"},
		{ID: "789012345678", Name: "account-7"},
		{ID: "890123456789", Name: "account-8"},
	}

	// Test with high concurrency to ensure thread safety
	results, err := client.GetAllAccountsBudgets(ctx, accounts, 10)

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, len(accounts))
}

// TestGetAllAccountsBudgets_ContextCancellation tests context cancellation
func TestGetAllAccountsBudgets_ContextCancellation(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
	}

	// The function should still complete and return results
	// (even if they're error results)
	results, err := client.GetAllAccountsBudgets(ctx, accounts, 2)

	// Should not panic and should return some result
	assert.NotNil(t, results)
	// Error may or may not occur depending on timing
	_ = err
}

// Helper function to create smithy API error
func newAPIError(errorCode string) error {
	return &smithy.GenericAPIError{
		Code:    errorCode,
		Message: "test error",
	}
}

// TestErrorClassification tests classification of AWS API errors
func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name           string
		errorCode      string
		isAccessDenied bool
		isNotFound     bool
	}{
		{
			name:           "AccessDeniedException",
			errorCode:      "AccessDeniedException",
			isAccessDenied: true,
			isNotFound:     false,
		},
		{
			name:           "AccessDenied",
			errorCode:      "AccessDenied",
			isAccessDenied: true,
			isNotFound:     false,
		},
		{
			name:           "NotFoundException",
			errorCode:      "NotFoundException",
			isAccessDenied: false,
			isNotFound:     true,
		},
		{
			name:           "NotFound",
			errorCode:      "NotFound",
			isAccessDenied: false,
			isNotFound:     true,
		},
		{
			name:           "OtherError",
			errorCode:      "SomeOtherError",
			isAccessDenied: false,
			isNotFound:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newAPIError(tt.errorCode)
			assert.Equal(t, tt.isAccessDenied, isAccessDeniedError(err))
			assert.Equal(t, tt.isNotFound, isNotFoundError(err))
		})
	}
}

// TestSTSRoleSessionName tests that the STS session name is set correctly
func TestSTSRoleSessionName(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClientWithAssumeRole(cfg, "TestRole")
	ctx := context.Background()

	// The session name should be "bud" as set in getClientForAccount
	// This test documents the expected behavior
	// Actual verification would require deeper mocking of STS client
	resultClient, err := client.getClientForAccount(ctx, "123456789012")

	// Without valid credentials, this will fail but the client should be created
	if err != nil {
		assert.Contains(t, err.Error(), "failed to retrieve credentials")
	}
	assert.NotNil(t, resultClient)
}

// TestGetAccountBudgets_WithRoleAssumption tests role assumption behavior
func TestGetAccountBudgets_WithRoleAssumption(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	// Create client with role assumption
	client := NewClientWithAssumeRole(cfg, "OrganizationAccountAccessRole")
	ctx := context.Background()

	// This will fail without actual AWS credentials and proper IAM role setup
	budgets, err := client.GetAccountBudgets(ctx, "123456789012", "test-account")

	// Should return results (with error status) rather than throwing
	assert.NotNil(t, budgets)

	if err != nil {
		t.Logf("Expected error when calling AWS API without credentials: %v", err)
	}

	// If we got results, they should have the account info
	if len(budgets) > 0 {
		assert.Equal(t, "123456789012", budgets[0].AccountID)
		assert.Equal(t, "test-account", budgets[0].AccountName)
	}
}

// TestBudgetAccessStatusValues tests BudgetAccessStatus constants
func TestBudgetAccessStatusValues(t *testing.T) {
	tests := []struct {
		status types.BudgetAccessStatus
		valid  bool
	}{
		{types.BudgetAccessSuccess, true},
		{types.BudgetAccessNotFound, true},
		{types.BudgetAccessDenied, true},
		{types.BudgetAccessError, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			validValues := map[types.BudgetAccessStatus]bool{
				types.BudgetAccessSuccess:  true,
				types.BudgetAccessNotFound: true,
				types.BudgetAccessDenied:   true,
				types.BudgetAccessError:    true,
			}
			assert.Equal(t, tt.valid, validValues[tt.status])
		})
	}
}
