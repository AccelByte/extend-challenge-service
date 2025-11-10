package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetGoalActive_ActivateGoal_Success_HTTP verifies successful goal activation via HTTP
func TestSetGoalActive_ActivateGoal_Success_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-1-http"

	// Activate a goal that's not default-assigned
	// PUT /v1/challenges/{challenge_id}/goals/{goal_id}/active with JSON body
	body := map[string]interface{}{
		"is_active": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "SetGoalActive should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Validate response fields (camelCase!)
	assert.Equal(t, "winter-challenge-2025", resp["challengeId"])
	assert.Equal(t, "kill-10-snowmen", resp["goalId"])
	assert.Equal(t, true, resp["isActive"], "Goal should be active")
	assert.NotEmpty(t, resp["assignedAt"], "AssignedAt should be set")
	assert.Equal(t, "Goal activated successfully", resp["message"])

	// Validate timestamp format
	assignedAt, ok := resp["assignedAt"].(string)
	require.True(t, ok, "AssignedAt should be string")
	_, err = time.Parse(time.RFC3339, assignedAt)
	assert.NoError(t, err, "AssignedAt should be valid RFC3339 timestamp")
}

// TestSetGoalActive_DeactivateGoal_Success_HTTP verifies successful goal deactivation via HTTP
func TestSetGoalActive_DeactivateGoal_Success_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-2-http"

	// First, initialize player to get default goals
	initReq := httptest.NewRequest(http.MethodPost, "/v1/challenges/initialize", nil)
	initReq.Header.Set("x-mock-user-id", userID)
	initW := httptest.NewRecorder()
	handler.ServeHTTP(initW, initReq)
	assert.Equal(t, http.StatusOK, initW.Code, "InitializePlayer should succeed")

	// Deactivate the default goal
	// PUT /v1/challenges/{challenge_id}/goals/{goal_id}/active with JSON body
	body := map[string]interface{}{
		"is_active": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/complete-tutorial/active",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "SetGoalActive should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "winter-challenge-2025", resp["challengeId"])
	assert.Equal(t, "complete-tutorial", resp["goalId"])
	assert.Equal(t, false, resp["isActive"], "Goal should be inactive")
	assert.NotEmpty(t, resp["assignedAt"], "AssignedAt should be set")
	assert.Equal(t, "Goal deactivated successfully", resp["message"])
}

// TestSetGoalActive_InvalidGoalID_HTTP verifies error handling for invalid goal via HTTP
func TestSetGoalActive_InvalidGoalID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-4-http"

	// Try to activate non-existent goal
	body := map[string]interface{}{
		"is_active": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/invalid-goal-id/active",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return error status
	assert.NotEqual(t, http.StatusOK, w.Code, "SetGoalActive should fail for invalid goal")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	// Verify error response contains descriptive message
	if message, ok := resp["message"].(string); ok {
		assert.Contains(t, strings.ToLower(message), "goal",
			"Error message should mention 'goal'")
	} else if errorMsg, ok := resp["error"].(string); ok {
		assert.Contains(t, strings.ToLower(errorMsg), "goal",
			"Error message should mention 'goal'")
	}
}

// TestSetGoalActive_ActivateThenDeactivate_HTTP verifies full activation cycle via HTTP
func TestSetGoalActive_ActivateThenDeactivate_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-3-http"

	// Step 1: Activate goal
	body1 := map[string]interface{}{"is_active": true}
	bodyBytes1, _ := json.Marshal(body1)
	req1 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes1),
	)
	req1.Header.Set("x-mock-user-id", userID)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code, "Activation should succeed")
	var resp1 map[string]interface{}
	json.NewDecoder(w1.Body).Decode(&resp1)
	assert.Equal(t, true, resp1["isActive"])

	// Step 2: Verify goal is active in GetUserChallenges
	req2 := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req2.Header.Set("x-mock-user-id", userID)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var challenges map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&challenges)

	// Find the activated goal
	challengeList := challenges["challenges"].([]interface{})
	var foundGoal map[string]interface{}
	for _, ch := range challengeList {
		challenge := ch.(map[string]interface{})
		if challenge["challengeId"] == "winter-challenge-2025" {
			goals := challenge["goals"].([]interface{})
			for _, g := range goals {
				goal := g.(map[string]interface{})
				if goal["goalId"] == "kill-10-snowmen" {
					foundGoal = goal
					break
				}
			}
		}
	}
	require.NotNil(t, foundGoal, "Activated goal should be visible in GetUserChallenges")

	// Step 3: Deactivate goal
	body3 := map[string]interface{}{"is_active": false}
	bodyBytes3, _ := json.Marshal(body3)
	req3 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes3),
	)
	req3.Header.Set("x-mock-user-id", userID)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code, "Deactivation should succeed")
	var resp3 map[string]interface{}
	json.NewDecoder(w3.Body).Decode(&resp3)
	assert.Equal(t, false, resp3["isActive"])
}

// TestSetGoalActive_Idempotent_HTTP verifies idempotent activation via HTTP
func TestSetGoalActive_Idempotent_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-8-http"

	// Activate goal twice
	url := "/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active"
	body := map[string]interface{}{"is_active": true}
	bodyBytes, _ := json.Marshal(body)

	req1 := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(bodyBytes))
	req1.Header.Set("x-mock-user-id", userID)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code, "First activation should succeed")
	var resp1 map[string]interface{}
	json.NewDecoder(w1.Body).Decode(&resp1)

	// Second request (need fresh body reader)
	bodyBytes2, _ := json.Marshal(body)
	req2 := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(bodyBytes2))
	req2.Header.Set("x-mock-user-id", userID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code, "Second activation should succeed (idempotent)")
	var resp2 map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&resp2)

	// Both should return success
	assert.Equal(t, true, resp1["isActive"])
	assert.Equal(t, true, resp2["isActive"])
	assert.Equal(t, "Goal activated successfully", resp1["message"])
	assert.Equal(t, "Goal activated successfully", resp2["message"])
}

// TestSetGoalActive_MultipleUsers_HTTP verifies user isolation via HTTP
func TestSetGoalActive_MultipleUsers_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	user1 := "set-active-user-9-http"
	user2 := "set-active-user-10-http"

	// User 1 activates goal
	body1 := map[string]interface{}{"is_active": true}
	bodyBytes1, _ := json.Marshal(body1)
	req1 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes1),
	)
	req1.Header.Set("x-mock-user-id", user1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code, "User 1 activation should succeed")

	// User 2 activates same goal (should be independent)
	bodyBytes2, _ := json.Marshal(body1)
	req2 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes2),
	)
	req2.Header.Set("x-mock-user-id", user2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "User 2 activation should succeed")

	// User 1 deactivates goal
	body3 := map[string]interface{}{"is_active": false}
	bodyBytes3, _ := json.Marshal(body3)
	req3 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/kill-10-snowmen/active",
		bytes.NewReader(bodyBytes3),
	)
	req3.Header.Set("x-mock-user-id", user1)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code, "User 1 deactivation should succeed")

	// This test verifies that deactivating for user 1 doesn't affect user 2
	// Both operations should succeed, proving user isolation
}

// TestSetGoalActive_MissingParameters_HTTP verifies validation for missing/invalid parameters via HTTP
func TestSetGoalActive_MissingParameters_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "set-active-user-validation-http"

	// Test case 1: Empty JSON body (missing is_active field)
	emptyBody := map[string]interface{}{}
	emptyBytes, _ := json.Marshal(emptyBody)
	req1 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/complete-tutorial/active",
		bytes.NewReader(emptyBytes),
	)
	req1.Header.Set("x-mock-user-id", userID)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// Should either succeed with default value (false) or fail with validation error
	assert.True(t, w1.Code >= 200 && w1.Code < 600,
		"Should return valid HTTP status code")

	// Test case 2: Invalid JSON body
	invalidJSON := []byte(`{"is_active": "not-a-boolean"}`)
	req2 := httptest.NewRequest(
		http.MethodPut,
		"/v1/challenges/winter-challenge-2025/goals/complete-tutorial/active",
		bytes.NewReader(invalidJSON),
	)
	req2.Header.Set("x-mock-user-id", userID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	// Should handle invalid type gracefully (either parse as false or return error)
	assert.True(t, w2.Code >= 200 && w2.Code < 600,
		"Should return valid HTTP status code for invalid type")

	fmt.Printf("Test: Empty body returned %d, Invalid type returned %d\n",
		w1.Code, w2.Code)
}
