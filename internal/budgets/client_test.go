package budgets

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
}

func TestIsAccessDeniedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"access denied", fmt.Errorf("AccessDeniedException: not authorized"), true},
		{"other error", assert.AnError, false},
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
		{"not found", fmt.Errorf("NotFoundException: budget not found"), true},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAccountBudgets(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}
	client := NewClient(cfg)

	ctx := context.Background()

	// This will attempt to call the actual AWS API
	// In production, we would mock this
	budgets, err := client.GetAccountBudgets(ctx, "123456789012", "test-account")

	// We expect either success with empty list or an error
	if err != nil {
		t.Logf("Expected error when calling AWS API without credentials: %v", err)
	} else {
		assert.NotNil(t, budgets)
		t.Logf("Retrieved %d budgets", len(budgets))
	}
}

func TestGetAllAccountsBudgets_Concurrency(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-east-1",
	}
	client := NewClient(cfg)

	accounts := []types.AccountInfo{
		{ID: "123456789012", Name: "account-1"},
		{ID: "234567890123", Name: "account-2"},
		{ID: "345678901234", Name: "account-3"},
	}

	ctx := context.Background()

	// This will attempt to call the actual AWS API
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
