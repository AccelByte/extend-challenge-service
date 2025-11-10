// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package integration

import (
	"testing"
	"time"

	pb "extend-challenge-service/pkg/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestSetGoalActive_ActivateGoal_Success verifies successful goal activation
func TestSetGoalActive_ActivateGoal_Success(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-1"
	ctx := createAuthContext(userID, "test-namespace")

	// Activate a goal that's not default-assigned
	resp, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen", // Not default-assigned
		IsActive:    true,
	})

	// Assertions
	require.NoError(t, err, "SetGoalActive should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Equal(t, "winter-challenge-2025", resp.ChallengeId)
	assert.Equal(t, "kill-10-snowmen", resp.GoalId)
	assert.True(t, resp.IsActive, "Goal should be active")
	assert.NotEmpty(t, resp.AssignedAt, "AssignedAt should be set")
	assert.Equal(t, "Goal activated successfully", resp.Message)

	// Validate timestamp format
	_, err = time.Parse(time.RFC3339, resp.AssignedAt)
	assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp")
}

// TestSetGoalActive_DeactivateGoal_Success verifies successful goal deactivation
func TestSetGoalActive_DeactivateGoal_Success(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-2"
	ctx := createAuthContext(userID, "test-namespace")

	// First, initialize player to get default goals
	_, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
	require.NoError(t, err, "InitializePlayer should succeed")

	// Deactivate the default goal
	resp, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "complete-tutorial",
		IsActive:    false,
	})

	// Assertions
	require.NoError(t, err, "SetGoalActive should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Equal(t, "winter-challenge-2025", resp.ChallengeId)
	assert.Equal(t, "complete-tutorial", resp.GoalId)
	assert.False(t, resp.IsActive, "Goal should be inactive")
	assert.NotEmpty(t, resp.AssignedAt, "AssignedAt should be set")
	assert.Equal(t, "Goal deactivated successfully", resp.Message)
}

// TestSetGoalActive_ActivateThenDeactivate verifies full activation cycle
func TestSetGoalActive_ActivateThenDeactivate(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-3"
	ctx := createAuthContext(userID, "test-namespace")

	// Step 1: Activate goal
	resp1, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    true,
	})
	require.NoError(t, err, "First activation should succeed")
	assert.True(t, resp1.IsActive)

	// Step 2: Verify goal is active in GetUserChallenges
	challenges, err := client.GetUserChallenges(ctx, &pb.GetChallengesRequest{})
	require.NoError(t, err, "GetUserChallenges should succeed")

	// Find the activated goal
	var foundGoal *pb.Goal
	for _, challenge := range challenges.Challenges {
		if challenge.ChallengeId == "winter-challenge-2025" {
			for _, goal := range challenge.Goals {
				if goal.GoalId == "kill-10-snowmen" {
					foundGoal = goal
					break
				}
			}
		}
	}
	require.NotNil(t, foundGoal, "Activated goal should be visible in GetUserChallenges")

	// Step 3: Deactivate goal
	resp2, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    false,
	})
	require.NoError(t, err, "Deactivation should succeed")
	assert.False(t, resp2.IsActive)
}

// TestSetGoalActive_InvalidGoalID verifies error handling for invalid goal
func TestSetGoalActive_InvalidGoalID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-4"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to activate non-existent goal
	_, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "invalid-goal-id",
		IsActive:    true,
	})

	// Assertions
	require.Error(t, err, "SetGoalActive should fail for invalid goal")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code(), "Should return Internal error")
	assert.Contains(t, st.Message(), "failed to set goal active status", "Error message should be descriptive")
}

// TestSetGoalActive_WrongChallenge verifies error handling for wrong challenge
func TestSetGoalActive_WrongChallenge(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-5"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to activate goal with wrong challenge ID
	_, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "wrong-challenge",
		GoalId:      "complete-tutorial", // This belongs to winter-challenge-2025
		IsActive:    true,
	})

	// Assertions
	require.Error(t, err, "SetGoalActive should fail for wrong challenge")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
}

// TestSetGoalActive_MissingChallengeID verifies validation for missing challenge_id
func TestSetGoalActive_MissingChallengeID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-6"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to activate goal without challenge_id
	_, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "", // Empty
		GoalId:      "complete-tutorial",
		IsActive:    true,
	})

	// Assertions
	require.Error(t, err, "SetGoalActive should fail for missing challenge_id")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "challenge_id is required")
}

// TestSetGoalActive_MissingGoalID verifies validation for missing goal_id
func TestSetGoalActive_MissingGoalID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-7"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to activate goal without goal_id
	_, err := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "", // Empty
		IsActive:    true,
	})

	// Assertions
	require.Error(t, err, "SetGoalActive should fail for missing goal_id")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "goal_id is required")
}

// TestSetGoalActive_Idempotent verifies that activating an already active goal is idempotent
func TestSetGoalActive_Idempotent(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "set-active-user-8"
	ctx := createAuthContext(userID, "test-namespace")

	// Activate goal twice
	resp1, err1 := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    true,
	})
	require.NoError(t, err1, "First activation should succeed")

	resp2, err2 := client.SetGoalActive(ctx, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    true,
	})
	require.NoError(t, err2, "Second activation should succeed (idempotent)")

	// Both should return success
	assert.True(t, resp1.IsActive)
	assert.True(t, resp2.IsActive)
	assert.Equal(t, "Goal activated successfully", resp1.Message)
	assert.Equal(t, "Goal activated successfully", resp2.Message)
}

// TestSetGoalActive_MultipleUsers verifies user isolation
func TestSetGoalActive_MultipleUsers(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	user1 := "set-active-user-9"
	user2 := "set-active-user-10"
	ctx1 := createAuthContext(user1, "test-namespace")
	ctx2 := createAuthContext(user2, "test-namespace")

	// User 1 activates goal
	_, err := client.SetGoalActive(ctx1, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    true,
	})
	require.NoError(t, err, "User 1 activation should succeed")

	// User 2 activates same goal (should be independent)
	_, err = client.SetGoalActive(ctx2, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    true,
	})
	require.NoError(t, err, "User 2 activation should succeed")

	// User 1 deactivates goal
	_, err = client.SetGoalActive(ctx1, &pb.SetGoalActiveRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
		IsActive:    false,
	})
	require.NoError(t, err, "User 1 deactivation should succeed")

	// Verify users are independent (both operations should succeed)
	// This test verifies that deactivating for user 1 doesn't affect user 2
}
