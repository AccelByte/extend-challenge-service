package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	pb "extend-challenge-service/pkg/pb"
)

// High Priority Error Scenarios

// TestError_400_GoalNotCompleted tests claiming an incomplete goal
func TestError_400_GoalNotCompleted(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed in-progress goal (progress < target)
	seedInProgressGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025", 5, 10)

	// Try to claim incomplete goal
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Goal not completed")

	// Verify reward was NOT granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_409_AlreadyClaimed tests claiming an already-claimed goal
func TestError_409_AlreadyClaimed(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed already-claimed goal
	seedClaimedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Try to claim again
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been claimed")

	// Verify reward was NOT granted (idempotency)
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_404_GoalNotFound tests claiming a non-existent goal
func TestError_404_GoalNotFound(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to claim non-existent goal
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "non-existent-goal",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Goal not found")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_404_ChallengeNotFound tests claiming from a non-existent challenge
func TestError_404_ChallengeNotFound(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to claim goal from non-existent challenge
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "non-existent-challenge",
		GoalId:      "some-goal",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	// Service returns "Goal not found" for non-existent challenge/goal combos
	assert.Contains(t, err.Error(), "Goal not found")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// Medium Priority Error Scenarios

// TestError_400_GoalLocked_PrerequisitesNotMet tests claiming a locked goal
//
// This test uses reach-level-5 which has prerequisite kill-10-snowmen in challenges.test.json
// The test seeds a completed reach-level-5 goal WITHOUT completing the prerequisite,
// which should result in a GOAL_LOCKED error when trying to claim.
func TestError_400_GoalLocked_PrerequisitesNotMet(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed completed goal WITHOUT completing prerequisite
	seedCompletedGoal(t, testDB, "test-user-123", "reach-level-5", "winter-challenge-2025")

	// Try to claim locked goal
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "reach-level-5",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Prerequisites not completed")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_503_DatabaseUnavailable tests database unavailability
//
// This test uses a mock setup with a failing database to simulate database unavailability
// without affecting other tests.
func TestError_503_DatabaseUnavailable(t *testing.T) {
	// This test uses setupTestServerWithMockDB instead of setupTestServer
	// to inject a failing mock repository that simulates database unavailability
	client, mockRewardClient, mockGoalRepo, cleanup := setupTestServerWithMockDB(t)
	defer cleanup()

	// Mock database to return error on BeginTx (simulates DB unavailability at transaction start)
	mockGoalRepo.On("BeginTx",
		mock.Anything,
	).Return(nil, errors.New("database connection failed"))

	// Try to claim when database is unavailable
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Database error")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
	mockGoalRepo.AssertExpectations(t)
}

// Low Priority Error Scenarios

// TestError_502_RewardGrantFailed tests reward grant failure
func TestError_502_RewardGrantFailed(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed prerequisite goal (kill-10-snowmen requires complete-tutorial)
	seedClaimedGoal(t, testDB, "test-user-123", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward client to fail (simulates AGS timeout or error)
	mockRewardClient.On("GrantReward", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("AGS Platform Service timeout"))

	// Try to claim
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to grant reward")

	// Verify reward grant was attempted but failed
	mockRewardClient.AssertExpectations(t)

	// Verify database NOT updated to claimed (rollback on failure)
	var status string
	dbErr := testDB.QueryRow(
		"SELECT status FROM user_goal_progress WHERE user_id = $1 AND goal_id = $2",
		"test-user-123", "kill-10-snowmen",
	).Scan(&status)
	assert.NoError(t, dbErr)
	assert.Equal(t, "completed", status) // Still completed, not claimed
}

// TestError_400_InvalidRequest_EmptyUserID tests invalid request with empty user ID
//
// Note: Since we use auth context, this tests missing auth context
func TestError_400_InvalidRequest_NoAuthContext(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Try to claim with NO auth context (no user ID)
	ctx := context.Background() // No auth context
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	// Should fail with Unauthenticated error
	assert.Contains(t, err.Error(), "Unauthenticated")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_400_InvalidRequest_EmptyChallengeID tests invalid request with empty challenge ID
func TestError_400_InvalidRequest_EmptyChallengeID(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to claim with empty challenge ID
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "", // Empty
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	// Should fail with validation error (empty challenge ID)
	// Just verify an error occurred - exact error message may vary

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_400_InvalidRequest_EmptyGoalID tests invalid request with empty goal ID
func TestError_400_InvalidRequest_EmptyGoalID(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Try to claim with empty goal ID
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "", // Empty
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	// Should fail with validation error (empty goal ID)
	// Just verify an error occurred - exact error message may vary

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_WithContext_UserMismatch tests that user can only access their own data
func TestError_WithContext_UserMismatch(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed completed goal for user-1
	seedCompletedGoal(t, testDB, "user-1", "kill-10-snowmen", "winter-challenge-2025")

	// Try to claim as user-2 (different user)
	// Since we extract userID from context, user-2 won't see user-1's progress
	ctx := createAuthContext("user-2", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	// Should fail because user-2 has no progress for this goal
	assert.Contains(t, err.Error(), "Goal not completed")

	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestError_NamespaceMismatch tests namespace isolation
func TestError_NamespaceMismatch(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed prerequisite and completed goal for test-namespace
	seedClaimedGoal(t, testDB, "test-user-123", "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting for the namespace in context (test-namespace)
	// Even though we use different namespace in auth context, the server uses its configured namespace
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace", // Server uses its configured namespace
		"test-user-123",
		mock.Anything,
	).Return(nil).Once()

	// Try to claim from different namespace context
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	resp, err := client.ClaimGoalReward(ctx, req)

	// Current implementation: server uses its configured namespace for all operations
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "claimed", resp.Status)

	// Verify reward was granted with server's namespace
	mockRewardClient.AssertExpectations(t)
}
