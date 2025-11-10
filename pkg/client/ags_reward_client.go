package client

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/platform-sdk/pkg/platformclient/entitlement"
	"github.com/AccelByte/accelbyte-go-sdk/platform-sdk/pkg/platformclient/wallet"
	"github.com/AccelByte/accelbyte-go-sdk/platform-sdk/pkg/platformclientmodels"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/platform"
	"github.com/sirupsen/logrus"

	commonClient "github.com/AccelByte/extend-challenge-common/pkg/client"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// AGSRewardClient implements RewardClient interface using AccelByte Gaming Services (AGS) Platform SDK.
// It provides retry logic for reliability and proper error handling for AGS-specific errors.
type AGSRewardClient struct {
	entitlementService *platform.EntitlementService
	walletService      *platform.WalletService
	logger             *logrus.Logger
}

// NewAGSRewardClient creates a new AGSRewardClient with AGS Platform SDK services.
//
// Parameters:
//   - entitlementService: AGS Platform EntitlementService for granting items
//   - walletService: AGS Platform WalletService for crediting virtual currency
//   - logger: Logger for structured logging
func NewAGSRewardClient(
	entitlementService *platform.EntitlementService,
	walletService *platform.WalletService,
	logger *logrus.Logger,
) commonClient.RewardClient {
	return &AGSRewardClient{
		entitlementService: entitlementService,
		walletService:      walletService,
		logger:             logger,
	}
}

// GrantItemReward grants an item entitlement to a user using AGS Platform Service.
//
// This method:
//   - Creates an EntitlementGrant request with itemID, namespace, and quantity
//   - Calls GrantUserEntitlementShort SDK function
//   - Retries on transient failures (502/503, timeouts) with exponential backoff
//   - Fails immediately on non-retryable errors (400, 404, 403)
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - namespace: AGS namespace for the deployment
//   - userID: User's unique identifier
//   - itemID: Item code from AGS inventory catalog
//   - quantity: Number of items to grant
//
// Returns error if grant fails after retries or on non-retryable errors.
func (c *AGSRewardClient) GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error {
	// Validate quantity is within int32 range to prevent overflow
	if quantity < 0 || quantity > 2147483647 {
		return &commonClient.BadRequestError{
			Message: fmt.Sprintf("quantity %d out of range for int32", quantity),
		}
	}

	return c.withRetry(ctx, "grant_item", func() error {
		// Create entitlement grant request
		// NOTE: ItemNamespace must equal deployment namespace (no cross-namespace grants)
		//nolint:gosec // G115: Safe conversion after range validation above
		quantity32 := int32(quantity)
		grant := &platformclientmodels.EntitlementGrant{
			ItemID:        &itemID,
			ItemNamespace: &namespace,
			Quantity:      &quantity32,
		}

		params := &entitlement.GrantUserEntitlementParams{
			Namespace: namespace,
			UserID:    userID,
			Body:      []*platformclientmodels.EntitlementGrant{grant}, // NOTE: Body is array, not single grant
		}

		// Call AGS Platform SDK
		response, err := c.entitlementService.GrantUserEntitlementShort(params)
		if err != nil {
			return c.wrapSDKError(err, "failed to grant item reward")
		}

		// Log response for audit (don't validate, just log)
		c.logger.WithFields(logrus.Fields{
			"namespace": namespace,
			"userID":    userID,
			"itemID":    itemID,
			"quantity":  quantity,
			"response":  response,
		}).Info("Item reward granted successfully")

		return nil
	})
}

// GrantWalletReward credits a user's wallet with virtual currency using AGS Platform Service.
//
// This method:
//   - Creates a CreditRequest with amount and currencyCode
//   - Calls CreditUserWalletShort SDK function (creates wallet if not exists)
//   - Retries on transient failures (502/503, timeouts) with exponential backoff
//   - Fails immediately on non-retryable errors (400, 404, 403)
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - namespace: AGS namespace for the deployment
//   - userID: User's unique identifier
//   - currencyCode: Currency code from AGS wallet (e.g., "GOLD", "GEMS")
//   - amount: Amount of currency to credit
//
// Returns error if grant fails after retries or on non-retryable errors.
func (c *AGSRewardClient) GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error {
	// Validate amount is non-negative (int64 range is much larger than int, so only check negative)
	if amount < 0 {
		return &commonClient.BadRequestError{
			Message: fmt.Sprintf("amount %d cannot be negative", amount),
		}
	}

	return c.withRetry(ctx, "grant_wallet", func() error {
		// Create credit request
		// NOTE: Amount is int64, not int
		amount64 := int64(amount) // Safe conversion from int to int64
		creditReq := &platformclientmodels.CreditRequest{
			Amount: &amount64,
			// Optional fields can be added here: Reason, Source, Origin, Metadata
		}

		params := &wallet.CreditUserWalletParams{
			Namespace:    namespace,
			UserID:       userID,
			CurrencyCode: currencyCode,
			Body:         creditReq,
		}

		// Call AGS Platform SDK
		response, err := c.walletService.CreditUserWalletShort(params)
		if err != nil {
			return c.wrapSDKError(err, "failed to credit wallet")
		}

		// Log response for audit (don't validate, just log)
		c.logger.WithFields(logrus.Fields{
			"namespace":    namespace,
			"userID":       userID,
			"currencyCode": currencyCode,
			"amount":       amount,
			"response":     response,
		}).Info("Wallet credited successfully")

		return nil
	})
}

// GrantReward is a convenience method that dispatches to the appropriate grant method
// based on the reward type (ITEM or WALLET).
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - namespace: AGS namespace for the deployment
//   - userID: User's unique identifier
//   - reward: Reward configuration from goal
//
// Returns error if reward type is unsupported or grant fails after retries.
func (c *AGSRewardClient) GrantReward(ctx context.Context, namespace, userID string, reward commonDomain.Reward) error {
	switch reward.Type {
	case "ITEM":
		return c.GrantItemReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	case "WALLET":
		return c.GrantWalletReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	default:
		c.logger.WithFields(logrus.Fields{
			"namespace":  namespace,
			"userID":     userID,
			"rewardType": reward.Type,
		}).Warn("Unknown reward type")
		return fmt.Errorf("unsupported reward type: %s", reward.Type)
	}
}

// withRetry executes the given function with retry logic for transient failures.
//
// Retry strategy:
//   - Maximum retries: 3 (total 4 attempts)
//   - Base delay: 500ms
//   - Exponential backoff: 500ms, 1s, 2s
//   - Total timeout: 10 seconds (prevents transaction timeout)
//   - Context cancellation check before each retry
//
// The function will:
//   - Retry on transient failures: 502/503, timeouts, network errors
//   - Fail immediately on non-retryable errors: 400, 404, 403
//   - Respect context cancellation during retry delays
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - operation: Operation name for logging (e.g., "grant_item", "grant_wallet")
//   - fn: Function to execute with retry logic
//
// Returns error if all retries are exhausted or on non-retryable errors.
func (c *AGSRewardClient) withRetry(ctx context.Context, operation string, fn func() error) error {
	const maxRetries = 3
	const baseDelay = 500 * time.Millisecond
	const totalTimeout = 10 * time.Second // NQ8: 10s total timeout to prevent transaction timeout

	// Create context with total timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		// Check context cancellation before retry (NQ4: always check ctx.Err())
		if err := timeoutCtx.Err(); err != nil {
			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
				"error":     err,
			}).Warn("Context cancelled or timeout exceeded, stopping retries")
			return fmt.Errorf("context cancelled: %w", err)
		}

		// Execute operation with timeout context
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 {
				c.logger.WithFields(logrus.Fields{
					"operation": operation,
					"attempt":   attempt,
				}).Info("Reward grant succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable (uses commonClient.IsRetryableError)
		if !commonClient.IsRetryableError(err) {
			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
				"error":     err,
			}).Error("Non-retryable error, failing immediately")
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't sleep after last attempt
		if attempt <= maxRetries {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // Exponential backoff: 500ms, 1s, 2s
			c.logger.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt,
				"nextDelay": delay,
				"error":     err,
			}).Warn("Reward grant failed, will retry")

			// Use time.After with select to respect context cancellation during sleep
			select {
			case <-time.After(delay):
				// Continue to next retry
			case <-timeoutCtx.Done():
				c.logger.WithFields(logrus.Fields{
					"operation": operation,
					"attempt":   attempt,
				}).Warn("Timeout during backoff delay")
				return fmt.Errorf("timeout during retry backoff: %w", timeoutCtx.Err())
			}
		}
	}

	// All retries exhausted
	c.logger.WithFields(logrus.Fields{
		"operation": operation,
		"attempts":  maxRetries + 1,
		"error":     lastErr,
	}).Error("Reward grant failed after all retries")
	return fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// wrapSDKError wraps an AGS SDK error with a custom error type that includes HTTP status code.
//
// This function attempts to extract the HTTP status code from the SDK error using type assertion.
// If successful, it creates an appropriate error type (BadRequestError, NotFoundError, etc.).
// If status code extraction fails, it wraps the error with a generic message.
//
// Parameters:
//   - err: Original error from AGS SDK
//   - message: Prefix message for the error
//
// Returns a wrapped error with HTTP status code (if available).
func (c *AGSRewardClient) wrapSDKError(err error, message string) error {
	if err == nil {
		return nil
	}

	// Try to extract status code from SDK error (NQ1: type assertion pattern)
	statusCode, ok := c.extractStatusCode(err)
	if !ok {
		// Could not extract status code, wrap with generic message
		return fmt.Errorf("%s: %w", message, err)
	}

	// Map status code to appropriate error type
	switch statusCode {
	case 400:
		return &commonClient.BadRequestError{
			Message: err.Error(),
		}
	case 401:
		return &commonClient.AuthenticationError{
			Message: err.Error(),
		}
	case 403:
		return &commonClient.ForbiddenError{
			Message: err.Error(),
		}
	case 404:
		return &commonClient.NotFoundError{
			Resource: err.Error(),
		}
	default:
		return &commonClient.AGSError{
			StatusCode: statusCode,
			Message:    err.Error(),
		}
	}
}

// extractStatusCode attempts to extract HTTP status code from SDK error.
//
// This function uses type assertion to identify specific SDK error types and extract their status codes.
// It handles known error types from entitlement and wallet operations, with a regex fallback for unknown types.
//
// Known SDK error types (Phase 7.6):
//   - entitlement.GrantUserEntitlementNotFound (404)
//   - entitlement.GrantUserEntitlementUnprocessableEntity (422)
//   - wallet.CreditUserWalletBadRequest (400)
//   - wallet.CreditUserWalletUnprocessableEntity (422)
//
// Fallback: Parses generic SDK error message format "[METHOD /path][CODE] errorName {...}"
//
// Parameters:
//   - err: Error from AGS SDK
//
// Returns (statusCode, true) if extraction successful, (0, false) otherwise.
func (c *AGSRewardClient) extractStatusCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}

	// Type assertion for known SDK error types (Phase 7.6: Option B)
	switch err.(type) {
	case *entitlement.GrantUserEntitlementNotFound:
		return 404, true
	case *entitlement.GrantUserEntitlementUnprocessableEntity:
		return 422, true
	case *wallet.CreditUserWalletBadRequest:
		return 400, true
	case *wallet.CreditUserWalletUnprocessableEntity:
		return 422, true
	}

	// Fallback: Parse generic SDK error message format
	// Example: "[POST /platform/...][404] grantUserEntitlementNotFound {...}"
	errMsg := err.Error()
	if strings.Contains(errMsg, "[") && strings.Contains(errMsg, "]") {
		re := regexp.MustCompile(`\[(\d{3})\]`)
		matches := re.FindStringSubmatch(errMsg)
		if len(matches) > 1 {
			if code, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
				return code, true
			}
		}
	}

	// Could not extract status code from error
	c.logger.WithFields(logrus.Fields{
		"errorType": fmt.Sprintf("%T", err),
		"error":     err.Error(),
	}).Debug("Could not extract status code from SDK error")

	return 0, false
}
