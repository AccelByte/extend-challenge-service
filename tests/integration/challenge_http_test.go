package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// TestGetUserChallenges_EmptyProgress_HTTP tests getting challenges with no user progress via HTTP
func TestGetUserChallenges_EmptyProgress_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// No seeded data - user has no progress

	// Make HTTP GET request
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "test-user-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Should return all challenges from test config
	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok)
	assert.Len(t, challenges, 2)

	// Verify challenges are present
	winterChallenge := findChallengeJSON(challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)
	assert.Equal(t, "Winter Challenge 2025", winterChallenge["name"])

	goals, ok := winterChallenge["goals"].([]interface{})
	require.True(t, ok)
	assert.Len(t, goals, 3)

	dailyQuests := findChallengeJSON(challenges, "daily-quests")
	assert.NotNil(t, dailyQuests)
	assert.Equal(t, "Daily Quests", dailyQuests["name"])

	// Verify MockRewardClient was not called
	mockRewardClient.AssertNotCalled(t, "GrantReward")
	mockRewardClient.AssertNotCalled(t, "GrantItemReward")
	mockRewardClient.AssertNotCalled(t, "GrantWalletReward")
}

// TestGetUserChallenges_WithProgress_HTTP tests getting challenges with user progress via HTTP
func TestGetUserChallenges_WithProgress_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Make HTTP GET request
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "test-user-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok)
	assert.Len(t, challenges, 2)

	// Find winter-challenge-2025
	winterChallenge := findChallengeJSON(challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)

	goals, ok := winterChallenge["goals"].([]interface{})
	require.True(t, ok)
	assert.Len(t, goals, 3)

	// Verify completed goal
	completedGoal := findGoalJSON(goals, "kill-10-snowmen")
	assert.NotNil(t, completedGoal)
	assert.Equal(t, "completed", completedGoal["status"])
	assert.Equal(t, float64(10), completedGoal["progress"])
	assert.NotEmpty(t, completedGoal["completedAt"])

	// Verify MockRewardClient was not called (just querying, not claiming)
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestClaimGoalReward_HappyPath_HTTP tests successful reward claiming via HTTP
func TestClaimGoalReward_HappyPath_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Seed prerequisite goal as claimed (kill-10-snowmen requires complete-tutorial)
	seedClaimedGoal(t, testDB, "test-user-123", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting to succeed (ITEM reward: winter_sword, quantity 1)
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"test-user-123",
		mock.MatchedBy(func(reward commonDomain.Reward) bool {
			return reward.Type == "ITEM" && reward.RewardID == "winter_sword" && reward.Quantity == 1
		}),
	).Return(nil)

	// Make HTTP POST request to claim reward
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "claimed", resp["status"])
	assert.NotEmpty(t, resp["claimedAt"])

	// Verify reward was granted
	mockRewardClient.AssertExpectations(t)

	// Verify database updated to claimed
	var status string
	err = testDB.QueryRow(
		"SELECT status FROM user_goal_progress WHERE user_id = $1 AND goal_id = $2",
		"test-user-123", "kill-10-snowmen",
	).Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "claimed", status)
}

// TestClaimGoalReward_Idempotency_HTTP tests claiming an already-claimed goal via HTTP
func TestClaimGoalReward_Idempotency_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Seed already-claimed goal
	seedClaimedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Try to claim again (should fail)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return error status
	assert.NotEqual(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Error message should indicate already claimed
	if message, ok := resp["message"].(string); ok {
		assert.Contains(t, message, "already been claimed")
	} else if errorMsg, ok := resp["error"].(string); ok {
		assert.Contains(t, errorMsg, "already been claimed")
	}

	// Verify reward was NOT granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
	mockRewardClient.AssertNotCalled(t, "GrantItemReward")
	mockRewardClient.AssertNotCalled(t, "GrantWalletReward")
}

// TestClaimGoalReward_MultipleUsers_HTTP tests isolation between users via HTTP
func TestClaimGoalReward_MultipleUsers_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Seed prerequisite goals for both users
	seedClaimedGoal(t, testDB, "user-1", "complete-tutorial", "winter-challenge-2025")
	seedClaimedGoal(t, testDB, "user-2", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goals for two different users
	seedCompletedGoal(t, testDB, "user-1", "kill-10-snowmen", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, "user-2", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting for both users
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"user-1",
		mock.Anything,
	).Return(nil).Once()

	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"user-2",
		mock.Anything,
	).Return(nil).Once()

	// User 1 claims
	req1 := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req1.Header.Set("x-mock-user-id", "user-1")
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	var resp1 map[string]interface{}
	err := json.NewDecoder(w1.Body).Decode(&resp1)
	require.NoError(t, err)
	assert.Equal(t, "claimed", resp1["status"])

	// User 2 can still claim (different user)
	req2 := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req2.Header.Set("x-mock-user-id", "user-2")
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]interface{}
	err = json.NewDecoder(w2.Body).Decode(&resp2)
	require.NoError(t, err)
	assert.Equal(t, "claimed", resp2["status"])

	// Verify both rewards were granted
	mockRewardClient.AssertExpectations(t)
}

// findChallengeJSON finds a challenge by ID in JSON response
// Note: grpc-gateway uses camelCase for JSON fields
func findChallengeJSON(challenges []interface{}, challengeID string) map[string]interface{} {
	for _, c := range challenges {
		chMap := c.(map[string]interface{})
		if chMap["challengeId"] == challengeID {
			return chMap
		}
	}
	return nil
}

// findGoalJSON finds a goal by ID in JSON goals array
// Note: grpc-gateway uses camelCase for JSON fields
func findGoalJSON(goals []interface{}, goalID string) map[string]interface{} {
	for _, g := range goals {
		goalMap := g.(map[string]interface{})
		if goalMap["goalId"] == goalID {
			return goalMap
		}
	}
	return nil
}

// TestGetUserChallenges_QueryParameterValidation_HTTP tests validation for invalid query parameters
func TestGetUserChallenges_QueryParameterValidation_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "test-user-validation"

	// Test invalid active_only parameter (should handle gracefully)
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=invalid-value", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should either:
	// 1. Return 400 Bad Request for invalid boolean
	// 2. Return 200 OK with default value (false)
	// Either behavior is acceptable - we just verify it doesn't crash
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest,
		"Should either succeed with default or fail with validation error, got %d", w.Code)

	if w.Code == http.StatusOK {
		var resp map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err, "Valid response should be parseable JSON")
		assert.Contains(t, resp, "challenges", "Response should have challenges field")
	}
}

// TestClaimGoalReward_ConcurrentClaims_HTTP tests race condition handling for concurrent claims
func TestClaimGoalReward_ConcurrentClaims_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "test-user-concurrent-claim"

	// Seed prerequisite goal as claimed
	seedClaimedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")

	// Seed completed goal
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting to succeed (only once!)
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		userID,
		mock.MatchedBy(func(reward commonDomain.Reward) bool {
			return reward.Type == "ITEM" && reward.RewardID == "winter_sword"
		}),
	).Return(nil).Once()

	// Make 2 concurrent HTTP POST requests to claim the same reward
	const numClaims = 2
	results := make(chan int, numClaims)
	errors := make(chan error, numClaims)

	for i := 0; i < numClaims; i++ {
		go func() {
			req := httptest.NewRequest(
				http.MethodPost,
				"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
				nil,
			)
			req.Header.Set("x-mock-user-id", userID)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code >= 200 && w.Code < 300 {
				results <- w.Code
			} else {
				results <- w.Code
			}
		}()
	}

	// Collect results
	var statusCodes []int
	for i := 0; i < numClaims; i++ {
		select {
		case code := <-results:
			statusCodes = append(statusCodes, code)
		case err := <-errors:
			t.Fatalf("Concurrent claim failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent claims")
		}
	}

	// Verify results:
	// - Exactly one claim should succeed (200 OK)
	// - The other should fail (already claimed)
	successCount := 0
	for _, code := range statusCodes {
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, 1, successCount,
		"Exactly one concurrent claim should succeed, got %d successes with codes: %v",
		successCount, statusCodes)

	// Verify reward was granted exactly once
	mockRewardClient.AssertExpectations(t)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

// TestClaimGoalReward_InactiveGoal_HTTP tests M3 Phase 6 validation (cannot claim inactive goal)
func TestClaimGoalReward_InactiveGoal_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "test-user-inactive-goal"

	// Seed prerequisite goal as claimed
	seedClaimedGoal(t, testDB, "test-user-123", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goal that is marked as inactive (is_active = false)
	_, err := testDB.Exec(`
		INSERT INTO user_goal_progress
		(user_id, goal_id, challenge_id, namespace, progress, status, is_active, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`, userID, "kill-10-snowmen", "winter-challenge-2025", "test-namespace",
		10, "completed", false, time.Now().UTC())
	require.NoError(t, err, "Failed to seed inactive completed goal")

	// Try to claim reward for inactive goal
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return error status (400 Bad Request or similar)
	assert.NotEqual(t, http.StatusOK, w.Code,
		"Should not allow claiming reward for inactive goal")

	var resp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	// Verify error message mentions inactive or active status
	if message, ok := resp["message"].(string); ok {
		assert.True(t,
			strings.Contains(strings.ToLower(message), "active") ||
				strings.Contains(strings.ToLower(message), "inactive"),
			"Error message should mention active/inactive status, got: %s", message)
	}

	// Verify reward was NOT granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}
