// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package response

import (
	"encoding/json"
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

	result, err := InjectProgressIntoChallenge(staticJSON, progress)
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

	result, err := InjectProgressIntoChallenge(staticJSON, nil)
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

	result, err := InjectProgressIntoChallenge(staticJSON, progress)
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

// BenchmarkInjectProgressIntoChallenge benchmarks full challenge injection
func BenchmarkInjectProgressIntoChallenge(b *testing.B) {
	staticJSON := []byte(`{"challengeId":"daily","name":"Daily Challenge","description":"Complete daily tasks","goals":[{"goalId":"g1","name":"Goal 1","targetValue":10},{"goalId":"g2","name":"Goal 2","targetValue":20},{"goalId":"g3","name":"Goal 3","targetValue":30},{"goalId":"g4","name":"Goal 4","targetValue":40},{"goalId":"g5","name":"Goal 5","targetValue":50}]}`)

	progress := map[string]*commonDomain.UserGoalProgress{
		"g1": {GoalID: "g1", Progress: 5, Status: "in_progress"},
		"g2": {GoalID: "g2", Progress: 10, Status: "in_progress"},
		"g3": {GoalID: "g3", Progress: 30, Status: "completed"},
		"g4": {GoalID: "g4", Progress: 0, Status: "not_started"},
		"g5": {GoalID: "g5", Progress: 25, Status: "in_progress"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = InjectProgressIntoChallenge(staticJSON, progress)
	}
}
