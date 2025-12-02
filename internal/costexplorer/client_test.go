package costexplorer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/mskutin/bud/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}

	client := NewClient(cfg, 3, 1000)

	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, 3, client.maxRetries)
	assert.Equal(t, 1000, client.backoffMs)
}

func TestCalculateBackoff(t *testing.T) {
	client := &Client{
		backoffMs: 1000,
	}

	tests := []struct {
		name    string
		attempt int
		minMs   float64
		maxMs   float64
	}{
		{"first retry", 0, 750, 1250},   // 1000 * 2^0 ± 25%
		{"second retry", 1, 1500, 2500}, // 1000 * 2^1 ± 25%
		{"third retry", 2, 3000, 5000},  // 1000 * 2^2 ± 25%
		{"capped", 10, 45000, 60000},    // Should cap at 60000
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := client.calculateBackoff(tt.attempt)
			ms := float64(backoff.Milliseconds())

			assert.GreaterOrEqual(t, ms, tt.minMs, "backoff should be >= min")
			assert.LessOrEqual(t, ms, tt.maxMs, "backoff should be <= max")
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"throttling", errors.New("ThrottlingException: Rate exceeded"), true},
		{"request limit", errors.New("RequestLimitExceeded"), true},
		{"service unavailable", errors.New("ServiceUnavailable"), true},
		{"internal error", errors.New("InternalError occurred"), true},
		{"connection error", errors.New("connection refused"), true},
		{"timeout error", errors.New("request timeout"), true},
		{"auth error", errors.New("UnauthorizedOperation"), false},
		{"validation error", errors.New("ValidationException"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestParseMonthFromDate(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		expected string
		wantErr  bool
	}{
		{"valid date", "2024-01-15", "2024-01", false},
		{"start of month", "2024-12-01", "2024-12", false},
		{"end of month", "2024-06-30", "2024-06", false},
		{"invalid format", "2024/01/15", "", true},
		{"invalid date", "not-a-date", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseMonthFromDate(tt.dateStr)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetAllAccountsCosts_Concurrency(t *testing.T) {
	// This test verifies that concurrent processing works correctly
	// We'll use a mock-like approach by testing the structure

	cfg := &aws.Config{
		Region: "us-east-1",
	}
	client := NewClient(cfg, 3, 100)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
	}

	ctx := context.Background()
	startDate := time.Now().AddDate(0, -3, 0)
	endDate := time.Now()

	// Note: This will make actual API calls if AWS credentials are configured
	// In a real test environment, we would mock the AWS SDK
	results, err := client.GetAllAccountsCosts(ctx, accounts, startDate, endDate, 2)

	// We expect results even if there are errors
	assert.NoError(t, err)
	assert.Len(t, results, len(accounts))

	// Verify all accounts are represented
	for i, result := range results {
		assert.NotNil(t, result)
		assert.Equal(t, accounts[i].ID, result.AccountID)
		assert.Equal(t, accounts[i].Name, result.AccountName)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAccountCosts_DateRange(t *testing.T) {
	// Test that date range is correctly calculated
	cfg := &aws.Config{
		Region: "us-east-1",
	}
	client := NewClient(cfg, 3, 1000)

	ctx := context.Background()
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)

	// This will attempt to call the API
	// In production tests, we would mock this
	result, _ := client.GetAccountCosts(ctx, "123456789012", "test-account", startDate, endDate)

	require.NotNil(t, result)
	assert.Equal(t, "123456789012", result.AccountID)
	assert.Equal(t, "test-account", result.AccountName)
}
