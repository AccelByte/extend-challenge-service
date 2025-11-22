// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRandomSelectGoals_Success_HTTP verifies successful random goal selection via HTTP
func TestRandomSelectGoals_Success_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-1-http"

	// Complete prerequisite goals so all goals are available
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// POST /v1/challenges/{challenge_id}/goals/random-select with JSON body
	body := map[string]interface{}{
		"count":            2,
		"replace_existing": false,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "RandomSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Validate response fields (camelCase!)
	assert.Equal(t, "winter-challenge-2025", resp["challengeId"])
	assert.Equal(t, float64(1), resp["totalActiveGoals"], "Should have 1 active goal")

	// Verify selected goals array
	selectedGoals, ok := resp["selectedGoals"].([]interface{})
	require.True(t, ok, "selectedGoals should be array")
	assert.Len(t, selectedGoals, 1, "Should return 1 available goal")

	// Verify selected goal
	goal := selectedGoals[0].(map[string]interface{})
	assert.Equal(t, "reach-level-5", goal["goalId"], "Should select uncompleted goal")
	assert.Equal(t, true, goal["isActive"], "Goal should be active")
	assert.NotEmpty(t, goal["assignedAt"], "assignedAt should be set")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestRandomSelectGoals_ReplaceExisting_HTTP verifies replacing existing active goals via HTTP
func TestRandomSelectGoals_ReplaceExisting_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-2-http"

	// Seed existing active goals (kill-10-snowmen must be completed for reach-level-5 prerequisite)
	seedActiveGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Verify initial state
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Replace with 1 random goal
	body := map[string]interface{}{
		"count":            1,
		"replace_existing": true,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "RandomSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, float64(1), resp["totalActiveGoals"], "Should have 1 active goal")

	// Verify replaced goals
	replacedGoals, ok := resp["replacedGoals"].([]interface{})
	require.True(t, ok, "replacedGoals should be array")
	assert.Len(t, replacedGoals, 2, "Should replace 2 goals")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Old goals should be inactive
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "complete-tutorial"))
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"))
}

// TestRandomSelectGoals_ExcludeActive_HTTP verifies excluding already active goals via HTTP
func TestRandomSelectGoals_ExcludeActive_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-3-http"

	// Seed one active completed goal (must be completed for reach-level-5 prerequisite)
	seedCompletedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Request 2 goals, excluding active ones
	body := map[string]interface{}{
		"count":            2,
		"replace_existing": false,
		"exclude_active":   true, // Exclude kill-10-snowmen
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "RandomSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, float64(3), resp["totalActiveGoals"], "Should have 3 active goals")

	// Verify database state
	assert.Equal(t, 3, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"), "Original goal should still be active")
}

// TestRandomSelectGoals_PartialResults_HTTP verifies handling when fewer goals available than requested via HTTP
func TestRandomSelectGoals_PartialResults_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-4-http"

	// Seed prerequisites as completed so reach-level-5 becomes available
	// (complete-tutorial and kill-10-snowmen are completed, so only reach-level-5 is available)
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Request 5 goals but only 1 is available (reach-level-5)
	body := map[string]interface{}{
		"count":            5,
		"replace_existing": false,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "RandomSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify selected goals array
	selectedGoals, ok := resp["selectedGoals"].([]interface{})
	require.True(t, ok, "selectedGoals should be array")
	assert.Len(t, selectedGoals, 1, "Should return 1 available goal (reach-level-5)")

	assert.Equal(t, float64(1), resp["totalActiveGoals"], "Should have 1 active goal")

	// Verify database state
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestRandomSelectGoals_InvalidCount_HTTP verifies error handling for invalid count via HTTP
func TestRandomSelectGoals_InvalidCount_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-5-http"

	// Try to select with count <= 0
	body := map[string]interface{}{
		"count":            0, // Invalid
		"replace_existing": false,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify error response
	assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 Bad Request")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	assert.Contains(t, resp["message"], "count must be greater than 0")
}

// TestRandomSelectGoals_ChallengeNotFound_HTTP verifies error handling for non-existent challenge via HTTP
func TestRandomSelectGoals_ChallengeNotFound_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-6-http"

	// Try to select from non-existent challenge
	body := map[string]interface{}{
		"count":            2,
		"replace_existing": false,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/nonexistent-challenge/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify error response
	assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 Not Found")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	assert.Contains(t, resp["message"], "not found")
}

// TestRandomSelectGoals_VerifyTimestamps_HTTP verifies assigned_at timestamp format via HTTP
func TestRandomSelectGoals_VerifyTimestamps_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "random-user-7-http"

	// Request 1 goal (complete-tutorial is available by default)
	body := map[string]interface{}{
		"count":            1,
		"replace_existing": false,
		"exclude_active":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/random-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "RandomSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify selectedGoals array
	selectedGoals, ok := resp["selectedGoals"].([]interface{})
	require.True(t, ok, "selectedGoals should be array")
	require.Len(t, selectedGoals, 1)

	// Verify all timestamps are valid
	for _, g := range selectedGoals {
		goal := g.(map[string]interface{})
		assignedAt, ok := goal["assignedAt"].(string)
		require.True(t, ok, "assignedAt should be string")
		assert.NotEmpty(t, assignedAt, "assignedAt should be set")

		// Validate timestamp format (RFC3339)
		_, err = time.Parse(time.RFC3339, assignedAt)
		assert.NoError(t, err, "assignedAt should be valid RFC3339 timestamp")
	}
}
