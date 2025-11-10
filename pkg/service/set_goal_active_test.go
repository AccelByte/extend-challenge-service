package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// NOTE: MockGoalCache and MockGoalRepository are defined in progress_query_test.go
// to avoid duplication across test files.

// Test SetGoalActive - Happy Path: Activate Goal (Creates Row)
func TestSetGoalActive_ActivateGoal_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	// Create mock goal
	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: challengeID,
		Name:        "Defeat 10 Enemies",
		Description: "Defeat 10 enemies in combat",
		Type:        domain.GoalTypeAbsolute,
		EventSource: domain.EventSourceStatistic,
		Requirement: domain.Requirement{
			StatCode:    "enemy_kills",
			Operator:    ">=",
			TargetValue: 10,
		},
		Reward: domain.Reward{
			Type:     string(domain.RewardTypeItem),
			RewardID: "bronze_sword",
			Quantity: 1,
		},
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)

	// Expect UpsertGoalActive call
	mockRepo.On("UpsertGoalActive", ctx, mock.MatchedBy(func(progress *domain.UserGoalProgress) bool {
		assert.Equal(t, userID, progress.UserID)
		assert.Equal(t, goalID, progress.GoalID)
		assert.Equal(t, challengeID, progress.ChallengeID)
		assert.Equal(t, namespace, progress.Namespace)
		assert.Equal(t, 0, progress.Progress)
		assert.Equal(t, domain.GoalStatusNotStarted, progress.Status)
		assert.True(t, progress.IsActive)
		assert.NotNil(t, progress.AssignedAt)
		return true
	})).Return(nil)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challengeID, result.ChallengeID)
	assert.Equal(t, goalID, result.GoalID)
	assert.True(t, result.IsActive)
	assert.NotNil(t, result.AssignedAt)
	assert.Equal(t, "Goal activated successfully", result.Message)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test SetGoalActive - Happy Path: Deactivate Goal (Updates Row)
func TestSetGoalActive_DeactivateGoal_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := false

	// Create mock goal
	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: challengeID,
		Name:        "Defeat 10 Enemies",
		Description: "Defeat 10 enemies in combat",
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)

	// Expect UpsertGoalActive call with is_active=false
	mockRepo.On("UpsertGoalActive", ctx, mock.MatchedBy(func(progress *domain.UserGoalProgress) bool {
		assert.Equal(t, userID, progress.UserID)
		assert.Equal(t, goalID, progress.GoalID)
		assert.False(t, progress.IsActive)
		return true
	})).Return(nil)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challengeID, result.ChallengeID)
	assert.Equal(t, goalID, result.GoalID)
	assert.False(t, result.IsActive)
	assert.Equal(t, "Goal deactivated successfully", result.Message)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test SetGoalActive - Activate Already Active Goal (Idempotent)
func TestSetGoalActive_ActivateAlreadyActive_Idempotent(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: challengeID,
		Name:        "Test Goal",
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)
	mockRepo.On("UpsertGoalActive", ctx, mock.Anything).Return(nil)

	// Call function twice
	result1, err1 := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)
	result2, err2 := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Both calls should succeed
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.True(t, result1.IsActive)
	assert.True(t, result2.IsActive)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test SetGoalActive - Invalid Goal ID (404 Error)
func TestSetGoalActive_InvalidGoalID_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "invalid-goal"
	namespace := "test-namespace"
	isActive := true

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Goal not found in cache
	mockCache.On("GetGoalByID", goalID).Return((*domain.Goal)(nil))

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")

	mockCache.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "UpsertGoalActive")
}

// Test SetGoalActive - Goal Belongs to Different Challenge (400 Error)
func TestSetGoalActive_WrongChallenge_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	// Mock goal belongs to different challenge
	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: "challenge2", // Different challenge!
		Name:        "Test Goal",
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to")

	mockCache.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "UpsertGoalActive")
}

// Test SetGoalActive - Database Error
func TestSetGoalActive_DatabaseError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: challengeID,
		Name:        "Test Goal",
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)
	mockRepo.On("UpsertGoalActive", ctx, mock.Anything).Return(errors.New("database error"))

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update goal active status")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test SetGoalActive - Empty UserID Validation
func TestSetGoalActive_EmptyUserID_Error(t *testing.T) {
	ctx := context.Background()
	userID := ""
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user ID cannot be empty")

	mockCache.AssertNotCalled(t, "GetGoalByID")
	mockRepo.AssertNotCalled(t, "UpsertGoalActive")
}

// Test SetGoalActive - Empty ChallengeID Validation
func TestSetGoalActive_EmptyChallengeID_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := ""
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "challenge ID cannot be empty")
}

// Test SetGoalActive - Empty GoalID Validation
func TestSetGoalActive_EmptyGoalID_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := ""
	namespace := "test-namespace"
	isActive := true

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "goal ID cannot be empty")
}

// Test SetGoalActive - Empty Namespace Validation
func TestSetGoalActive_EmptyNamespace_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := ""
	isActive := true

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Call function
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

// Test SetGoalActive - Nil GoalCache Validation
func TestSetGoalActive_NilGoalCache_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockRepo := new(MockGoalRepository)

	// Call function with nil cache
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, nil, mockRepo)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "goal cache cannot be nil")
}

// Test SetGoalActive - Nil Repository Validation
func TestSetGoalActive_NilRepository_Error(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockCache := new(MockGoalCache)

	// Call function with nil repo
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, nil)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

// Test SetGoalActive - AssignedAt Timestamp is Set
func TestSetGoalActive_AssignedAtTimestamp(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge1"
	goalID := "goal1"
	namespace := "test-namespace"
	isActive := true

	mockGoal := &domain.Goal{
		ID:          goalID,
		ChallengeID: challengeID,
		Name:        "Test Goal",
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalByID", goalID).Return(mockGoal)

	var capturedAssignedAt *time.Time
	mockRepo.On("UpsertGoalActive", ctx, mock.MatchedBy(func(progress *domain.UserGoalProgress) bool {
		capturedAssignedAt = progress.AssignedAt
		return true
	})).Return(nil)

	before := time.Now()
	result, err := SetGoalActive(ctx, userID, challengeID, goalID, namespace, isActive, mockCache, mockRepo)
	after := time.Now()

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, result.AssignedAt)
	require.NotNil(t, capturedAssignedAt)

	// Verify timestamp is within reasonable range
	assert.True(t, !capturedAssignedAt.Before(before), "AssignedAt should not be before function call")
	assert.True(t, !capturedAssignedAt.After(after), "AssignedAt should not be after function call")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}
