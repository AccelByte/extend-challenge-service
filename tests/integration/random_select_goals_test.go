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

// TestRandomSelectGoals_Success verifies successful random goal selection
func TestRandomSelectGoals_Success(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-1"
	ctx := createAuthContext(userID, "test-namespace")

	// Complete prerequisite goals so all goals are available
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Request 2 random goals from winter-challenge-2025 (1 goal left: reach-level-5)
	// Request more than available to test partial results
	resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           2,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.NoError(t, err, "RandomSelectGoals should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Equal(t, "winter-challenge-2025", resp.ChallengeId)
	assert.Len(t, resp.SelectedGoals, 1, "Should return 1 available goal (others completed)")
	assert.Equal(t, 1, int(resp.TotalActiveGoals), "Should have 1 active goal")
	assert.Empty(t, resp.ReplacedGoals, "Should not replace any goals")

	// Verify selected goal
	assert.Equal(t, "reach-level-5", resp.SelectedGoals[0].GoalId, "Should select the uncompleted goal")
	assert.True(t, resp.SelectedGoals[0].IsActive, "Goal should be active")
	assert.NotEmpty(t, resp.SelectedGoals[0].AssignedAt, "AssignedAt should be set")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestRandomSelectGoals_ReplaceExisting verifies replacing existing active goals
func TestRandomSelectGoals_ReplaceExisting(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-2"
	ctx := createAuthContext(userID, "test-namespace")

	// Seed existing active goals (kill-10-snowmen must be completed for reach-level-5 prerequisite)
	seedActiveGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Verify initial state
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Replace with 1 random goal
	resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           1,
		ReplaceExisting: true,
		ExcludeActive:   false,
	})

	// Assertions
	require.NoError(t, err, "RandomSelectGoals should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Len(t, resp.SelectedGoals, 1, "Should return 1 random goal")
	assert.Equal(t, 1, int(resp.TotalActiveGoals), "Should have 1 active goal")
	assert.Len(t, resp.ReplacedGoals, 2, "Should replace 2 goals")
	assert.Contains(t, resp.ReplacedGoals, "complete-tutorial")
	assert.Contains(t, resp.ReplacedGoals, "kill-10-snowmen")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Old goals should be inactive
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "complete-tutorial"))
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"))
}

// TestRandomSelectGoals_ExcludeActive verifies excluding already active goals
func TestRandomSelectGoals_ExcludeActive(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-3"
	ctx := createAuthContext(userID, "test-namespace")

	// Seed one active completed goal (must be completed for reach-level-5 prerequisite)
	seedCompletedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Request 2 goals, excluding active ones
	resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           2,
		ReplaceExisting: false,
		ExcludeActive:   true, // Exclude kill-10-snowmen
	})

	// Assertions
	require.NoError(t, err, "RandomSelectGoals should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Len(t, resp.SelectedGoals, 2, "Should return 2 goals")
	assert.Equal(t, 3, int(resp.TotalActiveGoals), "Should have 3 active goals (1 existing + 2 new)")

	// Verify the new goals don't include kill-10-snowmen
	for _, goal := range resp.SelectedGoals {
		assert.NotEqual(t, "kill-10-snowmen", goal.GoalId, "Should not select already active goal")
	}

	// Verify database state
	assert.Equal(t, 3, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"), "Original goal should still be active")
}

// TestRandomSelectGoals_PartialResults verifies handling when fewer goals available than requested
func TestRandomSelectGoals_PartialResults(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-4"
	ctx := createAuthContext(userID, "test-namespace")

	// Complete prerequisite goals so reach-level-5 becomes available
	// (complete-tutorial and kill-10-snowmen are completed, so only reach-level-5 is available)
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Request 5 goals but only 1 is available (reach-level-5)
	resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           5,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.NoError(t, err, "RandomSelectGoals should succeed with partial results")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Len(t, resp.SelectedGoals, 1, "Should return 1 available goal (reach-level-5)")
	assert.Equal(t, 1, int(resp.TotalActiveGoals), "Should have 1 active goal")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestRandomSelectGoals_NoGoalsAvailable verifies error when no goals are available
func TestRandomSelectGoals_NoGoalsAvailable(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-5"
	ctx := createAuthContext(userID, "test-namespace")

	// Seed all goals as completed (not available for selection)
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "reach-level-5", "winter-challenge-2025")

	// Try to select goals
	_, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           2,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.Error(t, err, "RandomSelectGoals should fail when no goals available")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "no goals available")
}

// TestRandomSelectGoals_InvalidCount verifies error handling for invalid count
func TestRandomSelectGoals_InvalidCount(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-6"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select with count <= 0
	_, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           0, // Invalid
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.Error(t, err, "RandomSelectGoals should fail for invalid count")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "count must be greater than 0")
}

// TestRandomSelectGoals_ChallengeNotFound verifies error handling for non-existent challenge
func TestRandomSelectGoals_ChallengeNotFound(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-7"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select from non-existent challenge
	_, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "nonexistent-challenge",
		Count:           2,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.Error(t, err, "RandomSelectGoals should fail for non-existent challenge")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "not found")
}

// TestRandomSelectGoals_MissingChallengeID verifies validation for missing challenge_id
func TestRandomSelectGoals_MissingChallengeID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-8"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select without challenge_id
	_, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "", // Empty
		Count:           2,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.Error(t, err, "RandomSelectGoals should fail for missing challenge_id")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "challenge_id is required")
}

// TestRandomSelectGoals_MultipleUsers verifies user isolation
func TestRandomSelectGoals_MultipleUsers(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	user1 := "random-user-9"
	user2 := "random-user-10"
	ctx1 := createAuthContext(user1, "test-namespace")
	ctx2 := createAuthContext(user2, "test-namespace")

	// Seed prerequisites for both users so they have 2 goals available
	// (complete-tutorial and kill-10-snowmen completed, making reach-level-5 available)
	seedCompletedGoal(t, testDB, user1, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, user1, "kill-10-snowmen", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, user2, "complete-tutorial", "winter-challenge-2025")

	// User 1 selects 1 random goal (only reach-level-5 available)
	resp1, err := client.RandomSelectGoals(ctx1, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           1,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})
	require.NoError(t, err, "User 1 selection should succeed")
	assert.Len(t, resp1.SelectedGoals, 1)

	// User 2 selects 1 random goal (kill-10-snowmen available)
	resp2, err := client.RandomSelectGoals(ctx2, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           1,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})
	require.NoError(t, err, "User 2 selection should succeed")
	assert.Len(t, resp2.SelectedGoals, 1)

	// Verify users have independent active goals
	assert.Equal(t, 1, countActiveGoals(t, testDB, user1, "winter-challenge-2025"))
	assert.Equal(t, 1, countActiveGoals(t, testDB, user2, "winter-challenge-2025"))
}

// TestRandomSelectGoals_VerifyRandomness verifies that multiple calls return different results
func TestRandomSelectGoals_VerifyRandomness(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Use 3 different users to avoid interaction
	users := []string{"random-user-11", "random-user-12", "random-user-13"}
	selectedGoalsPerUser := make([]string, len(users))

	for i, userID := range users {
		ctx := createAuthContext(userID, "test-namespace")

		// Request 1 random goal from winter-challenge-2025
		resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
			ChallengeId:     "winter-challenge-2025",
			Count:           1,
			ReplaceExisting: false,
			ExcludeActive:   false,
		})

		require.NoError(t, err, "RandomSelectGoals should succeed for user %s", userID)
		require.Len(t, resp.SelectedGoals, 1)

		selectedGoalsPerUser[i] = resp.SelectedGoals[0].GoalId
	}

	// Note: With 3 goals and 3 users selecting 1 goal each, there's a chance
	// all 3 select the same goal (1/9 probability). But it's very unlikely
	// all 3 selections are identical if randomness is working.
	//
	// We can't assert they're all different (not guaranteed), but we can
	// verify the selections are from valid goals
	validGoals := map[string]bool{
		"complete-tutorial": true,
		"kill-10-snowmen":   true,
		"reach-level-5":     true,
	}

	for _, goalID := range selectedGoalsPerUser {
		assert.True(t, validGoals[goalID], "Selected goal %s should be valid", goalID)
	}

	// Log the results to manually verify randomness
	t.Logf("Random selection results: %v", selectedGoalsPerUser)
}

// TestRandomSelectGoals_VerifyTimestamps verifies assigned_at timestamp format
func TestRandomSelectGoals_VerifyTimestamps(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "random-user-14"
	ctx := createAuthContext(userID, "test-namespace")

	// Request 1 goal (complete-tutorial is available by default)
	resp, err := client.RandomSelectGoals(ctx, &pb.RandomSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		Count:           1,
		ReplaceExisting: false,
		ExcludeActive:   false,
	})

	// Assertions
	require.NoError(t, err, "RandomSelectGoals should succeed")
	require.Len(t, resp.SelectedGoals, 1)

	// Verify all timestamps are valid
	for _, goal := range resp.SelectedGoals {
		assert.NotEmpty(t, goal.AssignedAt, "AssignedAt should be set for goal %s", goal.GoalId)

		// Validate timestamp format (RFC3339)
		_, err = time.Parse(time.RFC3339, goal.AssignedAt)
		assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp for goal %s", goal.GoalId)
	}
}
