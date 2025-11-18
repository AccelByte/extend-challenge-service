package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertTimestampsEqual compares two RFC3339 timestamp strings as time values
// This ensures timezone-agnostic comparison (e.g., "2025-11-11T13:11:39Z" == "2025-11-11T20:11:39+07:00")
func assertTimestampsEqualHTTP(t *testing.T, expected, actual string, msgAndArgs ...interface{}) {
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

// TestInitializePlayer_FirstLogin_HTTP verifies that a new player receives default goals via HTTP
func TestInitializePlayer_FirstLogin_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "init-user-first-http"

	// Make HTTP POST request to /v1/challenges/initialize
	req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "InitializePlayer should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Should assign 1 default goal (complete-tutorial)
	assert.Equal(t, float64(1), resp["newAssignments"], "Should assign 1 new goal")
	assert.Equal(t, float64(1), resp["totalActive"], "Should have 1 active goal")

	// Validate assigned goals array
	assignedGoals, ok := resp["assignedGoals"].([]interface{})
	require.True(t, ok, "Response should have assignedGoals array")
	assert.Len(t, assignedGoals, 1, "Should return 1 assigned goal")

	// Validate assigned goal structure (camelCase field names!)
	goal := assignedGoals[0].(map[string]interface{})
	assert.Equal(t, "winter-challenge-2025", goal["challengeId"], "Challenge ID should match")
	assert.Equal(t, "complete-tutorial", goal["goalId"], "Goal ID should match")
	assert.Equal(t, "Complete Tutorial", goal["name"], "Goal name should match")
	assert.Equal(t, true, goal["isActive"], "Goal should be active")
	assert.Equal(t, "not_started", goal["status"], "Initial status should be not_started")
	assert.Equal(t, float64(0), goal["progress"], "Initial progress should be 0")
	assert.Equal(t, float64(1), goal["target"], "Target should match config")
	assert.NotEmpty(t, goal["assignedAt"], "AssignedAt should be set")

	// Validate requirement (nested object)
	requirement, ok := goal["requirement"].(map[string]interface{})
	require.True(t, ok, "Requirement should be present")
	assert.Equal(t, "tutorial_completed", requirement["statCode"])
	assert.Equal(t, ">=", requirement["operator"])
	assert.Equal(t, float64(1), requirement["targetValue"])

	// Validate reward (nested object)
	reward, ok := goal["reward"].(map[string]interface{})
	require.True(t, ok, "Reward should be present")
	assert.Equal(t, "WALLET", reward["type"])
	assert.Equal(t, "GOLD", reward["rewardId"])
	assert.Equal(t, float64(100), reward["quantity"])

	// Validate timestamp format
	assignedAt, ok := goal["assignedAt"].(string)
	require.True(t, ok, "AssignedAt should be string")
	_, err = time.Parse(time.RFC3339, assignedAt)
	assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp")
}

// TestInitializePlayer_SubsequentLogin_FastPath_HTTP verifies idempotency and fast path via HTTP
func TestInitializePlayer_SubsequentLogin_FastPath_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "init-user-subsequent-http"

	// First initialization
	req1 := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req1.Header.Set("x-mock-user-id", userID)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)
	var resp1 map[string]interface{}
	_ = json.NewDecoder(w1.Body).Decode(&resp1)
	assert.Equal(t, float64(1), resp1["newAssignments"], "First call should assign 1 goal")

	// Get assigned_at timestamp from first call
	assignedGoals1 := resp1["assignedGoals"].([]interface{})
	goal1 := assignedGoals1[0].(map[string]interface{})
	firstAssignedAt := goal1["assignedAt"].(string)

	// Second initialization (fast path)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req2.Header.Set("x-mock-user-id", userID)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	_ = json.NewDecoder(w2.Body).Decode(&resp2)

	// Assertions - fast path
	assert.Equal(t, float64(0), resp2["newAssignments"], "Second call should assign 0 new goals (fast path)")
	assert.Equal(t, float64(1), resp2["totalActive"], "Should still have 1 active goal")

	assignedGoals2 := resp2["assignedGoals"].([]interface{})
	assert.Len(t, assignedGoals2, 1, "Should return 1 assigned goal")

	// Verify same goal is returned
	goal2 := assignedGoals2[0].(map[string]interface{})
	assert.Equal(t, "complete-tutorial", goal2["goalId"], "Same goal should be returned")
	assertTimestampsEqualHTTP(t, firstAssignedAt, goal2["assignedAt"].(string), "AssignedAt timestamp should not change")
	assert.Equal(t, "not_started", goal2["status"], "Status should remain unchanged")
	assert.Equal(t, float64(0), goal2["progress"], "Progress should remain unchanged")
}

// TestInitializePlayer_Idempotency_HTTP verifies that multiple HTTP calls are safe
func TestInitializePlayer_Idempotency_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "init-user-idempotent-http"

	// Call initialize 5 times in sequence
	var responses []map[string]interface{}
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
		req.Header.Set("x-mock-user-id", userID)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Call %d should return 200 OK", i+1)

		var resp map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err, "Call %d response should be valid JSON", i+1)
		responses = append(responses, resp)
	}

	// First call should assign 1 goal
	assert.Equal(t, float64(1), responses[0]["newAssignments"], "First call should assign 1 goal")
	assert.Equal(t, float64(1), responses[0]["totalActive"], "First call should have 1 active goal")

	// All subsequent calls should be fast path (0 new assignments)
	for i := 1; i < 5; i++ {
		assert.Equal(t, float64(0), responses[i]["newAssignments"],
			"Call %d should assign 0 new goals (fast path)", i+1)
		assert.Equal(t, float64(1), responses[i]["totalActive"],
			"Call %d should still have 1 active goal", i+1)

		assignedGoals := responses[i]["assignedGoals"].([]interface{})
		assert.Len(t, assignedGoals, 1, "Call %d should return 1 assigned goal", i+1)
	}

	// All calls should return the same goal_id
	firstGoal := responses[0]["assignedGoals"].([]interface{})[0].(map[string]interface{})
	for i := 1; i < 5; i++ {
		goal := responses[i]["assignedGoals"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, firstGoal["goalId"], goal["goalId"],
			"All calls should return the same goal")
		assertTimestampsEqualHTTP(t, firstGoal["assignedAt"].(string), goal["assignedAt"].(string),
			"AssignedAt timestamp should remain constant")
	}
}

// TestInitializePlayer_MultipleUsers_HTTP verifies user isolation via HTTP
func TestInitializePlayer_MultipleUsers_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	// Initialize user 1
	user1ID := "init-user-multi-1-http"
	req1 := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req1.Header.Set("x-mock-user-id", user1ID)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	var resp1 map[string]interface{}
	_ = json.NewDecoder(w1.Body).Decode(&resp1)
	assert.Equal(t, float64(1), resp1["newAssignments"], "User 1 should get 1 goal")

	// Initialize user 2
	user2ID := "init-user-multi-2-http"
	req2 := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req2.Header.Set("x-mock-user-id", user2ID)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]interface{}
	_ = json.NewDecoder(w2.Body).Decode(&resp2)
	assert.Equal(t, float64(1), resp2["newAssignments"], "User 2 should get 1 goal")

	// Verify user 1 still has 1 goal on subsequent call (fast path)
	req1Again := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	req1Again.Header.Set("x-mock-user-id", user1ID)
	w1Again := httptest.NewRecorder()

	handler.ServeHTTP(w1Again, req1Again)
	assert.Equal(t, http.StatusOK, w1Again.Code)

	var resp1Again map[string]interface{}
	_ = json.NewDecoder(w1Again.Body).Decode(&resp1Again)
	assert.Equal(t, float64(0), resp1Again["newAssignments"], "User 1 should have 0 new assignments (fast path)")
	assert.Equal(t, float64(1), resp1Again["totalActive"], "User 1 should still have 1 active goal")
}
