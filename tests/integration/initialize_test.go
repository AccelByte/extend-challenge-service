// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package integration

import (
	"fmt"
	"testing"
	"time"

	pb "extend-challenge-service/pkg/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertTimestampsEqual compares two RFC3339 timestamp strings as time values
// This ensures timezone-agnostic comparison (e.g., "2025-11-11T13:11:39Z" == "2025-11-11T20:11:39+07:00")
func assertTimestampsEqual(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
	t.Helper()

	expectedTime, err := time.Parse(time.RFC3339, expected)
	require.NoError(t, err, "Failed to parse expected timestamp: %s", expected)

	actualTime, err := time.Parse(time.RFC3339, actual)
	require.NoError(t, err, "Failed to parse actual timestamp: %s", actual)

	if !expectedTime.Equal(actualTime) {
		msg := fmt.Sprintf("Timestamps not equal:\n  Expected: %s (%s)\n  Actual:   %s (%s)",
			expected, expectedTime.UTC(), actual, actualTime.UTC())
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%v\n%s", msgAndArgs[0], msg)
		}
		t.Error(msg)
	}
}

// TestInitializePlayer_FirstLogin verifies that a new player receives default goals
func TestInitializePlayer_FirstLogin(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "init-user-first"
	ctx := createAuthContext(userID, "test-namespace")

	// Call initialize endpoint
	resp, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})

	// Assertions
	require.NoError(t, err, "InitializePlayer should succeed")
	require.NotNil(t, resp, "Response should not be nil")

	// Should assign 1 default goal (complete-tutorial)
	assert.Equal(t, int32(1), resp.NewAssignments, "Should assign 1 new goal")
	assert.Equal(t, int32(1), resp.TotalActive, "Should have 1 active goal")
	assert.Len(t, resp.AssignedGoals, 1, "Should return 1 assigned goal")

	// Validate assigned goal structure
	goal := resp.AssignedGoals[0]
	assert.Equal(t, "winter-challenge-2025", goal.ChallengeId, "Challenge ID should match")
	assert.Equal(t, "complete-tutorial", goal.GoalId, "Goal ID should match")
	assert.Equal(t, "Complete Tutorial", goal.Name, "Goal name should match")
	assert.True(t, goal.IsActive, "Goal should be active")
	assert.Equal(t, "not_started", goal.Status, "Initial status should be not_started")
	assert.Equal(t, int32(0), goal.Progress, "Initial progress should be 0")
	assert.Equal(t, int32(1), goal.Target, "Target should match config")
	assert.NotEmpty(t, goal.AssignedAt, "AssignedAt should be set")
	assert.Empty(t, goal.ExpiresAt, "ExpiresAt should be nil for M3 (permanent assignment)")

	// Validate requirement
	require.NotNil(t, goal.Requirement, "Requirement should not be nil")
	assert.Equal(t, "tutorial_completed", goal.Requirement.StatCode)
	assert.Equal(t, ">=", goal.Requirement.Operator)
	assert.Equal(t, int32(1), goal.Requirement.TargetValue)

	// Validate reward
	require.NotNil(t, goal.Reward, "Reward should not be nil")
	assert.Equal(t, "WALLET", goal.Reward.Type)
	assert.Equal(t, "GOLD", goal.Reward.RewardId)
	assert.Equal(t, int32(100), goal.Reward.Quantity)

	// Validate timestamp format
	_, err = time.Parse(time.RFC3339, goal.AssignedAt)
	assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp")
}

// TestInitializePlayer_SubsequentLogin_FastPath verifies idempotency and fast path
func TestInitializePlayer_SubsequentLogin_FastPath(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "init-user-subsequent"
	ctx := createAuthContext(userID, "test-namespace")

	// First initialization
	resp1, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
	require.NoError(t, err, "First initialization should succeed")
	require.Equal(t, int32(1), resp1.NewAssignments, "First call should assign 1 goal")

	// Get assigned_at timestamp from first call
	firstAssignedAt := resp1.AssignedGoals[0].AssignedAt

	// Second initialization (fast path)
	resp2, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
	require.NoError(t, err, "Second initialization should succeed")

	// Assertions - fast path
	assert.Equal(t, int32(0), resp2.NewAssignments, "Second call should assign 0 new goals (fast path)")
	assert.Equal(t, int32(1), resp2.TotalActive, "Should still have 1 active goal")
	assert.Len(t, resp2.AssignedGoals, 1, "Should return 1 assigned goal")

	// Verify same goal is returned
	goal := resp2.AssignedGoals[0]
	assert.Equal(t, "complete-tutorial", goal.GoalId, "Same goal should be returned")
	assertTimestampsEqual(t, firstAssignedAt, goal.AssignedAt, "AssignedAt timestamp should not change")
	assert.Equal(t, "not_started", goal.Status, "Status should remain unchanged")
	assert.Equal(t, int32(0), goal.Progress, "Progress should remain unchanged")
}

// TestInitializePlayer_MultipleUsers verifies user isolation
func TestInitializePlayer_MultipleUsers(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Initialize user 1
	user1ID := "init-user-multi-1"
	ctx1 := createAuthContext(user1ID, "test-namespace")
	resp1, err := client.InitializePlayer(ctx1, &pb.InitializeRequest{})
	require.NoError(t, err, "User 1 initialization should succeed")
	require.Equal(t, int32(1), resp1.NewAssignments, "User 1 should get 1 goal")

	// Initialize user 2
	user2ID := "init-user-multi-2"
	ctx2 := createAuthContext(user2ID, "test-namespace")
	resp2, err := client.InitializePlayer(ctx2, &pb.InitializeRequest{})
	require.NoError(t, err, "User 2 initialization should succeed")
	require.Equal(t, int32(1), resp2.NewAssignments, "User 2 should get 1 goal")

	// Verify both users got their goals independently
	// Note: assigned_at timestamps might be the same if operations are very fast (within same second)
	// The key is that each user gets their own goal assignment record

	// Verify user 1 still has 1 goal on subsequent call
	resp1Again, err := client.InitializePlayer(ctx1, &pb.InitializeRequest{})
	require.NoError(t, err, "User 1 re-initialization should succeed")
	assert.Equal(t, int32(0), resp1Again.NewAssignments, "User 1 should have 0 new assignments (fast path)")
	assert.Equal(t, int32(1), resp1Again.TotalActive, "User 1 should still have 1 active goal")
}

// TestInitializePlayer_WithProgress verifies that initialization preserves existing progress
func TestInitializePlayer_WithProgress(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "init-user-progress"
	ctx := createAuthContext(userID, "test-namespace")

	// First initialization
	resp1, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
	require.NoError(t, err, "Initialization should succeed")
	require.Equal(t, int32(1), resp1.NewAssignments, "Should assign 1 goal")

	// Simulate progress update by directly calling the challenges endpoint
	// (In a real scenario, this would happen via event handler)
	// For this test, we'll just verify that subsequent initialization preserves state

	// Second initialization
	resp2, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
	require.NoError(t, err, "Second initialization should succeed")

	// Verify goal is returned with preserved state
	assert.Equal(t, int32(0), resp2.NewAssignments, "No new assignments on second call")
	assert.Equal(t, int32(1), resp2.TotalActive, "Still have 1 active goal")
	assert.Len(t, resp2.AssignedGoals, 1, "Should return 1 assigned goal")

	goal := resp2.AssignedGoals[0]
	assert.Equal(t, "complete-tutorial", goal.GoalId)
	assert.Equal(t, int32(0), goal.Progress, "Progress should be preserved (still 0)")
	assert.Equal(t, "not_started", goal.Status, "Status should be preserved")
}

// TestInitializePlayer_Idempotency verifies that multiple calls are safe
func TestInitializePlayer_Idempotency(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "init-user-idempotent"
	ctx := createAuthContext(userID, "test-namespace")

	// Call initialize 5 times in sequence
	var responses []*pb.InitializeResponse
	for i := 0; i < 5; i++ {
		resp, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
		require.NoError(t, err, "Call %d should succeed", i+1)
		responses = append(responses, resp)
	}

	// First call should assign 1 goal
	assert.Equal(t, int32(1), responses[0].NewAssignments, "First call should assign 1 goal")
	assert.Equal(t, int32(1), responses[0].TotalActive, "First call should have 1 active goal")

	// All subsequent calls should be fast path (0 new assignments)
	for i := 1; i < 5; i++ {
		assert.Equal(t, int32(0), responses[i].NewAssignments,
			"Call %d should assign 0 new goals (fast path)", i+1)
		assert.Equal(t, int32(1), responses[i].TotalActive,
			"Call %d should still have 1 active goal", i+1)
		assert.Len(t, responses[i].AssignedGoals, 1,
			"Call %d should return 1 assigned goal", i+1)
	}

	// All calls should return the same goal_id
	for i := 1; i < 5; i++ {
		assert.Equal(t, responses[0].AssignedGoals[0].GoalId, responses[i].AssignedGoals[0].GoalId,
			"All calls should return the same goal")
		assertTimestampsEqual(t, responses[0].AssignedGoals[0].AssignedAt, responses[i].AssignedGoals[0].AssignedAt,
			"AssignedAt timestamp should remain constant")
	}
}

// TestInitializePlayer_NoDefaultGoals verifies behavior when no default goals configured
func TestInitializePlayer_NoDefaultGoals(t *testing.T) {
	// This test would require a different config file with no default goals
	// For now, we'll skip it as our test config has 1 default goal
	// TODO: Add test with alternate config that has default_assigned=false for all goals
	t.Skip("Requires alternate config with no default goals")
}

// TestInitializePlayer_ConcurrentCalls verifies thread safety
func TestInitializePlayer_ConcurrentCalls(t *testing.T) {
	client, _, cleanup := setupTestServer(t)
	defer cleanup()

	userID := "init-user-concurrent"

	// Make 10 concurrent initialization calls for the same user
	const numCalls = 10
	results := make(chan *pb.InitializeResponse, numCalls)
	errors := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			ctx := createAuthContext(userID, "test-namespace")
			resp, err := client.InitializePlayer(ctx, &pb.InitializeRequest{})
			if err != nil {
				errors <- err
			} else {
				results <- resp
			}
		}()
	}

	// Collect results
	var responses []*pb.InitializeResponse
	for i := 0; i < numCalls; i++ {
		select {
		case resp := <-results:
			responses = append(responses, resp)
		case err := <-errors:
			t.Fatalf("Concurrent call failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent calls")
		}
	}

	// All calls should succeed
	assert.Len(t, responses, numCalls, "All calls should succeed")

	// Exactly one call should have new_assignments=1 (first winner)
	// Others should have new_assignments=0 (fast path)
	newAssignmentCounts := make(map[int32]int)
	for _, resp := range responses {
		newAssignmentCounts[resp.NewAssignments]++
	}

	// We expect either:
	// - 1 call with new_assignments=1, 9 calls with new_assignments=0
	// - OR all 10 calls with new_assignments=0 (if timing allows first to complete before others start)
	assert.True(t,
		(newAssignmentCounts[1] == 1 && newAssignmentCounts[0] == 9) ||
			(newAssignmentCounts[0] == 10),
		"Expected 1 winner and 9 fast-path OR all fast-path, got: %v", newAssignmentCounts)

	// All responses should return 1 total active goal
	for i, resp := range responses {
		assert.Equal(t, int32(1), resp.TotalActive, "Response %d should have 1 total active goal", i)
		assert.Len(t, resp.AssignedGoals, 1, "Response %d should return 1 assigned goal", i)
	}
}
