package budgets

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/budgets"
	btypes "github.com/aws/aws-sdk-go-v2/service/budgets/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/mskutin/bud/pkg/types"
)

// Client wraps the AWS Budgets client
type Client struct {
	client         *budgets.Client
	config         *aws.Config
	assumeRoleName string // Optional role name to assume in child accounts
}

// NewClient creates a new Budgets client
func NewClient(cfg *aws.Config) *Client {
	return &Client{
		client: budgets.NewFromConfig(*cfg),
		config: cfg,
	}
}

// NewClientWithAssumeRole creates a new Budgets client with cross-account role assumption
func NewClientWithAssumeRole(cfg *aws.Config, assumeRoleName string) *Client {
	return &Client{
		client:         budgets.NewFromConfig(*cfg),
		config:         cfg,
		assumeRoleName: assumeRoleName,
	}
}

// getClientForAccount returns a budgets client for the specified account
// If assumeRoleName is set, it will assume that role in the target account
func (c *Client) getClientForAccount(ctx context.Context, accountID string) (*budgets.Client, error) {
	// If no role assumption is configured, use the default client
	if c.assumeRoleName == "" {
		return c.client, nil
	}

	// Build the role ARN
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, c.assumeRoleName)

	// Create STS client
	stsClient := sts.NewFromConfig(*c.config)

	// Create credentials provider that assumes the role
	creds := stscreds.NewAssumeRoleProvider(stsClient, roleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "bud"
	})

	// Create a new config with the assumed role credentials
	assumedConfig := c.config.Copy()
	assumedConfig.Credentials = aws.NewCredentialsCache(creds)

	// Return a new budgets client with the assumed role
	return budgets.NewFromConfig(assumedConfig), nil
}

// GetAccountBudgets retrieves all budgets for a single account
func (c *Client) GetAccountBudgets(
	ctx context.Context,
	accountID string,
	accountName string,
) ([]*types.BudgetConfig, error) {
	var budgetConfigs []*types.BudgetConfig

	// Get the appropriate client (with or without role assumption)
	client, err := c.getClientForAccount(ctx, accountID)
	if err != nil {
		return []*types.BudgetConfig{{
			AccountID:    accountID,
			AccountName:  accountName,
			AccessStatus: types.BudgetAccessError,
			AccessError:  fmt.Errorf("failed to assume role: %w", err),
		}}, nil
	}

	// List all budgets for the account
	input := &budgets.DescribeBudgetsInput{
		AccountId: aws.String(accountID),
	}

	paginator := budgets.NewDescribeBudgetsPaginator(client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			// Determine the type of error
			if isAccessDeniedError(err) {
				// Return a marker config indicating access denied
				return []*types.BudgetConfig{{
					AccountID:    accountID,
					AccountName:  accountName,
					AccessStatus: types.BudgetAccessDenied,
					AccessError:  err,
				}}, nil
			}
			if isNotFoundError(err) {
				// Return a marker config indicating no budget found
				return []*types.BudgetConfig{{
					AccountID:    accountID,
					AccountName:  accountName,
					AccessStatus: types.BudgetAccessNotFound,
				}}, nil
			}
			// Other error - return it
			return []*types.BudgetConfig{{
				AccountID:    accountID,
				AccountName:  accountName,
				AccessStatus: types.BudgetAccessError,
				AccessError:  err,
			}}, nil
		}

		for _, budget := range output.Budgets {
			config, err := c.parseBudgetConfig(ctx, client, accountID, accountName, budget)
			if err != nil {
				// Log error but continue with other budgets
				continue
			}
			config.AccessStatus = types.BudgetAccessSuccess
			budgetConfigs = append(budgetConfigs, config)
		}
	}

	// If no budgets found but no error, it means account has no budgets
	if len(budgetConfigs) == 0 {
		return []*types.BudgetConfig{{
			AccountID:    accountID,
			AccountName:  accountName,
			AccessStatus: types.BudgetAccessNotFound,
		}}, nil
	}

	return budgetConfigs, nil
}

// ProgressCallback is called after each account is processed
type ProgressCallback func()

// GetAllAccountsBudgets retrieves budgets for multiple accounts concurrently
func (c *Client) GetAllAccountsBudgets(
	ctx context.Context,
	accounts []types.AccountInfo,
	concurrency int,
) (map[string][]*types.BudgetConfig, error) {
	return c.GetAllAccountsBudgetsWithProgress(ctx, accounts, concurrency, nil)
}

// GetAllAccountsBudgetsWithProgress retrieves budgets with progress callback
func (c *Client) GetAllAccountsBudgetsWithProgress(
	ctx context.Context,
	accounts []types.AccountInfo,
	concurrency int,
	progressCallback ProgressCallback,
) (map[string][]*types.BudgetConfig, error) {
	results := make(map[string][]*types.BudgetConfig)
	var mu sync.Mutex

	// Create a worker pool
	jobs := make(chan int, len(accounts))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				account := accounts[idx]
				budgetConfigs, err := c.GetAccountBudgets(ctx, account.ID, account.Name)

				mu.Lock()
				if err != nil {
					// Store empty list on error
					results[account.ID] = []*types.BudgetConfig{}
				} else {
					results[account.ID] = budgetConfigs
				}
				mu.Unlock()

				// Call progress callback if provided
				if progressCallback != nil {
					progressCallback()
				}
			}
		}()
	}

	// Send jobs
	for i := range accounts {
		jobs <- i
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()

	return results, nil
}

// parseBudgetConfig converts AWS Budget to our BudgetConfig type
func (c *Client) parseBudgetConfig(
	ctx context.Context,
	client *budgets.Client,
	accountID string,
	accountName string,
	budget btypes.Budget,
) (*types.BudgetConfig, error) {
	config := &types.BudgetConfig{
		AccountID:   accountID,
		AccountName: accountName,
	}

	// Extract budget name
	if budget.BudgetName != nil {
		config.BudgetName = *budget.BudgetName
	}

	// Extract limit amount
	if budget.BudgetLimit != nil && budget.BudgetLimit.Amount != nil {
		// #nosec G104 - Sscanf error means LimitAmount stays 0.0, which is acceptable
		_, _ = fmt.Sscanf(*budget.BudgetLimit.Amount, "%f", &config.LimitAmount)
	}

	// Extract time unit
	config.TimeUnit = string(budget.TimeUnit)

	// Get notifications to check for FORECASTED and ACTUAL types
	notifInput := &budgets.DescribeNotificationsForBudgetInput{
		AccountId:  aws.String(accountID),
		BudgetName: budget.BudgetName,
	}

	notifOutput, err := client.DescribeNotificationsForBudget(ctx, notifInput)
	if err != nil {
		// If we can't get notifications, continue with what we have
		return config, nil
	}

	// Parse notifications
	subscribersMap := make(map[string]bool)
	for _, notification := range notifOutput.Notifications {
		// Check notification type
		if notification.NotificationType == btypes.NotificationTypeForecasted {
			config.HasForecasted = true
		}
		if notification.NotificationType == btypes.NotificationTypeActual {
			config.HasActual = true
		}

		// Get subscribers for this notification
		subsInput := &budgets.DescribeSubscribersForNotificationInput{
			AccountId:    aws.String(accountID),
			BudgetName:   budget.BudgetName,
			Notification: &notification,
		}

		subsOutput, err := client.DescribeSubscribersForNotification(ctx, subsInput)
		if err != nil {
			continue
		}

		for _, subscriber := range subsOutput.Subscribers {
			if subscriber.Address != nil {
				subscribersMap[*subscriber.Address] = true
			}
		}
	}

	// Convert subscribers map to slice
	for email := range subscribersMap {
		config.Subscribers = append(config.Subscribers, email)
	}

	return config, nil
}

// isAccessDeniedError checks if the error is an access denied error
func isAccessDeniedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "AccessDeniedException") || contains(errStr, "AccessDenied")
}

// isNotFoundError checks if the error indicates no budgets exist
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "NotFoundException") || contains(errStr, "NotFound")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
