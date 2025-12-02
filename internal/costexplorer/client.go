package costexplorer

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/mskutin/bud/pkg/types"
)

// Client wraps the AWS Cost Explorer client
type Client struct {
	client     *costexplorer.Client
	config     *aws.Config
	maxRetries int
	backoffMs  int
}

// NewClient creates a new Cost Explorer client
func NewClient(cfg *aws.Config, maxRetries, backoffMs int) *Client {
	return &Client{
		client:     costexplorer.NewFromConfig(*cfg),
		config:     cfg,
		maxRetries: maxRetries,
		backoffMs:  backoffMs,
	}
}

// GetAccountCosts retrieves cost data for a single account
func (c *Client) GetAccountCosts(
	ctx context.Context,
	accountID string,
	accountName string,
	startDate, endDate time.Time,
) (*types.AccountCostData, error) {
	result := &types.AccountCostData{
		AccountID:    accountID,
		AccountName:  accountName,
		MonthlyCosts: []types.MonthlyCost{},
	}

	// Format dates for Cost Explorer API (YYYY-MM-DD)
	start := startDate.Format("2006-01-02")
	end := endDate.Format("2006-01-02")

	// Build the Cost Explorer request
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter: &cetypes.Expression{
			Dimensions: &cetypes.DimensionValues{
				Key:    cetypes.DimensionLinkedAccount,
				Values: []string{accountID},
			},
		},
	}

	// Execute with retry logic
	var resp *costexplorer.GetCostAndUsageOutput
	var err error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err = c.client.GetCostAndUsage(ctx, input)

		if err == nil {
			break
		}

		// Check if we should retry
		if attempt < c.maxRetries && isRetryableError(err) {
			backoff := c.calculateBackoff(attempt)
			time.Sleep(backoff)
			continue
		}

		// Non-retryable error or max retries exceeded
		result.Error = fmt.Errorf("failed to get cost data after %d attempts: %w", attempt+1, err)
		return result, result.Error
	}

	// Parse the response
	for _, resultByTime := range resp.ResultsByTime {
		if resultByTime.TimePeriod == nil || resultByTime.TimePeriod.Start == nil {
			continue
		}

		// Extract month in YYYY-MM format
		month, err := parseMonthFromDate(*resultByTime.TimePeriod.Start)
		if err != nil {
			continue
		}

		// Extract cost amount
		amount := 0.0
		if resultByTime.Total != nil {
			if metric, ok := resultByTime.Total["UnblendedCost"]; ok {
				if metric.Amount != nil {
					fmt.Sscanf(*metric.Amount, "%f", &amount)
				}
			}
		}

		result.MonthlyCosts = append(result.MonthlyCosts, types.MonthlyCost{
			Month:  month,
			Amount: amount,
		})
	}

	return result, nil
}

// ProgressCallback is called after each account is processed
type ProgressCallback func()

// GetAllAccountsCosts retrieves cost data for multiple accounts concurrently
func (c *Client) GetAllAccountsCosts(
	ctx context.Context,
	accounts []types.AccountInfo,
	startDate, endDate time.Time,
	concurrency int,
) ([]*types.AccountCostData, error) {
	return c.GetAllAccountsCostsWithProgress(ctx, accounts, startDate, endDate, concurrency, nil)
}

// GetAllAccountsCostsWithProgress retrieves cost data with progress callback
func (c *Client) GetAllAccountsCostsWithProgress(
	ctx context.Context,
	accounts []types.AccountInfo,
	startDate, endDate time.Time,
	concurrency int,
	progressCallback ProgressCallback,
) ([]*types.AccountCostData, error) {
	results := make([]*types.AccountCostData, len(accounts))

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
				costData, err := c.GetAccountCosts(
					ctx,
					account.ID,
					account.Name,
					startDate,
					endDate,
				)
				if err != nil {
					// Error is already set in costData.Error
					costData = &types.AccountCostData{
						AccountID:   account.ID,
						AccountName: account.Name,
						Error:       err,
					}
				}
				results[idx] = costData

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

// calculateBackoff calculates exponential backoff with jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: baseMs * 2^attempt
	backoffMs := float64(c.backoffMs) * math.Pow(2, float64(attempt))

	// Cap at 60 seconds
	if backoffMs > 60000 {
		backoffMs = 60000
	}

	// Add jitter (±25%)
	jitter := backoffMs * 0.25
	backoffMs = backoffMs - jitter + (2 * jitter * float64(time.Now().UnixNano()%1000) / 1000)

	return time.Duration(backoffMs) * time.Millisecond
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable errors
	errStr := err.Error()

	// Rate limiting
	if contains(errStr, "ThrottlingException") || contains(errStr, "RequestLimitExceeded") {
		return true
	}

	// Service unavailable
	if contains(errStr, "ServiceUnavailable") || contains(errStr, "InternalError") {
		return true
	}

	// Network errors
	if contains(errStr, "connection") || contains(errStr, "timeout") {
		return true
	}

	return false
}

// parseMonthFromDate extracts YYYY-MM from a date string
func parseMonthFromDate(dateStr string) (string, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01"), nil
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
