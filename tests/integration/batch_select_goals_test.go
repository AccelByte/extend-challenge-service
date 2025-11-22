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

// TestBatchSelectGoals_Success verifies successful batch goal selection
func TestBatchSelectGoals_Success(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-1"
	ctx := createAuthContext(userID, "test-namespace")

	// Select 2 goals from winter-challenge-2025
	resp, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"kill-10-snowmen",
			"reach-level-5",
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.NoError(t, err, "BatchSelectGoals should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Equal(t, "winter-challenge-2025", resp.ChallengeId)
	assert.Len(t, resp.SelectedGoals, 2, "Should return 2 selected goals")
	assert.Equal(t, 2, int(resp.TotalActiveGoals), "Should have 2 active goals")
	assert.Empty(t, resp.ReplacedGoals, "Should not replace any goals")

	// Verify the selected goals
	goal1 := findSelectedGoal(resp.SelectedGoals, "kill-10-snowmen")
	require.NotNil(t, goal1, "kill-10-snowmen should be selected")
	assert.Equal(t, "Kill 10 Snowmen", goal1.Name)
	assert.True(t, goal1.IsActive, "Goal should be active")
	assert.NotEmpty(t, goal1.AssignedAt, "AssignedAt should be set")

	goal2 := findSelectedGoal(resp.SelectedGoals, "reach-level-5")
	require.NotNil(t, goal2, "reach-level-5 should be selected")
	assert.True(t, goal2.IsActive, "Goal should be active")

	// Verify database state
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"), "Goal should be active in DB")
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "reach-level-5"), "Goal should be active in DB")
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestBatchSelectGoals_ReplaceExisting verifies replacing existing active goals
func TestBatchSelectGoals_ReplaceExisting(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-2"
	ctx := createAuthContext(userID, "test-namespace")

	// Seed existing active goals
	seedActiveGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Verify initial state
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Replace with new goals
	resp, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"reach-level-5",
		},
		ReplaceExisting: true,
	})

	// Assertions
	require.NoError(t, err, "BatchSelectGoals should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Len(t, resp.SelectedGoals, 1, "Should return 1 selected goal")
	assert.Equal(t, 1, int(resp.TotalActiveGoals), "Should have 1 active goal")
	assert.Len(t, resp.ReplacedGoals, 2, "Should replace 2 goals")
	assert.Contains(t, resp.ReplacedGoals, "complete-tutorial")
	assert.Contains(t, resp.ReplacedGoals, "kill-10-snowmen")

	// Verify database state
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "complete-tutorial"), "Old goal should be inactive")
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"), "Old goal should be inactive")
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "reach-level-5"), "New goal should be active")
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestBatchSelectGoals_Idempotent verifies selecting already active goals is idempotent
func TestBatchSelectGoals_Idempotent(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-3"
	ctx := createAuthContext(userID, "test-namespace")

	// Seed existing active goal
	seedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Select the same goal again (plus a new one)
	resp, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"kill-10-snowmen", // Already active
			"reach-level-5",   // New
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.NoError(t, err, "BatchSelectGoals should be idempotent")
	require.NotNil(t, resp, "Response should not be nil")

	assert.Len(t, resp.SelectedGoals, 2, "Should return both goals")
	assert.Equal(t, 2, int(resp.TotalActiveGoals), "Should have 2 active goals (not counting duplicate)")

	// Verify database state
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestBatchSelectGoals_EmptyGoalList verifies error handling for empty goal list
func TestBatchSelectGoals_EmptyGoalList(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-4"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select with empty goal list
	_, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId:     "winter-challenge-2025",
		GoalIds:         []string{},
		ReplaceExisting: false,
	})

	// Assertions
	require.Error(t, err, "BatchSelectGoals should fail for empty goal list")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "cannot be empty")
}

// TestBatchSelectGoals_InvalidGoalID verifies error handling for invalid goal ID
func TestBatchSelectGoals_InvalidGoalID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-5"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select non-existent goal
	_, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"kill-10-snowmen",
			"invalid-goal-id", // Does not exist
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.Error(t, err, "BatchSelectGoals should fail for invalid goal")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "not found")
}

// TestBatchSelectGoals_WrongChallenge verifies error handling for goal from wrong challenge
func TestBatchSelectGoals_WrongChallenge(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-6"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select goal from different challenge
	_, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"login-today", // This belongs to daily-quests
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.Error(t, err, "BatchSelectGoals should fail for wrong challenge")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "does not belong to challenge")
}

// TestBatchSelectGoals_MissingChallengeID verifies validation for missing challenge_id
func TestBatchSelectGoals_MissingChallengeID(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-7"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select without challenge_id
	_, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "", // Empty
		GoalIds: []string{
			"kill-10-snowmen",
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.Error(t, err, "BatchSelectGoals should fail for missing challenge_id")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "challenge_id is required")
}

// TestBatchSelectGoals_ChallengeNotFound verifies error handling for non-existent challenge
func TestBatchSelectGoals_ChallengeNotFound(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-8"
	ctx := createAuthContext(userID, "test-namespace")

	// Try to select from non-existent challenge
	_, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "nonexistent-challenge",
		GoalIds: []string{
			"some-goal",
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.Error(t, err, "BatchSelectGoals should fail for non-existent challenge")

	st, ok := status.FromError(err)
	require.True(t, ok, "Error should be a gRPC status error")
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "not found")
}

// TestBatchSelectGoals_MultipleUsers verifies user isolation
func TestBatchSelectGoals_MultipleUsers(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	user1 := "batch-user-9"
	user2 := "batch-user-10"
	ctx1 := createAuthContext(user1, "test-namespace")
	ctx2 := createAuthContext(user2, "test-namespace")

	// User 1 selects goals
	resp1, err := client.BatchSelectGoals(ctx1, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"kill-10-snowmen",
		},
		ReplaceExisting: false,
	})
	require.NoError(t, err, "User 1 selection should succeed")
	assert.Equal(t, 1, int(resp1.TotalActiveGoals))

	// User 2 selects different goals
	resp2, err := client.BatchSelectGoals(ctx2, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"reach-level-5",
		},
		ReplaceExisting: false,
	})
	require.NoError(t, err, "User 2 selection should succeed")
	assert.Equal(t, 1, int(resp2.TotalActiveGoals))

	// Verify users are isolated
	assert.True(t, getGoalActiveStatus(t, testDB, user1, "kill-10-snowmen"))
	assert.False(t, getGoalActiveStatus(t, testDB, user1, "reach-level-5"))
	assert.False(t, getGoalActiveStatus(t, testDB, user2, "kill-10-snowmen"))
	assert.True(t, getGoalActiveStatus(t, testDB, user2, "reach-level-5"))
}

// TestBatchSelectGoals_VerifyTimestamps verifies assigned_at timestamp format
func TestBatchSelectGoals_VerifyTimestamps(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "batch-user-11"
	ctx := createAuthContext(userID, "test-namespace")

	resp, err := client.BatchSelectGoals(ctx, &pb.BatchSelectRequest{
		ChallengeId: "winter-challenge-2025",
		GoalIds: []string{
			"kill-10-snowmen",
		},
		ReplaceExisting: false,
	})

	// Assertions
	require.NoError(t, err, "BatchSelectGoals should succeed")
	require.Len(t, resp.SelectedGoals, 1)

	goal := resp.SelectedGoals[0]
	assert.NotEmpty(t, goal.AssignedAt, "AssignedAt should be set")

	// Validate timestamp format (RFC3339)
	_, err = time.Parse(time.RFC3339, goal.AssignedAt)
	assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp")
}
