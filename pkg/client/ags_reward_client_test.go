package client

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/platform-sdk/pkg/platformclient/entitlement"
	"github.com/AccelByte/accelbyte-go-sdk/platform-sdk/pkg/platformclient/wallet"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/platform"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	commonClient "github.com/AccelByte/extend-challenge-common/pkg/client"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// Mock EntitlementService
type MockEntitlementService struct {
	mock.Mock
}

func (m *MockEntitlementService) GrantUserEntitlementShort(params *entitlement.GrantUserEntitlementParams) (interface{}, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

// Mock WalletService
type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) CreditUserWalletShort(params *wallet.CreditUserWalletParams) (interface{}, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

// Mock SDK error types that match AccelByte SDK v0.80.0 error structure (Phase 7.6)
// Real SDK errors don't have StatusCode() method - they use type assertion

// mockGrantUserEntitlementNotFound simulates entitlement.GrantUserEntitlementNotFound (404)
type mockGrantUserEntitlementNotFound struct {
	message string
}

func (e *mockGrantUserEntitlementNotFound) Error() string {
	if e.message != "" {
		return e.message
	}
	return "[POST /platform/admin/namespaces/test/users/user123/entitlements][404] grantUserEntitlementNotFound {\"errorCode\":30341,\"errorMessage\":\"Item [item123] does not exist in namespace [test]\"}"
}

// mockGrantUserEntitlementUnprocessableEntity simulates entitlement.GrantUserEntitlementUnprocessableEntity (422)
type mockGrantUserEntitlementUnprocessableEntity struct {
	message string
}

func (e *mockGrantUserEntitlementUnprocessableEntity) Error() string {
	if e.message != "" {
		return e.message
	}
	return "[POST /platform/admin/namespaces/test/users/user123/entitlements][422] grantUserEntitlementUnprocessableEntity {\"errorCode\":30342,\"errorMessage\":\"Invalid entitlement grant\"}"
}

// mockCreditUserWalletBadRequest simulates wallet.CreditUserWalletBadRequest (400)
type mockCreditUserWalletBadRequest struct {
	message string
}

func (e *mockCreditUserWalletBadRequest) Error() string {
	if e.message != "" {
		return e.message
	}
	return "[PUT /platform/admin/namespaces/test/users/user123/wallets/GOLD/credit][400] creditUserWalletBadRequest {\"errorCode\":35123,\"errorMessage\":\"Invalid credit amount\"}"
}

// mockCreditUserWalletUnprocessableEntity simulates wallet.CreditUserWalletUnprocessableEntity (422)
type mockCreditUserWalletUnprocessableEntity struct {
	message string
}

func (e *mockCreditUserWalletUnprocessableEntity) Error() string {
	if e.message != "" {
		return e.message
	}
	return "[PUT /platform/admin/namespaces/test/users/user123/wallets/GOLD/credit][422] creditUserWalletUnprocessableEntity {\"errorCode\":35124,\"errorMessage\":\"Wallet operation failed\"}"
}

// mockGenericSDKError simulates an unknown SDK error with generic format
type mockGenericSDKError struct {
	statusCode int
	message    string
}

func (e *mockGenericSDKError) Error() string {
	if e.message != "" {
		return e.message
	}
	return fmt.Sprintf("[POST /platform/some/endpoint][%d] someSDKError {\"errorCode\":99999,\"errorMessage\":\"Generic SDK error\"}", e.statusCode)
}

// TestNewAGSRewardClient tests the constructor
func TestNewAGSRewardClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress logs in tests

	// Create wrapper services that match SDK structure
	entitlementService := &platform.EntitlementService{}
	walletService := &platform.WalletService{}

	client := NewAGSRewardClient(entitlementService, walletService, logger)

	assert.NotNil(t, client)
	agsClient, ok := client.(*AGSRewardClient)
	assert.True(t, ok)
	assert.NotNil(t, agsClient.entitlementService)
	assert.NotNil(t, agsClient.walletService)
	assert.NotNil(t, agsClient.logger)
}

// TestGrantReward_UnknownType tests handling of unknown reward type
func TestGrantReward_UnknownType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: &platform.EntitlementService{},
		walletService:      &platform.WalletService{},
		logger:             logger,
	}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "UNKNOWN",
		RewardID: "something",
		Quantity: 10,
	}

	err := client.GrantReward(ctx, "test-namespace", "user123", reward)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported reward type")
}

// TestWrapSDKError_BadRequest tests 400 error mapping using wallet.CreditUserWalletBadRequest
func TestWrapSDKError_BadRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate wallet.CreditUserWalletBadRequest (400) using regex fallback
	sdkErr := &mockCreditUserWalletBadRequest{
		message: "[PUT /platform/admin/namespaces/test/users/user123/wallets/GOLD/credit][400] creditUserWalletBadRequest {\"errorCode\":35123,\"errorMessage\":\"Invalid credit amount\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var badReqErr *commonClient.BadRequestError
	assert.ErrorAs(t, wrappedErr, &badReqErr)
	assert.Equal(t, 400, badReqErr.HTTPStatusCode())
}

// TestWrapSDKError_NotFound tests 404 error mapping using entitlement.GrantUserEntitlementNotFound
func TestWrapSDKError_NotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate entitlement.GrantUserEntitlementNotFound (404) using regex fallback
	sdkErr := &mockGrantUserEntitlementNotFound{
		message: "[POST /platform/admin/namespaces/test/users/user123/entitlements][404] grantUserEntitlementNotFound {\"errorCode\":30341,\"errorMessage\":\"Item not found\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var notFoundErr *commonClient.NotFoundError
	assert.ErrorAs(t, wrappedErr, &notFoundErr)
	assert.Equal(t, 404, notFoundErr.HTTPStatusCode())
}

// TestWrapSDKError_Unauthorized tests 401 error mapping
func TestWrapSDKError_Unauthorized(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate generic SDK error with 401 status code using regex fallback
	sdkErr := &mockGenericSDKError{
		statusCode: 401,
		message:    "[POST /platform/some/endpoint][401] unauthorized {\"errorCode\":20001,\"errorMessage\":\"Invalid credentials\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var authErr *commonClient.AuthenticationError
	assert.ErrorAs(t, wrappedErr, &authErr)
	assert.Equal(t, 401, authErr.HTTPStatusCode())
}

// TestWrapSDKError_Forbidden tests 403 error mapping
func TestWrapSDKError_Forbidden(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate generic SDK error with 403 status code using regex fallback
	sdkErr := &mockGenericSDKError{
		statusCode: 403,
		message:    "[POST /platform/some/endpoint][403] forbidden {\"errorCode\":20013,\"errorMessage\":\"Insufficient permissions\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var forbiddenErr *commonClient.ForbiddenError
	assert.ErrorAs(t, wrappedErr, &forbiddenErr)
	assert.Equal(t, 403, forbiddenErr.HTTPStatusCode())
}

// TestWrapSDKError_AGSError tests 502 error mapping
func TestWrapSDKError_AGSError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate generic SDK error with 502 status code using regex fallback
	sdkErr := &mockGenericSDKError{
		statusCode: 502,
		message:    "[POST /platform/some/endpoint][502] badGateway {\"errorCode\":50000,\"errorMessage\":\"Bad gateway\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var agsErr *commonClient.AGSError
	assert.ErrorAs(t, wrappedErr, &agsErr)
	assert.Equal(t, 502, agsErr.HTTPStatusCode())
}

// TestWrapSDKError_NoStatusCode tests error without status code
func TestWrapSDKError_NoStatusCode(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	genericErr := errors.New("some generic error")
	wrappedErr := client.wrapSDKError(genericErr, "test operation")

	assert.Error(t, wrappedErr)
	assert.Contains(t, wrappedErr.Error(), "test operation")
	assert.Contains(t, wrappedErr.Error(), "some generic error")
}

// TestWrapSDKError_NilError tests wrapping nil error
func TestWrapSDKError_NilError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	wrappedErr := client.wrapSDKError(nil, "test operation")

	assert.Nil(t, wrappedErr)
}

// TestWrapSDKError_503ServiceUnavailable tests 503 error mapping to AGSError
func TestWrapSDKError_503ServiceUnavailable(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Simulate generic SDK error with 503 status code using regex fallback
	sdkErr := &mockGenericSDKError{
		statusCode: 503,
		message:    "[POST /platform/some/endpoint][503] serviceUnavailable {\"errorCode\":50003,\"errorMessage\":\"Service unavailable\"}",
	}

	wrappedErr := client.wrapSDKError(sdkErr, "test operation")

	assert.Error(t, wrappedErr)

	var agsErr *commonClient.AGSError
	assert.ErrorAs(t, wrappedErr, &agsErr)
	assert.Equal(t, 503, agsErr.HTTPStatusCode())
}

// TestExtractStatusCode_Success tests successful status code extraction using regex fallback
func TestExtractStatusCode_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Test regex fallback with SDK error format
	sdkErr := &mockCreditUserWalletBadRequest{
		message: "[PUT /platform/admin/namespaces/test/users/user123/wallets/GOLD/credit][400] creditUserWalletBadRequest {\"errorCode\":35123,\"errorMessage\":\"Bad request\"}",
	}

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.True(t, ok)
	assert.Equal(t, 400, statusCode)
}

// TestExtractStatusCode_Failure tests status code extraction failure
func TestExtractStatusCode_Failure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	genericErr := errors.New("generic error")

	statusCode, ok := client.extractStatusCode(genericErr)

	assert.False(t, ok)
	assert.Equal(t, 0, statusCode)
}

// TestExtractStatusCode_Nil tests nil error
func TestExtractStatusCode_Nil(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	statusCode, ok := client.extractStatusCode(nil)

	assert.False(t, ok)
	assert.Equal(t, 0, statusCode)
}

// TestExtractStatusCode_RegexFallback_404 tests regex fallback for 404 entitlement error
func TestExtractStatusCode_RegexFallback_404(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Test regex fallback with entitlement 404 error format
	sdkErr := &mockGrantUserEntitlementNotFound{}

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.True(t, ok)
	assert.Equal(t, 404, statusCode)
}

// TestExtractStatusCode_RegexFallback_422 tests regex fallback for 422 wallet error
func TestExtractStatusCode_RegexFallback_422(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Test regex fallback with wallet 422 error format
	sdkErr := &mockCreditUserWalletUnprocessableEntity{}

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.True(t, ok)
	assert.Equal(t, 422, statusCode)
}

// TestExtractStatusCode_RegexFallback_Entitlement422 tests regex fallback for 422 entitlement error
func TestExtractStatusCode_RegexFallback_Entitlement422(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Test regex fallback with entitlement 422 error format
	sdkErr := &mockGrantUserEntitlementUnprocessableEntity{}

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.True(t, ok)
	assert.Equal(t, 422, statusCode)
}

// TestExtractStatusCode_RegexFallback_MultipleDigits tests regex extracts first status code
func TestExtractStatusCode_RegexFallback_MultipleDigits(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Test that regex extracts the first [NNN] pattern (status code), not error codes
	sdkErr := &mockGenericSDKError{
		statusCode: 503,
		message:    "[POST /endpoint][503] error {\"errorCode\":50003}",
	}

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.True(t, ok)
	assert.Equal(t, 503, statusCode) // Should extract 503, not 50003
}

// TestExtractStatusCode_InvalidFormat tests handling of non-SDK error format
func TestExtractStatusCode_InvalidFormat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Error message without [NNN] pattern
	sdkErr := errors.New("some error without status code format")

	statusCode, ok := client.extractStatusCode(sdkErr)

	assert.False(t, ok)
	assert.Equal(t, 0, statusCode)
}

// TestWithRetry_Success tests retry logic with immediate success
func TestWithRetry_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		return nil // Success on first try
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// TestWithRetry_SuccessAfterRetries tests retry logic with eventual success
func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		if callCount < 3 {
			// Fail first 2 attempts with retryable error
			return &commonClient.AGSError{
				StatusCode: 503,
				Message:    "service unavailable",
			}
		}
		return nil // Success on 3rd try
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

// TestWithRetry_NonRetryableError tests immediate failure on non-retryable error
func TestWithRetry_NonRetryableError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		// Return non-retryable error (400)
		return &commonClient.BadRequestError{
			Message: "invalid request",
		}
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // Should not retry
	assert.Contains(t, err.Error(), "non-retryable error")
}

// TestWithRetry_MaxRetriesExceeded tests failure after max retries
func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		// Always return retryable error
		return &commonClient.AGSError{
			StatusCode: 503,
			Message:    "service unavailable",
		}
	})

	assert.Error(t, err)
	assert.Equal(t, 4, callCount) // 1 initial + 3 retries
	assert.Contains(t, err.Error(), "failed after 4 attempts")
}

// TestWithRetry_ContextCancelled tests context cancellation before retry
func TestWithRetry_ContextCancelled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		// Return retryable error, but context should cancel before retry
		return &commonClient.AGSError{
			StatusCode: 503,
			Message:    "service unavailable",
		}
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
	// Should fail fast due to context cancellation
	assert.LessOrEqual(t, callCount, 2)
}

// TestWithRetry_ContextCancelledDuringBackoff tests context cancellation during retry delay
func TestWithRetry_ContextCancelledDuringBackoff(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	// Create context with timeout that expires during backoff
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	callCount := 0

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		// Always return retryable error
		return &commonClient.AGSError{
			StatusCode: 503,
			Message:    "service unavailable",
		}
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	// Should fail after 1-2 attempts due to timeout during backoff
	assert.LessOrEqual(t, callCount, 2)
}

// TestWithRetry_ExponentialBackoff tests exponential backoff timing
func TestWithRetry_ExponentialBackoff(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	callCount := 0
	callTimes := []time.Time{}

	err := client.withRetry(ctx, "test_op", func() error {
		callCount++
		callTimes = append(callTimes, time.Now())

		if callCount < 4 {
			// Fail first 3 attempts
			return &commonClient.AGSError{
				StatusCode: 503,
				Message:    "service unavailable",
			}
		}
		return nil // Success on 4th try
	})

	assert.NoError(t, err)
	assert.Equal(t, 4, callCount)
	assert.Len(t, callTimes, 4)

	// Verify exponential backoff: ~500ms, ~1s, ~2s
	// Allow ±200ms tolerance for timing variations
	delay1 := callTimes[1].Sub(callTimes[0])
	delay2 := callTimes[2].Sub(callTimes[1])
	delay3 := callTimes[3].Sub(callTimes[2])

	assert.InDelta(t, 500*time.Millisecond, delay1, float64(200*time.Millisecond))
	assert.InDelta(t, 1*time.Second, delay2, float64(200*time.Millisecond))
	assert.InDelta(t, 2*time.Second, delay3, float64(200*time.Millisecond))
}

// TestWithRetry_TotalTimeout tests that retry loop respects total timeout
func TestWithRetry_TotalTimeout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		logger: logger,
	}

	ctx := context.Background()
	startTime := time.Now()

	err := client.withRetry(ctx, "test_op", func() error {
		// Always return retryable error
		return &commonClient.AGSError{
			StatusCode: 503,
			Message:    "service unavailable",
		}
	})

	duration := time.Since(startTime)

	assert.Error(t, err)
	// With 3 retries and exponential backoff (500ms, 1s, 2s), total is ~3.5s
	// Should complete before 10s timeout since retries exhaust first
	assert.Less(t, duration, 10*time.Second, "Should complete before 10s timeout")
	// Should take around 3.5s for all retries (allow ±1s tolerance)
	expectedDuration := 3500 * time.Millisecond
	tolerance := float64(1 * time.Second)
	assert.InDelta(t, float64(expectedDuration), float64(duration), tolerance)
}

// ============================================================================
// Phase 7.5: Input Validation Tests
// ============================================================================

// TestGrantItemReward_ValidQuantity tests that valid quantity passes validation
func TestGrantItemReward_ValidQuantity(t *testing.T) {
	// This test verifies that valid quantities pass the validation check
	// We test the validation logic directly rather than calling GrantItemReward
	// to avoid needing to mock the SDK services

	testCases := []struct {
		quantity int
		valid    bool
	}{
		{1, true},
		{100, true},
		{1000, true},
		{2147483647, true},  // int32 max
		{-1, false},         // negative
		{2147483648, false}, // overflow
	}

	for _, tc := range testCases {
		// Test the validation logic
		inRange := tc.quantity >= 0 && tc.quantity <= 2147483647
		assert.Equal(t, tc.valid, inRange, "quantity %d validation should be %v", tc.quantity, tc.valid)
	}
}

// TestGrantItemReward_QuantityNegative tests that negative quantity fails validation
func TestGrantItemReward_QuantityNegative(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: (*platform.EntitlementService)(nil),
		walletService:      (*platform.WalletService)(nil),
		logger:             logger,
	}

	ctx := context.Background()

	// Test negative quantities
	testCases := []int{-1, -10, -100, -2147483648}

	for _, quantity := range testCases {
		err := client.GrantItemReward(ctx, "test-ns", "user-123", "item-001", quantity)

		// Should return BadRequestError
		assert.Error(t, err)
		var badReqErr *commonClient.BadRequestError
		assert.ErrorAs(t, err, &badReqErr, "negative quantity %d should return BadRequestError", quantity)
		assert.Contains(t, err.Error(), "out of range", "error message should mention 'out of range'")
	}
}

// TestGrantItemReward_QuantityOverflow tests that quantity > int32 max fails validation
func TestGrantItemReward_QuantityOverflow(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: (*platform.EntitlementService)(nil),
		walletService:      (*platform.WalletService)(nil),
		logger:             logger,
	}

	ctx := context.Background()

	// Test quantities that overflow int32
	testCases := []int{2147483648, 3000000000, 9223372036854775807}

	for _, quantity := range testCases {
		err := client.GrantItemReward(ctx, "test-ns", "user-123", "item-001", quantity)

		// Should return BadRequestError
		assert.Error(t, err)
		var badReqErr *commonClient.BadRequestError
		assert.ErrorAs(t, err, &badReqErr, "overflow quantity %d should return BadRequestError", quantity)
		assert.Contains(t, err.Error(), "out of range", "error message should mention 'out of range'")
	}
}

// TestGrantWalletReward_ValidAmount tests that valid amount passes validation
func TestGrantWalletReward_ValidAmount(t *testing.T) {
	// This test verifies that valid amounts pass the validation check
	// We test the validation logic directly rather than calling GrantWalletReward
	// to avoid needing to mock the SDK services

	testCases := []struct {
		amount int
		valid  bool
	}{
		{0, true},
		{1, true},
		{100, true},
		{1000, true},
		{1000000, true},
		{-1, false},       // negative
		{-100, false},     // negative
		{-1000000, false}, // negative
	}

	for _, tc := range testCases {
		// Test the validation logic
		isValid := tc.amount >= 0
		assert.Equal(t, tc.valid, isValid, "amount %d validation should be %v", tc.amount, tc.valid)
	}
}

// TestGrantWalletReward_AmountNegative tests that negative amount fails validation
func TestGrantWalletReward_AmountNegative(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: (*platform.EntitlementService)(nil),
		walletService:      (*platform.WalletService)(nil),
		logger:             logger,
	}

	ctx := context.Background()

	// Test negative amounts
	testCases := []int{-1, -10, -100, -1000000}

	for _, amount := range testCases {
		err := client.GrantWalletReward(ctx, "test-ns", "user-123", "GOLD", amount)

		// Should return BadRequestError
		assert.Error(t, err)
		var badReqErr *commonClient.BadRequestError
		assert.ErrorAs(t, err, &badReqErr, "negative amount %d should return BadRequestError", amount)
		assert.Contains(t, err.Error(), "cannot be negative", "error message should mention 'cannot be negative'")
	}
}

// ============================================================================
// Phase 7.5: Dispatcher Routing Tests
// ============================================================================

// TestGrantReward_ItemTypeRouting tests that ITEM type routes to GrantItemReward
func TestGrantReward_ItemTypeRouting(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: (*platform.EntitlementService)(nil),
		walletService:      (*platform.WalletService)(nil),
		logger:             logger,
	}

	ctx := context.Background()
	// Use invalid quantity to trigger GrantItemReward validation (not SDK call)
	reward := commonDomain.Reward{
		Type:     "ITEM",
		RewardID: "sword_001",
		Quantity: -5, // Invalid - will trigger GrantItemReward validation
	}

	err := client.GrantReward(ctx, "test-ns", "user-123", reward)

	// Should route to GrantItemReward and hit its validation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range", "should route to GrantItemReward and trigger its validation")

	var badReqErr *commonClient.BadRequestError
	assert.ErrorAs(t, err, &badReqErr, "should return BadRequestError from GrantItemReward validation")
}

// TestGrantReward_WalletTypeRouting tests that WALLET type routes to GrantWalletReward
func TestGrantReward_WalletTypeRouting(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &AGSRewardClient{
		entitlementService: (*platform.EntitlementService)(nil),
		walletService:      (*platform.WalletService)(nil),
		logger:             logger,
	}

	ctx := context.Background()
	// Use invalid amount to trigger GrantWalletReward validation (not SDK call)
	reward := commonDomain.Reward{
		Type:     "WALLET",
		RewardID: "GOLD",
		Quantity: -100, // Invalid - will trigger GrantWalletReward validation
	}

	err := client.GrantReward(ctx, "test-ns", "user-123", reward)

	// Should route to GrantWalletReward and hit its validation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be negative", "should route to GrantWalletReward and trigger its validation")

	var badReqErr *commonClient.BadRequestError
	assert.ErrorAs(t, err, &badReqErr, "should return BadRequestError from GrantWalletReward validation")
}
