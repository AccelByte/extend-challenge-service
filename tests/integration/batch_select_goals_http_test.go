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

// TestBatchSelectGoals_Success_HTTP verifies successful batch goal selection via HTTP
func TestBatchSelectGoals_Success_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-1-http"

	// POST /v1/challenges/{challenge_id}/goals/batch-select with JSON body
	body := map[string]interface{}{
		"goal_ids":         []string{"kill-10-snowmen", "reach-level-5"},
		"replace_existing": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "BatchSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Validate response fields (camelCase!)
	assert.Equal(t, "winter-challenge-2025", resp["challengeId"])
	assert.Equal(t, float64(2), resp["totalActiveGoals"], "Should have 2 active goals")

	// Verify selected goals array
	selectedGoals, ok := resp["selectedGoals"].([]interface{})
	require.True(t, ok, "selectedGoals should be array")
	assert.Len(t, selectedGoals, 2, "Should return 2 selected goals")

	// Verify database state
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"))
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "reach-level-5"))
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestBatchSelectGoals_ReplaceExisting_HTTP verifies replacing existing active goals via HTTP
func TestBatchSelectGoals_ReplaceExisting_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-2-http"

	// Seed existing active goals
	seedActiveGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedActiveGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	// Verify initial state
	assert.Equal(t, 2, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))

	// Replace with new goals
	body := map[string]interface{}{
		"goal_ids":         []string{"reach-level-5"},
		"replace_existing": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "BatchSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, float64(1), resp["totalActiveGoals"], "Should have 1 active goal")

	// Verify replaced goals
	replacedGoals, ok := resp["replacedGoals"].([]interface{})
	require.True(t, ok, "replacedGoals should be array")
	assert.Len(t, replacedGoals, 2, "Should replace 2 goals")

	// Verify database state
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "complete-tutorial"))
	assert.False(t, getGoalActiveStatus(t, testDB, userID, "kill-10-snowmen"))
	assert.True(t, getGoalActiveStatus(t, testDB, userID, "reach-level-5"))
	assert.Equal(t, 1, countActiveGoals(t, testDB, userID, "winter-challenge-2025"))
}

// TestBatchSelectGoals_EmptyGoalList_HTTP verifies error handling for empty goal list via HTTP
func TestBatchSelectGoals_EmptyGoalList_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-3-http"

	// Try to select with empty goal list
	body := map[string]interface{}{
		"goal_ids":         []string{},
		"replace_existing": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
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

	// Verify error details
	assert.Contains(t, resp["message"], "cannot be empty")
}

// TestBatchSelectGoals_InvalidGoalID_HTTP verifies error handling for invalid goal ID via HTTP
func TestBatchSelectGoals_InvalidGoalID_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-4-http"

	// Try to select non-existent goal
	body := map[string]interface{}{
		"goal_ids":         []string{"kill-10-snowmen", "invalid-goal-id"},
		"replace_existing": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify error response
	assert.Equal(t, http.StatusInternalServerError, w.Code, "Should return 500 Internal Server Error")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	assert.Contains(t, resp["message"], "not found")
}

// TestBatchSelectGoals_WrongChallenge_HTTP verifies error handling for goal from wrong challenge via HTTP
func TestBatchSelectGoals_WrongChallenge_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-5-http"

	// Try to select goal from different challenge
	body := map[string]interface{}{
		"goal_ids":         []string{"login-today"}, // This belongs to daily-quests
		"replace_existing": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify error response
	assert.Equal(t, http.StatusInternalServerError, w.Code, "Should return 500 Internal Server Error")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Error response should be valid JSON")

	assert.Contains(t, resp["message"], "does not belong to challenge")
}

// TestBatchSelectGoals_VerifyTimestamps_HTTP verifies assigned_at timestamp format via HTTP
func TestBatchSelectGoals_VerifyTimestamps_HTTP(t *testing.T) {
	handler, _, cleanup := setupHTTPTestServer(t)
	defer cleanup()

	userID := "batch-user-6-http"

	body := map[string]interface{}{
		"goal_ids":         []string{"kill-10-snowmen"},
		"replace_existing": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/challenges/winter-challenge-2025/goals/batch-select",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("x-mock-user-id", userID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code, "BatchSelectGoals should return 200 OK")

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify selectedGoals array
	selectedGoals, ok := resp["selectedGoals"].([]interface{})
	require.True(t, ok, "selectedGoals should be array")
	require.Len(t, selectedGoals, 1)

	goal := selectedGoals[0].(map[string]interface{})
	assignedAt, ok := goal["assignedAt"].(string)
	require.True(t, ok, "assignedAt should be string")
	assert.NotEmpty(t, assignedAt, "assignedAt should be set")

	// Validate timestamp format (RFC3339)
	_, err = time.Parse(time.RFC3339, assignedAt)
	assert.NoError(t, err, "assignedAt should be valid RFC3339 timestamp")
}
