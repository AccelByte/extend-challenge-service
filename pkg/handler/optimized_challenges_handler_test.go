// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"extend-challenge-service/pkg/cache"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// MockGoalCache is a mock implementation of commonCache.GoalCache
type MockGoalCache struct {
	mock.Mock
}

func (m *MockGoalCache) GetGoalByID(goalID string) *commonDomain.Goal {
	args := m.Called(goalID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*commonDomain.Goal)
}

func (m *MockGoalCache) GetGoalsByStatCode(statCode string) []*commonDomain.Goal {
	args := m.Called(statCode)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*commonDomain.Goal)
}

func (m *MockGoalCache) GetChallengeByChallengeID(challengeID string) *commonDomain.Challenge {
	args := m.Called(challengeID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*commonDomain.Challenge)
}

func (m *MockGoalCache) GetAllChallenges() []*commonDomain.Challenge {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*commonDomain.Challenge)
}

func (m *MockGoalCache) GetAllGoals() []*commonDomain.Goal {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*commonDomain.Goal)
}

func (m *MockGoalCache) GetGoalsWithDefaultAssigned() []*commonDomain.Goal {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*commonDomain.Goal)
}

func (m *MockGoalCache) Reload() error {
	args := m.Called()
	return args.Error(0)
}

// MockGoalRepository is a mock implementation of commonRepo.GoalRepository
type MockGoalRepository struct {
	mock.Mock
}

func (m *MockGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) UpsertProgress(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgress(ctx context.Context, updates []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGoalRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int, isDailyIncrement bool) error {
	args := m.Called(ctx, userID, goalID, challengeID, namespace, delta, targetValue, isDailyIncrement)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchIncrementProgress(ctx context.Context, increments []commonRepo.ProgressIncrement) error {
	args := m.Called(ctx, increments)
	return args.Error(0)
}

func (m *MockGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockGoalRepository) BeginTx(ctx context.Context) (commonRepo.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(commonRepo.TxRepository), args.Error(1)
}

func (m *MockGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) BulkInsert(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGoalRepository) BulkInsertWithCOPY(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGoalRepository) UpsertGoalActive(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// M3 Phase 9: Fast path optimization methods
func (m *MockGoalRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockGoalRepository) GetActiveGoals(ctx context.Context, userID string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

// Helper function to create test challenges
func createTestChallenges() []*commonDomain.Challenge {
	return []*commonDomain.Challenge{
		{
			ID:          "daily-challenge",
			Name:        "Daily Challenge",
			Description: "Complete daily tasks",
			Goals: []*commonDomain.Goal{
				{
					ID:          "daily-login",
					ChallengeID: "daily-challenge",
					Name:        "Login Daily",
					Description: "Login to the game",
					Type:        commonDomain.GoalTypeAbsolute,
					EventSource: commonDomain.EventSourceLogin,
					Requirement: commonDomain.Requirement{
						StatCode:    "login_count",
						Operator:    ">=",
						TargetValue: 1,
					},
					Reward: commonDomain.Reward{
						Type:     "WALLET",
						RewardID: "gold",
						Quantity: 100,
					},
				},
			},
		},
	}
}

// Helper function to create test progress
func createTestProgress(isActive bool) []*commonDomain.UserGoalProgress {
	return []*commonDomain.UserGoalProgress{
		{
			UserID:      "test-user",
			GoalID:      "daily-login",
			ChallengeID: "daily-challenge",
			Namespace:   "test-namespace",
			Progress:    1,
			Status:      commonDomain.GoalStatusCompleted,
			IsActive:    isActive,
		},
	}
}

func TestOptimizedChallengesHandler_ServeHTTP_ActiveOnlyFalse(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create serialization cache
	serCache := cache.NewSerializedChallengeCache()
	// Note: We don't warm up the cache in tests - the handler will use the response builder
	// which can handle empty cache gracefully

	// Create handler with auth disabled for simplicity
	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Setup expectations
	challenges := createTestChallenges()
	mockCache.On("GetAllChallenges").Return(challenges)
	mockRepo.On("GetUserProgress", mock.Anything, "test-user", false).Return(createTestProgress(false), nil)

	// Create request without active_only parameter (default: false)
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "test-user") // Test header for auth-disabled mode

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// The main test objective is to verify that activeOnly=false is passed to repository
	// Response building may fail without warming up the serialization cache, but that's OK
	// The key behavior (query parameter extraction and repository call) is verified by mock expectations

	// Verify mock expectations - this confirms activeOnly was extracted and passed correctly
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedChallengesHandler_ServeHTTP_ActiveOnlyTrue(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create serialization cache
	serCache := cache.NewSerializedChallengeCache()

	// Create handler with auth disabled for simplicity
	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Setup expectations - only active goals
	challenges := createTestChallenges()
	mockCache.On("GetAllChallenges").Return(challenges)
	mockRepo.On("GetUserProgress", mock.Anything, "test-user", true).Return(createTestProgress(true), nil)

	// Create request with active_only=true parameter
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=true", nil)
	req.Header.Set("x-mock-user-id", "test-user") // Test header for auth-disabled mode

	// Create response recorder
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// The main test objective is to verify that activeOnly=true is passed to repository
	// Response building may fail without warming up the serialization cache, but that's OK
	// The key behavior (query parameter extraction and repository call) is verified by mock expectations

	// Verify mock expectations - this confirms activeOnly was extracted and passed correctly
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedChallengesHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	// Setup minimal mocks (won't be called)
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false,
		nil,
	)

	// Create POST request (not allowed)
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges", nil)
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestOptimizedChallengesHandler_ServeHTTP_NoChallenges(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false,
		nil,
	)

	// Setup expectations - no challenges
	mockCache.On("GetAllChallenges").Return([]*commonDomain.Challenge{})

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"challenges":[]}`, w.Body.String())

	// Verify mock expectations
	mockCache.AssertExpectations(t)
}

func TestOptimizedChallengesHandler_ServeHTTP_DatabaseError(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false,
		nil,
	)

	// Setup expectations - database error
	challenges := createTestChallenges()
	mockCache.On("GetAllChallenges").Return(challenges)
	mockRepo.On("GetUserProgress", mock.Anything, "test-user", false).Return(nil, assert.AnError)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// TestOptimizedChallengesHandler_ExtractUserID_NoAuthNoHeader tests missing user ID header
func TestOptimizedChallengesHandler_ExtractUserID_NoAuthNoHeader(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Create request without x-mock-user-id header (should use default "test-user-id")
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	w := httptest.NewRecorder()

	// Setup mock - should be called with default user ID
	mockCache.On("GetAllChallenges").Return([]*commonDomain.Challenge{})

	// Execute request
	handler.ServeHTTP(w, req)

	// Should succeed with default test user ID
	assert.Equal(t, http.StatusOK, w.Code)

	mockCache.AssertExpectations(t)
}

// TestOptimizedChallengesHandler_ExtractUserID_WithCustomHeader tests custom user ID header
func TestOptimizedChallengesHandler_ExtractUserID_WithCustomHeader(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Create request with custom x-mock-user-id header
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "custom-user-123")
	w := httptest.NewRecorder()

	// Setup mock - should be called with custom user ID
	mockCache.On("GetAllChallenges").Return([]*commonDomain.Challenge{})

	// Execute request
	handler.ServeHTTP(w, req)

	// Should succeed
	assert.Equal(t, http.StatusOK, w.Code)

	mockCache.AssertExpectations(t)
}

// TestDecodeJWTClaims_Success tests successful JWT decoding
func TestDecodeJWTClaims_Success(t *testing.T) {
	// Create a valid JWT-like token: header.payload.signature
	// Payload: {"sub":"user123","namespace":"test","exp":1700000000}
	encodedPayload := "eyJzdWIiOiJ1c2VyMTIzIiwibmFtZXNwYWNlIjoidGVzdCIsImV4cCI6MTcwMDAwMDAwMH0"
	token := "header." + encodedPayload + ".signature"

	claims, err := decodeJWTClaims(token)

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user123", claims.Sub)
	assert.Equal(t, "test", claims.Namespace)
	assert.Equal(t, int64(1700000000), claims.Exp)
}

// TestDecodeJWTClaims_InvalidFormat tests JWT with wrong number of parts
func TestDecodeJWTClaims_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "only one part",
			token: "onlyonepart",
		},
		{
			name:  "only two parts",
			token: "header.payload",
		},
		{
			name:  "four parts",
			token: "header.payload.signature.extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := decodeJWTClaims(tt.token)

			assert.Error(t, err)
			assert.Nil(t, claims)
			assert.Contains(t, err.Error(), "invalid JWT format")
		})
	}
}

// TestDecodeJWTClaims_InvalidBase64 tests JWT with invalid base64 encoding
func TestDecodeJWTClaims_InvalidBase64(t *testing.T) {
	// Invalid base64 in payload part
	token := "header.invalid!!!base64.signature"

	claims, err := decodeJWTClaims(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to decode JWT payload")
}

// TestDecodeJWTClaims_InvalidJSON tests JWT with invalid JSON in payload
func TestDecodeJWTClaims_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON: "not json"
	// base64 encode of "not json" is "bm90IGpzb24"
	token := "header.bm90IGpzb24.signature"

	claims, err := decodeJWTClaims(token)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse JWT claims")
}

// TestAuthError_Error tests authError.Error() method
func TestAuthError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *authError
		expected string
	}{
		{
			name: "error without cause",
			err: &authError{
				message: "authentication failed",
				cause:   nil,
			},
			expected: "authentication failed",
		},
		{
			name: "error with cause",
			err: &authError{
				message: "authentication failed",
				cause:   assert.AnError,
			},
			expected: "authentication failed: assert.AnError general error for testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAuthError_Unwrap tests authError.Unwrap() method
func TestAuthError_Unwrap(t *testing.T) {
	tests := []struct {
		name     string
		err      *authError
		expected error
	}{
		{
			name: "error without cause",
			err: &authError{
				message: "authentication failed",
				cause:   nil,
			},
			expected: nil,
		},
		{
			name: "error with cause",
			err: &authError{
				message: "authentication failed",
				cause:   assert.AnError,
			},
			expected: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Unwrap()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockTokenValidator is a mock implementation of token validator
type MockTokenValidator struct {
	mock.Mock
}

func (m *MockTokenValidator) Initialize(ctx ...context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTokenValidator) Validate(token string, permission *iam.Permission, namespace *string, userId *string) error {
	args := m.Called(token, permission, namespace, userId)
	return args.Error(0)
}

// TestExtractUserID_AuthEnabled_MissingAuthHeader tests missing Authorization header
func TestExtractUserID_AuthEnabled_MissingAuthHeader(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Create request without Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)

	userID, err := handler.extractUserID(req)

	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "missing authorization header")
}

// TestExtractUserID_AuthEnabled_InvalidFormat tests invalid Authorization header format
func TestExtractUserID_AuthEnabled_InvalidFormat(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "no Bearer prefix",
			header: "sometoken",
		},
		{
			name:   "wrong prefix",
			header: "Basic sometoken",
		},
		{
			name:   "Bearer without space",
			header: "Bearertoken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
			req.Header.Set("Authorization", tt.header)

			userID, err := handler.extractUserID(req)

			assert.Error(t, err)
			assert.Empty(t, userID)
			assert.Contains(t, err.Error(), "invalid authorization header format")
		})
	}
}

// TestExtractUserID_AuthEnabled_TokenValidationFails tests token validation failure
func TestExtractUserID_AuthEnabled_TokenValidationFails(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Setup mock to return validation error
	mockValidator.On("Validate", "invalidtoken", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")

	userID, err := handler.extractUserID(req)

	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "invalid token")

	mockValidator.AssertExpectations(t)
}

// TestExtractUserID_AuthEnabled_InvalidJWT tests JWT decode failure
func TestExtractUserID_AuthEnabled_InvalidJWT(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Token with invalid format (not 3 parts)
	invalidToken := "invalid.jwt"

	// Setup mock to pass validation (but JWT decode will fail)
	mockValidator.On("Validate", invalidToken, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("Authorization", "Bearer "+invalidToken)

	userID, err := handler.extractUserID(req)

	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "failed to decode JWT claims")

	mockValidator.AssertExpectations(t)
}

// TestExtractUserID_AuthEnabled_MissingSubClaim tests JWT without sub claim
func TestExtractUserID_AuthEnabled_MissingSubClaim(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Create JWT with empty sub claim: {"sub":"","namespace":"test","exp":1700000000}
	// Base64 of: {"sub":"","namespace":"test","exp":1700000000}
	payload := "eyJzdWIiOiIiLCJuYW1lc3BhY2UiOiJ0ZXN0IiwiZXhwIjoxNzAwMDAwMDAwfQ"
	token := "header." + payload + ".signature"

	// Setup mock to pass validation
	mockValidator.On("Validate", token, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := handler.extractUserID(req)

	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "user ID not found in token claims")

	mockValidator.AssertExpectations(t)
}

// TestExtractUserID_AuthEnabled_Success tests successful user ID extraction with auth
func TestExtractUserID_AuthEnabled_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()
	mockValidator := new(MockTokenValidator)

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Create valid JWT: {"sub":"user123","namespace":"test","exp":1700000000}
	payload := "eyJzdWIiOiJ1c2VyMTIzIiwibmFtZXNwYWNlIjoidGVzdCIsImV4cCI6MTcwMDAwMDAwMH0"
	token := "header." + payload + ".signature"

	// Setup mock to pass validation
	mockValidator.On("Validate", token, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := handler.extractUserID(req)

	assert.NoError(t, err)
	assert.Equal(t, "user123", userID)

	mockValidator.AssertExpectations(t)
}

// TestExtractUserID_AuthEnabled_NilValidator tests nil token validator
func TestExtractUserID_AuthEnabled_NilValidator(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	serCache := cache.NewSerializedChallengeCache()

	handler := NewOptimizedChallengesHandler(
		mockCache,
		mockRepo,
		serCache,
		"test-namespace",
		true, // auth enabled
		nil,  // nil validator
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("Authorization", "Bearer sometoken")

	userID, err := handler.extractUserID(req)

	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "token validator not initialized")
}
