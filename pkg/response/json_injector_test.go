// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// TestInjectProgressIntoGoal_NoProgress tests injecting defaults when no progress exists
func TestInjectProgressIntoGoal_NoProgress(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Test Goal","targetValue":10}`)

	result := InjectProgressIntoGoal(staticJSON, nil)

	// Validate JSON is parseable
	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Validate default fields
	if goal["progress"].(float64) != 0 {
		t.Errorf("Expected progress=0, got %v", goal["progress"])
	}
	if goal["status"].(string) != "not_started" {
		t.Errorf("Expected status=not_started, got %v", goal["status"])
	}
	if goal["completedAt"].(string) != "" {
		t.Errorf("Expected completed_at empty, got %v", goal["completedAt"])
	}
	if goal["claimedAt"].(string) != "" {
		t.Errorf("Expected claimed_at empty, got %v", goal["claimedAt"])
	}

	// Validate original fields are preserved
	if goal["goalId"].(string) != "g1" {
		t.Errorf("goal_id not preserved")
	}
	if goal["name"].(string) != "Test Goal" {
		t.Errorf("name not preserved")
	}
	if goal["targetValue"].(float64) != 10 {
		t.Errorf("target_value not preserved")
	}
}

// TestInjectProgressIntoGoal_WithProgress tests injecting actual user progress
func TestInjectProgressIntoGoal_WithProgress(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Test Goal","targetValue":10}`)

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	progress := &commonDomain.UserGoalProgress{
		GoalID:      "g1",
		Progress:    5,
		Status:      "in_progress",
		CompletedAt: &completedAt,
		ClaimedAt:   nil,
	}

	result := InjectProgressIntoGoal(staticJSON, progress)

	// Validate JSON is parseable
	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Validate injected fields
	if goal["progress"].(float64) != 5 {
		t.Errorf("Expected progress=5, got %v", goal["progress"])
	}
	if goal["status"].(string) != "in_progress" {
		t.Errorf("Expected status=in_progress, got %v", goal["status"])
	}
	if goal["completedAt"].(string) != "2025-01-15T10:30:00Z" {
		t.Errorf("Expected completed_at=2025-01-15T10:30:00Z, got %v", goal["completedAt"])
	}
	if goal["claimedAt"].(string) != "" {
		t.Errorf("Expected claimed_at empty, got %v", goal["claimedAt"])
	}
}

// TestInjectProgressIntoGoal_Completed tests a completed goal
func TestInjectProgressIntoGoal_Completed(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Test Goal","targetValue":10}`)

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	claimedAt := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)
	progress := &commonDomain.UserGoalProgress{
		GoalID:      "g1",
		Progress:    10,
		Status:      "claimed",
		CompletedAt: &completedAt,
		ClaimedAt:   &claimedAt,
	}

	result := InjectProgressIntoGoal(staticJSON, progress)

	// Validate JSON
	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Validate fields
	if goal["progress"].(float64) != 10 {
		t.Errorf("Expected progress=10, got %v", goal["progress"])
	}
	if goal["status"].(string) != "claimed" {
		t.Errorf("Expected status=claimed, got %v", goal["status"])
	}
	if goal["completedAt"].(string) != "2025-01-15T10:30:00Z" {
		t.Errorf("Expected completed_at, got %v", goal["completedAt"])
	}
	if goal["claimedAt"].(string) != "2025-01-15T10:35:00Z" {
		t.Errorf("Expected claimed_at, got %v", goal["claimedAt"])
	}
}

// TestInjectProgressIntoGoal_ComplexJSON tests injection into complex goal JSON
func TestInjectProgressIntoGoal_ComplexJSON(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Test Goal","description":"A complex goal","targetValue":100,"requirements":{"statCode":"DAILY_LOGIN","operator":"GTE","targetValue":7},"rewards":[{"type":"ITEM","rewardId":"GOLD","quantity":100}],"prerequisites":["g0"],"locked":false}`)

	progress := &commonDomain.UserGoalProgress{
		GoalID:   "g1",
		Progress: 50,
		Status:   "in_progress",
	}

	result := InjectProgressIntoGoal(staticJSON, progress)

	// Validate JSON
	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v\nJSON: %s", err, string(result))
	}

	// Validate original fields preserved
	if goal["goalId"].(string) != "g1" {
		t.Errorf("goal_id not preserved")
	}
	if goal["name"].(string) != "Test Goal" {
		t.Errorf("name not preserved")
	}
	if goal["description"].(string) != "A complex goal" {
		t.Errorf("description not preserved")
	}
	if goal["targetValue"].(float64) != 100 {
		t.Errorf("target_value not preserved")
	}

	// Validate nested objects preserved
	requirements := goal["requirements"].(map[string]interface{})
	if requirements["statCode"].(string) != "DAILY_LOGIN" {
		t.Errorf("requirements.stat_code not preserved")
	}

	rewards := goal["rewards"].([]interface{})
	if len(rewards) != 1 {
		t.Errorf("rewards array not preserved")
	}

	prerequisites := goal["prerequisites"].([]interface{})
	if len(prerequisites) != 1 {
		t.Errorf("prerequisites array not preserved")
	}

	if goal["locked"].(bool) != false {
		t.Errorf("locked not preserved")
	}

	// Validate injected fields
	if goal["progress"].(float64) != 50 {
		t.Errorf("Expected progress=50, got %v", goal["progress"])
	}
	if goal["status"].(string) != "in_progress" {
		t.Errorf("Expected status=in_progress, got %v", goal["status"])
	}
}

// TestInjectProgressIntoChallenge tests injecting progress into a full challenge
func TestInjectProgressIntoChallenge(t *testing.T) {
	staticJSON := []byte(`{"challengeId":"daily","name":"Daily Challenge","goals":[{"goalId":"g1","name":"Goal 1","targetValue":10},{"goalId":"g2","name":"Goal 2","targetValue":20}]}`)

	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {
			GoalID:   "g1",
			Progress: 5,
			Status:   "in_progress",
		},
		"g2": {
			GoalID:   "g2",
			Progress: 20,
			Status:   "completed",
		},
	}

	goalCount := 2

	result, err := InjectProgressIntoChallenge(staticJSON, progress, goalCount)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	// Validate JSON
	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v\nJSON: %s", err, string(result))
	}

	// Validate challenge fields preserved
	if challenge["challengeId"].(string) != "daily" {
		t.Errorf("challenge_id not preserved")
	}
	if challenge["name"].(string) != "Daily Challenge" {
		t.Errorf("name not preserved")
	}

	// Validate goals array
	goals := challenge["goals"].([]interface{})
	if len(goals) != 2 {
		t.Fatalf("Expected 2 goals, got %d", len(goals))
	}

	// Validate goal 1
	goal1 := goals[0].(map[string]interface{})
	if goal1["goalId"].(string) != "g1" {
		t.Errorf("goal1.goal_id not preserved")
	}
	if goal1["progress"].(float64) != 5 {
		t.Errorf("Expected goal1.progress=5, got %v", goal1["progress"])
	}
	if goal1["status"].(string) != "in_progress" {
		t.Errorf("Expected goal1.status=in_progress, got %v", goal1["status"])
	}

	// Validate goal 2
	goal2 := goals[1].(map[string]interface{})
	if goal2["goalId"].(string) != "g2" {
		t.Errorf("goal2.goal_id not preserved")
	}
	if goal2["progress"].(float64) != 20 {
		t.Errorf("Expected goal2.progress=20, got %v", goal2["progress"])
	}
	if goal2["status"].(string) != "completed" {
		t.Errorf("Expected goal2.status=completed, got %v", goal2["status"])
	}
}

// TestInjectProgressIntoChallenge_NoGoals tests challenge with no goals
func TestInjectProgressIntoChallenge_NoGoals(t *testing.T) {
	staticJSON := []byte(`{"challengeId":"daily","name":"Daily Challenge"}`)

	goalCount := 0

	result, err := InjectProgressIntoChallenge(staticJSON, nil, goalCount)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	// Validate JSON
	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Should be unchanged
	if challenge["challengeId"].(string) != "daily" {
		t.Errorf("challenge_id not preserved")
	}
}

// TestInjectProgressIntoChallenge_MissingProgress tests goals with missing progress
func TestInjectProgressIntoChallenge_MissingProgress(t *testing.T) {
	staticJSON := []byte(`{"challengeId":"daily","goals":[{"goalId":"g1","name":"Goal 1"},{"goalId":"g2","name":"Goal 2"}]}`)

	// Only provide progress for g1, not g2
	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {
			GoalID:   "g1",
			Progress: 5,
			Status:   "in_progress",
		},
	}

	goalCount := 2

	result, err := InjectProgressIntoChallenge(staticJSON, progress, goalCount)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	// Validate JSON
	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	goals := challenge["goals"].([]interface{})

	// g1 should have progress
	goal1 := goals[0].(map[string]interface{})
	if goal1["progress"].(float64) != 5 {
		t.Errorf("Expected goal1.progress=5, got %v", goal1["progress"])
	}

	// g2 should have defaults
	goal2 := goals[1].(map[string]interface{})
	if goal2["progress"].(float64) != 0 {
		t.Errorf("Expected goal2.progress=0, got %v", goal2["progress"])
	}
	if goal2["status"].(string) != "not_started" {
		t.Errorf("Expected goal2.status=not_started, got %v", goal2["status"])
	}
}

// TestExtractGoalID tests goal ID extraction
func TestExtractGoalID(t *testing.T) {
	tests := []struct {
		name        string
		goalJSON    []byte
		expectedID  string
		expectError bool
	}{
		{
			name:        "Simple goal",
			goalJSON:    []byte(`{"goalId":"g1","name":"Test"}`),
			expectedID:  "g1",
			expectError: false,
		},
		{
			name:        "Goal ID at end",
			goalJSON:    []byte(`{"name":"Test","goalId":"g2"}`),
			expectedID:  "g2",
			expectError: false,
		},
		{
			name:        "Missing goal_id",
			goalJSON:    []byte(`{"name":"Test"}`),
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Invalid JSON structure",
			goalJSON:    []byte(`{"goalId":123}`),
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := extractGoalID(tt.goalJSON)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if id != tt.expectedID {
					t.Errorf("Expected ID=%s, got %s", tt.expectedID, id)
				}
			}
		})
	}
}

// TestEscapeJSONString tests JSON string escaping
func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "normal string",
			expected: "normal string",
		},
		{
			input:    `string with "quotes"`,
			expected: `string with \"quotes\"`,
		},
		{
			input:    `string with \backslash`,
			expected: `string with \\backslash`,
		},
		{
			input:    "string with\nnewline",
			expected: "string with\\nnewline",
		},
		{
			input:    "string with\ttab",
			expected: "string with\\ttab",
		},
		{
			input:    "string with\rcarriage return",
			expected: "string with\\rcarriage return",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}

			// Validate the escaped string is valid in JSON
			jsonStr := `{"test":"` + result + `"}`
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
				t.Errorf("Escaped string is not valid JSON: %v\nJSON: %s", err, jsonStr)
			}
		})
	}
}

// TestInjectProgressIntoGoal_InvalidJSON tests error handling for invalid JSON
func TestInjectProgressIntoGoal_InvalidJSON(t *testing.T) {
	// Missing closing brace
	staticJSON := []byte(`{"goalId":"g1","name":"Test"`)

	result := InjectProgressIntoGoal(staticJSON, nil)

	// Should return unchanged (invalid JSON protection)
	if string(result) != string(staticJSON) {
		t.Errorf("Expected unchanged result for invalid JSON")
	}
}

// TestInjectProgressIntoChallenge_LargeGoalCount tests with 50+ goals to validate at scale
func TestInjectProgressIntoChallenge_LargeGoalCount(t *testing.T) {
	// Build a challenge JSON with 50 goals
	var goalsJSON bytes.Buffer
	goalsJSON.WriteString(`{"challengeId":"large","name":"Large Challenge","goals":[`)
	progress := make(map[string]*commonDomain.UserGoalProgress)
	for i := 0; i < 50; i++ {
		if i > 0 {
			goalsJSON.WriteByte(',')
		}
		goalID := fmt.Sprintf("g%d", i)
		fmt.Fprintf(&goalsJSON, `{"goalId":"%s","name":"Goal %d","targetValue":%d}`, goalID, i, (i+1)*10)
		progress[goalID] = &commonDomain.UserGoalProgress{
			GoalID:   goalID,
			Progress: i * 2,
			Status:   commonDomain.GoalStatus("in_progress"),
		}
	}
	goalsJSON.WriteString(`]}`)

	result, err := InjectProgressIntoChallenge(goalsJSON.Bytes(), progress, 50)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	goals := challenge["goals"].([]interface{})
	if len(goals) != 50 {
		t.Fatalf("Expected 50 goals, got %d", len(goals))
	}

	// Spot-check a few goals
	for _, idx := range []int{0, 24, 49} {
		goal := goals[idx].(map[string]interface{})
		expectedID := fmt.Sprintf("g%d", idx)
		if goal["goalId"].(string) != expectedID {
			t.Errorf("Goal %d: expected goalId=%s, got %v", idx, expectedID, goal["goalId"])
		}
		if int(goal["progress"].(float64)) != idx*2 {
			t.Errorf("Goal %d: expected progress=%d, got %v", idx, idx*2, goal["progress"])
		}
	}
}

// TestInjectProgressIntoChallenge_GoalWithNestedBraces tests goals with nested braces in string values
func TestInjectProgressIntoChallenge_GoalWithNestedBraces(t *testing.T) {
	// Goal with description containing JSON-like text with braces
	staticJSON := []byte(`{"challengeId":"tricky","goals":[{"goalId":"g1","name":"Goal 1","description":"Use {item} to win"},{"goalId":"g2","name":"Goal 2","description":"Collect {\"gold\":100}"}]}`)

	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {GoalID: "g1", Progress: 3, Status: "in_progress"},
		"g2": {GoalID: "g2", Progress: 100, Status: "completed"},
	}

	result, err := InjectProgressIntoChallenge(staticJSON, progress, 2)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v\nJSON: %s", err, string(result))
	}

	goals := challenge["goals"].([]interface{})
	if len(goals) != 2 {
		t.Fatalf("Expected 2 goals, got %d", len(goals))
	}

	goal1 := goals[0].(map[string]interface{})
	if goal1["description"].(string) != "Use {item} to win" {
		t.Errorf("Goal 1 description not preserved: %v", goal1["description"])
	}
	if goal1["progress"].(float64) != 3 {
		t.Errorf("Goal 1 progress wrong: %v", goal1["progress"])
	}

	goal2 := goals[1].(map[string]interface{})
	if goal2["description"].(string) != `Collect {"gold":100}` {
		t.Errorf("Goal 2 description not preserved: %v", goal2["description"])
	}
	if goal2["progress"].(float64) != 100 {
		t.Errorf("Goal 2 progress wrong: %v", goal2["progress"])
	}
}

// TestInjectProgressIntoChallenge_AllStatusTypes tests all status types across multiple goals
func TestInjectProgressIntoChallenge_AllStatusTypes(t *testing.T) {
	staticJSON := []byte(`{"challengeId":"statuses","goals":[{"goalId":"g1","name":"G1","targetValue":10},{"goalId":"g2","name":"G2","targetValue":20},{"goalId":"g3","name":"G3","targetValue":30},{"goalId":"g4","name":"G4","targetValue":40}]}`)

	completedAt := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	claimedAt := time.Date(2025, 6, 1, 12, 5, 0, 0, time.UTC)

	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {GoalID: "g1", Progress: 0, Status: "not_started"},
		"g2": {GoalID: "g2", Progress: 5, Status: "in_progress", IsActive: true},
		"g3": {GoalID: "g3", Progress: 30, Status: "completed", CompletedAt: &completedAt, IsActive: true},
		"g4": {GoalID: "g4", Progress: 40, Status: "claimed", CompletedAt: &completedAt, ClaimedAt: &claimedAt},
	}

	result, err := InjectProgressIntoChallenge(staticJSON, progress, 4)
	if err != nil {
		t.Fatalf("InjectProgressIntoChallenge failed: %v", err)
	}

	var challenge map[string]interface{}
	if err := json.Unmarshal(result, &challenge); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	goals := challenge["goals"].([]interface{})
	if len(goals) != 4 {
		t.Fatalf("Expected 4 goals, got %d", len(goals))
	}

	expectedStatuses := []string{"not_started", "in_progress", "completed", "claimed"}
	for i, expected := range expectedStatuses {
		goal := goals[i].(map[string]interface{})
		if goal["status"].(string) != expected {
			t.Errorf("Goal %d: expected status=%s, got %v", i, expected, goal["status"])
		}
	}

	// Verify claimed goal has both timestamps
	goal4 := goals[3].(map[string]interface{})
	if goal4["completedAt"].(string) != "2025-06-01T12:00:00Z" {
		t.Errorf("Goal 4 completedAt wrong: %v", goal4["completedAt"])
	}
	if goal4["claimedAt"].(string) != "2025-06-01T12:05:00Z" {
		t.Errorf("Goal 4 claimedAt wrong: %v", goal4["claimedAt"])
	}
}

// TestInjectProgressIntoGoal_WithExpiresAt tests expiresAt and expiresInSeconds injection
func TestInjectProgressIntoGoal_WithExpiresAt(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Rotating Goal","targetValue":10}`)

	// Set expiresAt to 1 hour in the future
	expiresAt := time.Now().UTC().Add(1 * time.Hour)
	progress := &commonDomain.UserGoalProgress{
		GoalID:    "g1",
		Progress:  3,
		Status:    "in_progress",
		IsActive:  true,
		ExpiresAt: &expiresAt,
	}

	result := InjectProgressIntoGoal(staticJSON, progress)

	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	if goal["isActive"].(bool) != true {
		t.Errorf("Expected isActive=true, got %v", goal["isActive"])
	}
	if goal["expiresAt"].(string) == "" {
		t.Error("Expected expiresAt to be non-empty")
	}

	expiresInSeconds := goal["expiresInSeconds"].(float64)
	// Should be approximately 3600 seconds (1 hour), allow 10s tolerance
	if expiresInSeconds < 3590 || expiresInSeconds > 3610 {
		t.Errorf("Expected expiresInSeconds ~3600, got %v", expiresInSeconds)
	}
}

// TestInjectProgressIntoGoal_WithExpiredExpiresAt tests expired goal (expiresInSeconds = 0)
func TestInjectProgressIntoGoal_WithExpiredExpiresAt(t *testing.T) {
	staticJSON := []byte(`{"goalId":"g1","name":"Expired Goal","targetValue":10}`)

	// Set expiresAt to 1 hour in the past
	expiresAt := time.Now().UTC().Add(-1 * time.Hour)
	progress := &commonDomain.UserGoalProgress{
		GoalID:    "g1",
		Progress:  3,
		Status:    "in_progress",
		ExpiresAt: &expiresAt,
	}

	result := InjectProgressIntoGoal(staticJSON, progress)

	var goal map[string]interface{}
	if err := json.Unmarshal(result, &goal); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	if goal["expiresInSeconds"].(float64) != 0 {
		t.Errorf("Expected expiresInSeconds=0 for expired goal, got %v", goal["expiresInSeconds"])
	}
}

// TestExtractGoalIDRange tests the byte-range based goal ID extraction
func TestExtractGoalIDRange(t *testing.T) {
	tests := []struct {
		name        string
		goalJSON    []byte
		expectedID  string
		expectError bool
	}{
		{
			name:        "Simple goal",
			goalJSON:    []byte(`{"goalId":"g1","name":"Test"}`),
			expectedID:  "g1",
			expectError: false,
		},
		{
			name:        "Goal ID at end",
			goalJSON:    []byte(`{"name":"Test","goalId":"g2"}`),
			expectedID:  "g2",
			expectError: false,
		},
		{
			name:        "Long goal ID",
			goalJSON:    []byte(`{"goalId":"daily_login_streak_bonus","name":"Test"}`),
			expectedID:  "daily_login_streak_bonus",
			expectError: false,
		},
		{
			name:        "Missing goalId",
			goalJSON:    []byte(`{"name":"Test"}`),
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Numeric goalId value",
			goalJSON:    []byte(`{"goalId":123}`),
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := extractGoalIDRange(tt.goalJSON)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			got := string(tt.goalJSON[start:end])
			if got != tt.expectedID {
				t.Errorf("Expected ID=%s, got %s", tt.expectedID, got)
			}
		})
	}
}

// TestWriteProgressFields_MatchesBuildProgressFields verifies the buffer-writing version
// produces identical output to the original buildProgressFields.
func TestWriteProgressFields_MatchesBuildProgressFields(t *testing.T) {
	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	claimedAt := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)
	expiresAt := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) // far future so expiresInSeconds is stable

	cases := []struct {
		name     string
		progress *commonDomain.UserGoalProgress
	}{
		{
			name: "in_progress with completedAt",
			progress: &commonDomain.UserGoalProgress{
				Progress: 5, Status: "in_progress", CompletedAt: &completedAt,
			},
		},
		{
			name: "claimed with both timestamps",
			progress: &commonDomain.UserGoalProgress{
				Progress: 10, Status: "claimed", CompletedAt: &completedAt, ClaimedAt: &claimedAt,
			},
		},
		{
			name: "not_started minimal",
			progress: &commonDomain.UserGoalProgress{
				Progress: 0, Status: "not_started",
			},
		},
		{
			name: "active with expiresAt",
			progress: &commonDomain.UserGoalProgress{
				Progress: 3, Status: "in_progress", IsActive: true, ExpiresAt: &expiresAt,
			},
		},
		{
			name: "inactive with no timestamps",
			progress: &commonDomain.UserGoalProgress{
				Progress: 0, Status: "not_started", IsActive: false,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use buildProgressFields (original)
			expected := buildProgressFields(tc.progress)

			// Use writeProgressFields (new buffer-writing version)
			var buf bytes.Buffer
			writeProgressFields(&buf, tc.progress)
			got := buf.Bytes()

			if !bytes.Equal(expected, got) {
				t.Errorf("Mismatch:\nexpected: %s\ngot:      %s", string(expected), string(got))
			}
		})
	}
}

// TestWriteDefaultProgressFields_MatchesBuildDefault verifies buffer default matches original
func TestWriteDefaultProgressFields_MatchesBuildDefault(t *testing.T) {
	expected := buildDefaultProgressFields()
	var buf bytes.Buffer
	writeDefaultProgressFields(&buf)
	got := buf.Bytes()

	if !bytes.Equal(expected, got) {
		t.Errorf("Mismatch:\nexpected: %s\ngot:      %s", string(expected), string(got))
	}
}

// BenchmarkInjectProgressIntoGoal benchmarks single goal injection
func BenchmarkInjectProgressIntoGoal(b *testing.B) {
	staticJSON := []byte(`{"goalId":"g1","name":"Test Goal","description":"A test goal","targetValue":100,"requirements":{"statCode":"DAILY_LOGIN","operator":"GTE","targetValue":7},"rewards":[{"type":"ITEM","rewardId":"GOLD","quantity":100}],"prerequisites":[],"locked":false}`)

	completedAt := time.Now()
	progress := &commonDomain.UserGoalProgress{
		GoalID:      "g1",
		Progress:    50,
		Status:      "in_progress",
		CompletedAt: &completedAt,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = InjectProgressIntoGoal(staticJSON, progress)
	}
}

// BenchmarkInjectProgressIntoChallenge benchmarks full challenge injection (5 goals)
func BenchmarkInjectProgressIntoChallenge(b *testing.B) {
	staticJSON := []byte(`{"challengeId":"daily","name":"Daily Challenge","description":"Complete daily tasks","goals":[{"goalId":"g1","name":"Goal 1","targetValue":10},{"goalId":"g2","name":"Goal 2","targetValue":20},{"goalId":"g3","name":"Goal 3","targetValue":30},{"goalId":"g4","name":"Goal 4","targetValue":40},{"goalId":"g5","name":"Goal 5","targetValue":50}]}`)

	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {GoalID: "g1", Progress: 5, Status: "in_progress"},
		"g2": {GoalID: "g2", Progress: 10, Status: "in_progress"},
		"g3": {GoalID: "g3", Progress: 30, Status: "completed"},
		"g4": {GoalID: "g4", Progress: 0, Status: "not_started"},
		"g5": {GoalID: "g5", Progress: 25, Status: "in_progress"},
	}

	goalCount := 5

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = InjectProgressIntoChallenge(staticJSON, progress, goalCount)
	}
}

// BenchmarkInjectProgressIntoChallenge_600Goals benchmarks with 600 goals (matches loadtest config)
func BenchmarkInjectProgressIntoChallenge_600Goals(b *testing.B) {
	// Build a challenge with 600 goals
	var challenge bytes.Buffer
	challenge.WriteString(`{"challengeId":"large","name":"Large Challenge","goals":[`)
	progress := make(map[string]*commonDomain.UserGoalProgress, 600)
	completedAt := time.Now().UTC()
	for i := 0; i < 600; i++ {
		if i > 0 {
			challenge.WriteByte(',')
		}
		goalID := fmt.Sprintf("goal_%03d", i)
		fmt.Fprintf(&challenge, `{"goalId":"%s","name":"Goal %d","targetValue":%d,"requirements":{"statCode":"STAT_%d","operator":"GTE","targetValue":%d}}`, goalID, i, (i+1)*10, i, (i+1)*5)
		progress[goalID] = &commonDomain.UserGoalProgress{
			GoalID:      goalID,
			Progress:    i * 3,
			Status:      commonDomain.GoalStatus("in_progress"),
			CompletedAt: &completedAt,
			IsActive:    true,
		}
	}
	challenge.WriteString(`]}`)

	staticJSON := challenge.Bytes()
	goalCount := 600

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = InjectProgressIntoChallenge(staticJSON, progress, goalCount)
	}
}
