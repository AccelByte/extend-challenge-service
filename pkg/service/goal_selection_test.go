// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

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

// Test RandomSelectGoals - Happy Path: Select 3 Random Goals
func TestRandomSelectGoals_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 3

	// Create challenge with 10 goals
	challenge := createTestChallengeWithGoals(challengeID, 10)

	// User has no progress yet (all goals available)
	userProgress := []*domain.UserGoalProgress{}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.AnythingOfType("[]*domain.UserGoalProgress")).Return(nil)
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, count, len(result.SelectedGoals))
	assert.Equal(t, challengeID, result.ChallengeID)
	assert.Equal(t, count, result.TotalActiveGoals)
	assert.Empty(t, result.ReplacedGoals)

	// Verify all selected goals are from the challenge
	goalIDs := make(map[string]bool)
	for _, selectedGoal := range result.SelectedGoals {
		goalIDs[selectedGoal.GoalID] = true
		assert.True(t, selectedGoal.IsActive)
		assert.Equal(t, 0, selectedGoal.Progress)
		assert.NotNil(t, selectedGoal.AssignedAt)
	}
	assert.Equal(t, count, len(goalIDs))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test RandomSelectGoals - Replace Existing Goals
func TestRandomSelectGoals_ReplaceExisting(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 3

	// Create challenge with 10 goals
	challenge := createTestChallengeWithGoals(challengeID, 10)

	// User has 2 active goals already
	now := time.Now()
	userProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      challenge.Goals[0].ID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      challenge.Goals[1].ID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			Progress:    2,
			Status:      domain.GoalStatusInProgress,
			AssignedAt:  &now,
		},
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	// Expect deactivation batch
	mockTx.On("BatchUpsertGoalActive", ctx, mock.MatchedBy(func(progresses []*domain.UserGoalProgress) bool {
		// First call: deactivation (2 goals)
		if len(progresses) == 2 {
			for _, p := range progresses {
				if p.IsActive {
					return false
				}
			}
			return true
		}
		return false
	})).Return(nil).Once()
	// Expect activation batch
	mockTx.On("BatchUpsertGoalActive", ctx, mock.MatchedBy(func(progresses []*domain.UserGoalProgress) bool {
		// Second call: activation (3 goals)
		if len(progresses) == 3 {
			for _, p := range progresses {
				if !p.IsActive {
					return false
				}
			}
			return true
		}
		return false
	})).Return(nil).Once()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)

	// Execute with replace_existing = true
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, true, false, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, count, len(result.SelectedGoals))
	assert.Equal(t, count, result.TotalActiveGoals) // Should be exactly count after replace
	assert.Equal(t, 2, len(result.ReplacedGoals))   // 2 goals were replaced

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test RandomSelectGoals - Exclude Active Goals
func TestRandomSelectGoals_ExcludeActive(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 3

	// Create challenge with 5 goals
	challenge := createTestChallengeWithGoals(challengeID, 5)

	// User has 2 active goals already
	now := time.Now()
	userProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      challenge.Goals[0].ID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      challenge.Goals[1].ID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			Progress:    2,
			Status:      domain.GoalStatusInProgress,
			AssignedAt:  &now,
		},
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.AnythingOfType("[]*domain.UserGoalProgress")).Return(nil)
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)

	// Execute with exclude_active = true
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, true, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, count, len(result.SelectedGoals))

	// Verify selected goals are NOT the already-active ones
	for _, selectedGoal := range result.SelectedGoals {
		assert.NotEqual(t, challenge.Goals[0].ID, selectedGoal.GoalID)
		assert.NotEqual(t, challenge.Goals[1].ID, selectedGoal.GoalID)
	}

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test RandomSelectGoals - Partial Results (Fewer Available Than Requested)
func TestRandomSelectGoals_PartialResults(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 5 // Request 5

	// Create challenge with only 3 goals
	challenge := createTestChallengeWithGoals(challengeID, 3)

	userProgress := []*domain.UserGoalProgress{}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.AnythingOfType("[]*domain.UserGoalProgress")).Return(nil)
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert - should return all 3 available goals (partial result)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.SelectedGoals)) // Only 3 available

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test RandomSelectGoals - No Goals Available
func TestRandomSelectGoals_NoGoalsAvailable(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 3

	// Create challenge with 3 goals
	challenge := createTestChallengeWithGoals(challengeID, 3)

	// All goals are already completed
	now := time.Now()
	userProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      challenge.Goals[0].ID,
			ChallengeID: challengeID,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &now,
		},
		{
			UserID:      userID,
			GoalID:      challenge.Goals[1].ID,
			ChallengeID: challengeID,
			Status:      domain.GoalStatusClaimed,
			CompletedAt: &now,
			ClaimedAt:   &now,
		},
		{
			UserID:      userID,
			GoalID:      challenge.Goals[2].ID,
			ChallengeID: challengeID,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &now,
		},
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert - should return error (no goals available)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no goals available")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test RandomSelectGoals - Challenge Not Found
func TestRandomSelectGoals_ChallengeNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "nonexistent"
	namespace := "test-namespace"
	count := 3

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(nil)

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")

	mockCache.AssertExpectations(t)
}

// Test RandomSelectGoals - Invalid Count
func TestRandomSelectGoals_InvalidCount(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 0 // Invalid

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "must be greater than 0")
}

// Test RandomSelectGoals - Database Error
func TestRandomSelectGoals_DatabaseError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	count := 3

	challenge := createTestChallengeWithGoals(challengeID, 10)

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress(nil), errors.New("database error"))

	// Execute
	result, err := RandomSelectGoals(ctx, userID, challengeID, count, false, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get user progress")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test BatchSelectGoals - Happy Path
func TestBatchSelectGoals_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"

	// Create challenge with 10 goals
	challenge := createTestChallengeWithGoals(challengeID, 10)

	// Select 3 specific goals
	goalIDs := []string{
		challenge.Goals[0].ID,
		challenge.Goals[2].ID,
		challenge.Goals[5].ID,
	}

	userProgress := []*domain.UserGoalProgress{}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	for _, goalID := range goalIDs {
		mockCache.On("GetGoalByID", goalID).Return(findGoalByID(challenge, goalID))
	}
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.AnythingOfType("[]*domain.UserGoalProgress")).Return(nil)
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result.SelectedGoals))
	assert.Equal(t, challengeID, result.ChallengeID)
	assert.Equal(t, 3, result.TotalActiveGoals)

	// Verify correct goals were selected
	selectedGoalIDs := make(map[string]bool)
	for _, selectedGoal := range result.SelectedGoals {
		selectedGoalIDs[selectedGoal.GoalID] = true
	}
	for _, goalID := range goalIDs {
		assert.True(t, selectedGoalIDs[goalID])
	}

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test BatchSelectGoals - Goal Not Found
func TestBatchSelectGoals_GoalNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"

	challenge := createTestChallengeWithGoals(challengeID, 10)

	// Include a nonexistent goal ID
	goalIDs := []string{
		challenge.Goals[0].ID,
		"nonexistent-goal",
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockCache.On("GetGoalByID", challenge.Goals[0].ID).Return(challenge.Goals[0])
	mockCache.On("GetGoalByID", "nonexistent-goal").Return(nil)

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")

	mockCache.AssertExpectations(t)
}

// Test BatchSelectGoals - Goal From Different Challenge
func TestBatchSelectGoals_GoalFromDifferentChallenge(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"

	challenge := createTestChallengeWithGoals(challengeID, 10)
	otherChallenge := createTestChallengeWithGoals("other-challenge", 5)

	// Try to select goal from different challenge
	goalIDs := []string{
		challenge.Goals[0].ID,
		otherChallenge.Goals[0].ID, // Wrong challenge!
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockCache.On("GetGoalByID", challenge.Goals[0].ID).Return(challenge.Goals[0])
	mockCache.On("GetGoalByID", otherChallenge.Goals[0].ID).Return(otherChallenge.Goals[0])

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to challenge")

	mockCache.AssertExpectations(t)
}

// Test BatchSelectGoals - Empty Goal List
func TestBatchSelectGoals_EmptyGoalList(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Execute with empty goal list
	result, err := BatchSelectGoals(ctx, userID, challengeID, []string{}, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// Test BatchSelectGoals - Begin Transaction Error
func TestBatchSelectGoals_BeginTransactionError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	goalIDs := []string{"goal-A", "goal-B"}

	challenge := createTestChallengeWithGoals(challengeID, 3)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	for _, goalID := range goalIDs {
		mockCache.On("GetGoalByID", goalID).Return(challenge.Goals[0])
	}
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress{}, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, errors.New("transaction error"))

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to begin transaction")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test BatchSelectGoals - Activation Error
func TestBatchSelectGoals_ActivationError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	goalIDs := []string{"goal-A", "goal-B"}

	challenge := createTestChallengeWithGoals(challengeID, 3)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	for _, goalID := range goalIDs {
		mockCache.On("GetGoalByID", goalID).Return(challenge.Goals[0])
	}
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress{}, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.Anything).Return(errors.New("activation error"))
	mockTx.On("Rollback").Return(nil)

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to batch activate goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test BatchSelectGoals - Commit Error
func TestBatchSelectGoals_CommitError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	goalIDs := []string{"goal-A", "goal-B"}

	challenge := createTestChallengeWithGoals(challengeID, 3)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	for _, goalID := range goalIDs {
		mockCache.On("GetGoalByID", goalID).Return(challenge.Goals[0])
	}
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress{}, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	mockTx.On("BatchUpsertGoalActive", ctx, mock.Anything).Return(nil)
	mockTx.On("Commit").Return(errors.New("commit error"))
	mockTx.On("Rollback").Return(nil)

	// Execute
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, false, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to commit transaction")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test BatchSelectGoals - Deactivation Error in Replace Mode
func TestBatchSelectGoals_DeactivationError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "daily-challenge"
	namespace := "test-namespace"
	goalIDs := []string{"goal-C"}

	challenge := createTestChallengeWithGoals(challengeID, 3)
	now := time.Now()

	// Existing active goals
	userProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      "goal-A",
			ChallengeID: challengeID,
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-B",
			ChallengeID: challengeID,
			IsActive:    true,
			AssignedAt:  &now,
		},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTx := new(MockTxRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockCache.On("GetGoalByID", "goal-C").Return(challenge.Goals[2])
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(userProgress, nil)
	mockRepo.On("BeginTx", ctx).Return(mockTx, nil)
	// First call to deactivate should fail
	mockTx.On("BatchUpsertGoalActive", ctx, mock.Anything).Return(errors.New("deactivation error")).Once()
	mockTx.On("Rollback").Return(nil)

	// Execute with replace mode
	result, err := BatchSelectGoals(ctx, userID, challengeID, goalIDs, true, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to deactivate goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

// Test BatchSelectGoals - Nil Cache
func TestBatchSelectGoals_NilCache(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockGoalRepository)

	result, err := BatchSelectGoals(ctx, "user123", "challenge", []string{"goal-A"}, false, "namespace", nil, mockRepo)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cache cannot be nil")
}

// Test BatchSelectGoals - Nil Repository
func TestBatchSelectGoals_NilRepository(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)

	result, err := BatchSelectGoals(ctx, "user123", "challenge", []string{"goal-A"}, false, "namespace", mockCache, nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

// Test BatchSelectGoals - Empty UserID
func TestBatchSelectGoals_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	result, err := BatchSelectGoals(ctx, "", "challenge", []string{"goal-A"}, false, "namespace", mockCache, mockRepo)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

// Test BatchSelectGoals - Empty ChallengeID
func TestBatchSelectGoals_EmptyChallengeID(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	result, err := BatchSelectGoals(ctx, "user123", "", []string{"goal-A"}, false, "namespace", mockCache, mockRepo)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "challenge ID cannot be empty")
}

// Test BatchSelectGoals - Empty Namespace
func TestBatchSelectGoals_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	result, err := BatchSelectGoals(ctx, "user123", "challenge", []string{"goal-A"}, false, "", mockCache, mockRepo)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

// Test BatchSelectGoals - Challenge Not Found
func TestBatchSelectGoals_ChallengeNotFound(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", "nonexistent").Return(nil)

	result, err := BatchSelectGoals(ctx, "user123", "nonexistent", []string{"goal-A"}, false, "namespace", mockCache, mockRepo)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")

	mockCache.AssertExpectations(t)
}

// Test filterAvailableGoals - All Goals Available
func TestFilterAvailableGoals_AllAvailable(t *testing.T) {
	challenge := createTestChallengeWithGoals("test-challenge", 5)
	userProgress := make(map[string]*domain.UserGoalProgress)
	mockCache := new(MockGoalCache)

	available := filterAvailableGoals(challenge.Goals, userProgress, false, mockCache)

	assert.Equal(t, 5, len(available))
}

// Test filterAvailableGoals - Exclude Completed Goals
func TestFilterAvailableGoals_ExcludeCompleted(t *testing.T) {
	challenge := createTestChallengeWithGoals("test-challenge", 5)
	now := time.Now()
	userProgress := map[string]*domain.UserGoalProgress{
		challenge.Goals[0].ID: {
			UserID:      "user123",
			GoalID:      challenge.Goals[0].ID,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &now,
		},
		challenge.Goals[1].ID: {
			UserID:      "user123",
			GoalID:      challenge.Goals[1].ID,
			Status:      domain.GoalStatusClaimed,
			CompletedAt: &now,
			ClaimedAt:   &now,
		},
	}
	mockCache := new(MockGoalCache)

	available := filterAvailableGoals(challenge.Goals, userProgress, false, mockCache)

	// Should exclude 2 completed/claimed goals
	assert.Equal(t, 3, len(available))
	for _, goalID := range available {
		assert.NotEqual(t, challenge.Goals[0].ID, goalID)
		assert.NotEqual(t, challenge.Goals[1].ID, goalID)
	}
}

// Test filterAvailableGoals - Exclude Active Goals
func TestFilterAvailableGoals_ExcludeActive(t *testing.T) {
	challenge := createTestChallengeWithGoals("test-challenge", 5)
	now := time.Now()
	userProgress := map[string]*domain.UserGoalProgress{
		challenge.Goals[0].ID: {
			UserID:     "user123",
			GoalID:     challenge.Goals[0].ID,
			IsActive:   true,
			AssignedAt: &now,
		},
		challenge.Goals[1].ID: {
			UserID:     "user123",
			GoalID:     challenge.Goals[1].ID,
			IsActive:   true,
			AssignedAt: &now,
		},
	}
	mockCache := new(MockGoalCache)

	available := filterAvailableGoals(challenge.Goals, userProgress, true, mockCache)

	// Should exclude 2 active goals
	assert.Equal(t, 3, len(available))
	for _, goalID := range available {
		assert.NotEqual(t, challenge.Goals[0].ID, goalID)
		assert.NotEqual(t, challenge.Goals[1].ID, goalID)
	}
}

// Test randomSample - Correct Count
func TestRandomSample_CorrectCount(t *testing.T) {
	pool := []string{"goal1", "goal2", "goal3", "goal4", "goal5"}
	count := 3

	result, err := randomSample(pool, count)

	require.NoError(t, err)
	assert.Equal(t, count, len(result))

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, goalID := range result {
		assert.False(t, seen[goalID], "Duplicate goal ID in random sample")
		seen[goalID] = true
	}
}

// Test randomSample - All Elements When Count Equals Pool Size
func TestRandomSample_AllElements(t *testing.T) {
	pool := []string{"goal1", "goal2", "goal3"}
	count := 3

	result, err := randomSample(pool, count)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result))
}

// Test randomSample - Empty Pool
func TestRandomSample_EmptyPool(t *testing.T) {
	pool := []string{}
	count := 3

	result, err := randomSample(pool, count)

	require.NoError(t, err)
	assert.Empty(t, result)
}

// Test getActiveGoalIDs
func TestGetActiveGoalIDs(t *testing.T) {
	now := time.Now()
	progressMap := map[string]*domain.UserGoalProgress{
		"goal1": {
			GoalID:     "goal1",
			IsActive:   true,
			AssignedAt: &now,
		},
		"goal2": {
			GoalID:   "goal2",
			IsActive: false,
		},
		"goal3": {
			GoalID:     "goal3",
			IsActive:   true,
			AssignedAt: &now,
		},
	}

	activeGoals := getActiveGoalIDs(progressMap)

	assert.Equal(t, 2, len(activeGoals))
	assert.Contains(t, activeGoals, "goal1")
	assert.Contains(t, activeGoals, "goal3")
}

// Test buildGoalDetails
func TestBuildGoalDetails(t *testing.T) {
	challenge := createTestChallengeWithGoals("test-challenge", 5)
	goalIDs := []string{
		challenge.Goals[0].ID,
		challenge.Goals[2].ID,
	}
	now := time.Now()

	result := buildGoalDetails(challenge, goalIDs, &now)

	assert.Equal(t, 2, len(result))
	for _, selectedGoal := range result {
		assert.True(t, selectedGoal.IsActive)
		assert.Equal(t, 0, selectedGoal.Progress)
		assert.NotNil(t, selectedGoal.AssignedAt)
		assert.Equal(t, now, *selectedGoal.AssignedAt)
	}
}

// Helper: Create test challenge with N goals
func createTestChallengeWithGoals(challengeID string, goalCount int) *domain.Challenge {
	goals := make([]*domain.Goal, goalCount)
	for i := 0; i < goalCount; i++ {
		goals[i] = &domain.Goal{
			ID:          challengeID + "-goal-" + string(rune('A'+i)),
			Name:        "Goal " + string(rune('A'+i)),
			Description: "Test goal " + string(rune('A'+i)),
			ChallengeID: challengeID,
			Type:        domain.GoalTypeAbsolute,
			EventSource: domain.EventSourceStatistic,
			Requirement: domain.Requirement{
				StatCode:    "test_stat",
				Operator:    ">=",
				TargetValue: 10,
			},
			Reward: domain.Reward{
				Type:     string(domain.RewardTypeItem),
				RewardID: "test_item",
				Quantity: 1,
			},
			Prerequisites: []string{},
		}
	}

	return &domain.Challenge{
		ID:          challengeID,
		Name:        "Test Challenge",
		Description: "Test challenge description",
		Goals:       goals,
	}
}

// Helper: Find goal by ID in challenge
func findGoalByID(challenge *domain.Challenge, goalID string) *domain.Goal {
	for _, goal := range challenge.Goals {
		if goal.ID == goalID {
			return goal
		}
	}
	return nil
}

// NOTE: MockTxRepository is defined in claim_test.go and reused here
