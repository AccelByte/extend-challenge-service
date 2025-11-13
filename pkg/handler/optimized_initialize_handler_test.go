// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"extend-challenge-service/pkg/service"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

func TestOptimizedInitializeHandler_ServeHTTP_Success(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create handler with auth disabled for simplicity
	handler := NewOptimizedInitializeHandler(
		mockCache,
		mockRepo,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Setup expectations
	defaultGoals := createTestDefaultGoals()
	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockCache.On("GetGoalByID", "daily-login").Return(defaultGoals[0])
	mockCache.On("GetGoalByID", "play-match").Return(defaultGoals[1])
	mockRepo.On("GetUserGoalCount", mock.Anything, "test-user").Return(0, nil)
	mockRepo.On("BulkInsert", mock.Anything, mock.AnythingOfType("[]*domain.UserGoalProgress")).Return(nil)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "test-user")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Parse response
	var response InitializeResponseDTO
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// Verify response structure
	assert.Equal(t, int32(2), response.NewAssignments)
	assert.Equal(t, int32(2), response.TotalActive)
	assert.Len(t, response.AssignedGoals, 2)

	// Verify first goal
	goal := response.AssignedGoals[0]
	assert.Equal(t, "daily-challenge", goal.ChallengeID)
	assert.Equal(t, "daily-login", goal.GoalID)
	assert.Equal(t, "Login Daily", goal.Name)
	assert.True(t, goal.IsActive)
	assert.Equal(t, int32(0), goal.Progress)
	assert.Equal(t, int32(1), goal.Target)
	assert.NotEmpty(t, goal.AssignedAt)

	// Verify requirement
	assert.NotNil(t, goal.Requirement)
	assert.Equal(t, "login_count", goal.Requirement.StatCode)
	assert.Equal(t, ">=", goal.Requirement.Operator)
	assert.Equal(t, int32(1), goal.Requirement.TargetValue)

	// Verify reward
	assert.NotNil(t, goal.Reward)
	assert.Equal(t, "WALLET", goal.Reward.Type)
	assert.Equal(t, "gold", goal.Reward.RewardID)
	assert.Equal(t, int32(100), goal.Reward.Quantity)

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedInitializeHandler_ServeHTTP_AlreadyInitialized(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create handler with auth disabled
	handler := NewOptimizedInitializeHandler(
		mockCache,
		mockRepo,
		"test-namespace",
		false,
		nil,
	)

	// Setup expectations for already initialized user
	defaultGoals := createTestDefaultGoals()
	activeProgress := createTestActiveProgress()

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockCache.On("GetGoalByID", "daily-login").Return(defaultGoals[0])
	mockCache.On("GetGoalByID", "play-match").Return(defaultGoals[1])
	mockRepo.On("GetUserGoalCount", mock.Anything, "test-user").Return(2, nil)
	mockRepo.On("GetActiveGoals", mock.Anything, "test-user").Return(activeProgress, nil)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "test-user")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response InitializeResponseDTO
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// Verify response - no new assignments (fast path)
	assert.Equal(t, int32(0), response.NewAssignments)
	assert.Equal(t, int32(2), response.TotalActive)
	assert.Len(t, response.AssignedGoals, 2)

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedInitializeHandler_ServeHTTP_NoDefaultGoals(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create handler
	handler := NewOptimizedInitializeHandler(
		mockCache,
		mockRepo,
		"test-namespace",
		false,
		nil,
	)

	// Setup expectations - no default goals
	mockCache.On("GetGoalsWithDefaultAssigned").Return([]*commonDomain.Goal{})

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "test-user")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response InitializeResponseDTO
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// Verify empty response
	assert.Equal(t, int32(0), response.NewAssignments)
	assert.Equal(t, int32(0), response.TotalActive)
	assert.Len(t, response.AssignedGoals, 0)

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedInitializeHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create handler
	handler := NewOptimizedInitializeHandler(
		mockCache,
		mockRepo,
		"test-namespace",
		false,
		nil,
	)

	// Create GET request (should be POST)
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "test-user")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Assert response
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestOptimizedInitializeHandler_ServeHTTP_DatabaseError(t *testing.T) {
	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Create handler
	handler := NewOptimizedInitializeHandler(
		mockCache,
		mockRepo,
		"test-namespace",
		false,
		nil,
	)

	// Setup expectations with database error
	defaultGoals := createTestDefaultGoals()
	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockRepo.On("GetUserGoalCount", mock.Anything, "test-user").Return(0, assert.AnError)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "test-user")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestOptimizedInitializeHandler_ExtractUserID_AuthDisabled(t *testing.T) {
	// Create handler with auth disabled
	handler := NewOptimizedInitializeHandler(
		nil,
		nil,
		"test-namespace",
		false, // auth disabled
		nil,
	)

	// Test with custom user ID header
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", "custom-user")

	userID, err := handler.extractUserID(req)
	assert.NoError(t, err)
	assert.Equal(t, "custom-user", userID)

	// Test without header (should use default)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)

	userID2, err := handler.extractUserID(req2)
	assert.NoError(t, err)
	assert.Equal(t, "test-user-id", userID2)
}

func TestOptimizedInitializeHandler_ExtractUserID_AuthEnabled_MissingToken(t *testing.T) {
	// Create mock validator
	mockValidator := new(MockTokenValidator)

	// Create handler with auth enabled
	handler := NewOptimizedInitializeHandler(
		nil,
		nil,
		"test-namespace",
		true, // auth enabled
		mockValidator,
	)

	// Test without Authorization header
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)

	userID, err := handler.extractUserID(req)
	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "missing authorization header")
}

func TestOptimizedInitializeHandler_ExtractUserID_AuthEnabled_InvalidFormat(t *testing.T) {
	// Create mock validator
	mockValidator := new(MockTokenValidator)

	// Create handler with auth enabled
	handler := NewOptimizedInitializeHandler(
		nil,
		nil,
		"test-namespace",
		true,
		mockValidator,
	)

	// Test with invalid authorization format (missing "Bearer ")
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("Authorization", "InvalidToken")

	userID, err := handler.extractUserID(req)
	assert.Error(t, err)
	assert.Empty(t, userID)
	assert.Contains(t, err.Error(), "invalid authorization header format")
}

func TestToInitializeResponseDTO_NilResult(t *testing.T) {
	dto := toInitializeResponseDTO(nil)
	assert.NotNil(t, dto)
	assert.Equal(t, int32(0), dto.NewAssignments)
	assert.Equal(t, int32(0), dto.TotalActive)
	assert.Len(t, dto.AssignedGoals, 0)
}

func TestToAssignedGoalDTO_WithTimestamps(t *testing.T) {
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)

	goal := &service.AssignedGoal{
		ChallengeID: "test-challenge",
		GoalID:      "test-goal",
		Name:        "Test Goal",
		Description: "Test Description",
		IsActive:    true,
		AssignedAt:  &now,
		ExpiresAt:   &expires,
		Progress:    5,
		Target:      10,
		Status:      "in_progress",
		Requirement: commonDomain.Requirement{
			StatCode:    "test_stat",
			Operator:    ">=",
			TargetValue: 10,
		},
		Reward: commonDomain.Reward{
			Type:     "ITEM",
			RewardID: "item123",
			Quantity: 1,
		},
	}

	dto := toAssignedGoalDTO(goal)

	assert.NotNil(t, dto)
	assert.Equal(t, "test-challenge", dto.ChallengeID)
	assert.Equal(t, "test-goal", dto.GoalID)
	assert.Equal(t, "Test Goal", dto.Name)
	assert.True(t, dto.IsActive)
	assert.NotEmpty(t, dto.AssignedAt)
	assert.NotEmpty(t, dto.ExpiresAt)
	assert.Equal(t, int32(5), dto.Progress)
	assert.Equal(t, int32(10), dto.Target)
	assert.Equal(t, "in_progress", dto.Status)
}

func TestToAssignedGoalDTO_NilGoal(t *testing.T) {
	dto := toAssignedGoalDTO(nil)
	assert.Nil(t, dto)
}

// Helper function to create test default goals
func createTestDefaultGoals() []*commonDomain.Goal {
	return []*commonDomain.Goal{
		{
			ID:              "daily-login",
			ChallengeID:     "daily-challenge",
			Name:            "Login Daily",
			Description:     "Login to the game",
			Type:            commonDomain.GoalTypeAbsolute,
			EventSource:     commonDomain.EventSourceLogin,
			DefaultAssigned: true,
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
		{
			ID:              "play-match",
			ChallengeID:     "daily-challenge",
			Name:            "Play Match",
			Description:     "Play one match",
			Type:            commonDomain.GoalTypeAbsolute,
			EventSource:     commonDomain.EventSourceStatistic,
			DefaultAssigned: true,
			Requirement: commonDomain.Requirement{
				StatCode:    "matches_played",
				Operator:    ">=",
				TargetValue: 1,
			},
			Reward: commonDomain.Reward{
				Type:     "WALLET",
				RewardID: "gold",
				Quantity: 50,
			},
		},
	}
}

// Helper function to create test active progress
func createTestActiveProgress() []*commonDomain.UserGoalProgress {
	now := time.Now().UTC()
	return []*commonDomain.UserGoalProgress{
		{
			UserID:      "test-user",
			GoalID:      "daily-login",
			ChallengeID: "daily-challenge",
			Namespace:   "test-namespace",
			Progress:    0,
			Status:      commonDomain.GoalStatusNotStarted,
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      "test-user",
			GoalID:      "play-match",
			ChallengeID: "daily-challenge",
			Namespace:   "test-namespace",
			Progress:    0,
			Status:      commonDomain.GoalStatusNotStarted,
			IsActive:    true,
			AssignedAt:  &now,
		},
	}
}
