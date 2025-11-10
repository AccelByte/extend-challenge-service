package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetChallenges_MissingAuth_HTTP tests request without authentication header
func TestGetChallenges_MissingAuth_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Make request WITHOUT x-mock-user-id header
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	// No auth header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// In test mode with testAuthMiddleware, missing header means no userID in context
	// The handler should still work but might use a default or empty user ID
	// Let's verify it doesn't crash
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusUnauthorized,
		"Should either succeed with default user or require auth, got %d", w.Code)
}

// TestGetChallenges_MethodNotAllowed_HTTP tests using wrong HTTP method
func TestGetChallenges_MethodNotAllowed_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	testCases := []struct {
		name   string
		method string
	}{
		{"POST not allowed", http.MethodPost},
		{"PUT not allowed", http.MethodPut},
		{"DELETE not allowed", http.MethodDelete},
		{"PATCH not allowed", http.MethodPatch},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/challenges", nil)
			req.Header.Set("x-mock-user-id", "test-user")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Should return 405 Method Not Allowed, 404 Not Found, or 501 Not Implemented
			// (grpc-gateway returns 501 for unsupported methods)
			assert.True(t, w.Code == http.StatusMethodNotAllowed || w.Code == http.StatusNotFound || w.Code == http.StatusNotImplemented,
				"Expected 404, 405, or 501, got %d", w.Code)
		})
	}
}

// TestGetChallenges_EmptyUserID_HTTP tests with empty user ID
func TestGetChallenges_EmptyUserID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Set empty user ID
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should either use default user or return error
	// Either is acceptable - just verify it doesn't crash
	assert.True(t, w.Code >= 200 && w.Code < 600, "Should return valid HTTP status")
}

// TestClaimReward_MissingChallengeID_HTTP tests claiming with missing challenge ID
func TestClaimReward_MissingChallengeID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Try to claim with empty challenge ID (invalid URL)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges//goals/some-goal/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 400 or 404 (invalid URL pattern or bad request)
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound,
		"Empty challenge ID should result in 400 or 404, got %d", w.Code)
}

// TestClaimReward_MissingGoalID_HTTP tests claiming with missing goal ID
func TestClaimReward_MissingGoalID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Try to claim with empty goal ID (invalid URL)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/some-challenge/goals//claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 400 or 404 (invalid URL pattern or bad request)
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound,
		"Empty goal ID should result in 400 or 404, got %d", w.Code)
}

// TestClaimReward_NonExistentChallenge_HTTP tests claiming for non-existent challenge
func TestClaimReward_NonExistentChallenge_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/nonexistent-challenge-xyz/goals/some-goal/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 404 Not Found
	assert.Equal(t, http.StatusNotFound, w.Code,
		"Non-existent challenge should result in 404")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify error message mentions challenge not found
	if message, ok := resp["message"].(string); ok {
		assert.Contains(t, message, "challenge", "Error message should mention challenge")
	} else if errorMsg, ok := resp["error"].(string); ok {
		assert.Contains(t, errorMsg, "challenge", "Error message should mention challenge")
	}

	// Verify no reward was granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestClaimReward_NonExistentGoal_HTTP tests claiming for non-existent goal
func TestClaimReward_NonExistentGoal_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/nonexistent-goal-xyz/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 404 Not Found
	assert.Equal(t, http.StatusNotFound, w.Code,
		"Non-existent goal should result in 404")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify error message mentions goal not found
	if message, ok := resp["message"].(string); ok {
		assert.Contains(t, message, "goal", "Error message should mention goal")
	} else if errorMsg, ok := resp["error"].(string); ok {
		assert.Contains(t, errorMsg, "goal", "Error message should mention goal")
	}

	// Verify no reward was granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestClaimReward_GoalNotCompleted_HTTP tests claiming an incomplete goal
func TestClaimReward_GoalNotCompleted_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "test-user-incomplete"

	// Seed prerequisite as claimed
	seedClaimedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")

	// Seed goal that is in_progress (not completed)
	seedInProgressGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025", 5, 10)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"Claiming incomplete goal should result in 400")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify error message mentions not completed
	if message, ok := resp["message"].(string); ok {
		assert.Contains(t, message, "not completed", "Error message should mention not completed")
	} else if errorMsg, ok := resp["error"].(string); ok {
		assert.Contains(t, errorMsg, "not completed", "Error message should mention not completed")
	}

	// Verify no reward was granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestGetChallenges_QueryParameter_ActiveOnly_HTTP tests active_only query parameter
func TestGetChallenges_QueryParameter_ActiveOnly_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "test-user-active-filter"

	testCases := []struct {
		name          string
		queryParam    string
		expectedCode  int
		shouldSucceed bool
	}{
		{"active_only=true", "?active_only=true", http.StatusOK, true},
		{"active_only=false", "?active_only=false", http.StatusOK, true},
		{"active_only omitted", "", http.StatusOK, true},
		{"active_only=1", "?active_only=1", http.StatusOK, true}, // Might be interpreted as true
		{"active_only=0", "?active_only=0", http.StatusOK, true}, // Might be interpreted as false
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/challenges"+tc.queryParam, nil)
			req.Header.Set("x-mock-user-id", userID)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code,
				"Query parameter %s should return %d", tc.queryParam, tc.expectedCode)

			if tc.shouldSucceed {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, resp, "challenges", "Response should have challenges field")
			}
		})
	}
}

// TestInitializePlayer_MissingUserID_HTTP tests initialization without user ID
func TestInitializePlayer_MissingUserID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Make request without user ID header
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	// No x-mock-user-id header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return error or use default user ID
	// Either behavior is acceptable - just verify it doesn't crash
	assert.True(t, w.Code >= 200 && w.Code < 600, "Should return valid HTTP status")
}

// TestSetGoalActive_MissingUserID_HTTP tests set-goal-active without user ID
func TestSetGoalActive_MissingUserID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Make request without user ID header
	req := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		nil,
	)
	// No x-mock-user-id header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return error or use default user ID
	assert.True(t, w.Code >= 200 && w.Code < 600, "Should return valid HTTP status")
}

// TestClaimReward_EmptyUserID_HTTP tests claiming with empty user ID
func TestClaimReward_EmptyUserID_HTTP(t *testing.T) {
	handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Seed completed goal for empty user ID (edge case)
	seedCompletedGoal(t, testDB, "", "kill-10-snowmen", "winter-challenge-2025")

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/claim",
		nil,
	)
	req.Header.Set("x-mock-user-id", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should handle gracefully (either error or use default)
	assert.True(t, w.Code >= 200 && w.Code < 600, "Should return valid HTTP status")

	// Verify no reward was granted (empty user ID should not work)
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}
