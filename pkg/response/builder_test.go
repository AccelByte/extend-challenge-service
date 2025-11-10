// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package response

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"extend-challenge-service/pkg/cache"
	pb "extend-challenge-service/pkg/pb"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// createTestCache creates a SerializedChallengeCache populated with test data
func createTestCache(t *testing.T) *cache.SerializedChallengeCache {
	t.Helper()

	c := cache.NewSerializedChallengeCache()

	// Create test challenges
	challenges := []*pb.Challenge{
		{
			ChallengeId: "challenge1",
			Name:        "Test Challenge 1",
			Description: "First test challenge",
			Goals: []*pb.Goal{
				{
					GoalId:      "goal1",
					Name:        "Test Goal 1",
					Description: "First test goal",
					Requirement: &pb.Requirement{
						StatCode:    "kills",
						Operator:    ">=",
						TargetValue: 10,
					},
					Reward: &pb.Reward{
						Type:     "ITEM",
						RewardId: "sword",
						Quantity: 1,
					},
				},
				{
					GoalId:      "goal2",
					Name:        "Test Goal 2",
					Description: "Second test goal",
					Requirement: &pb.Requirement{
						StatCode:    "wins",
						Operator:    ">=",
						TargetValue: 5,
					},
					Reward: &pb.Reward{
						Type:     "WALLET",
						RewardId: "gold",
						Quantity: 100,
					},
				},
			},
		},
		{
			ChallengeId: "challenge2",
			Name:        "Test Challenge 2",
			Description: "Second test challenge",
			Goals: []*pb.Goal{
				{
					GoalId:      "goal3",
					Name:        "Test Goal 3",
					Description: "Third test goal",
					Requirement: &pb.Requirement{
						StatCode:    "logins",
						Operator:    ">=",
						TargetValue: 3,
					},
					Reward: &pb.Reward{
						Type:     "ITEM",
						RewardId: "potion",
						Quantity: 5,
					},
				},
			},
		},
	}

	err := c.WarmUp(challenges)
	require.NoError(t, err, "Failed to warm up cache")

	return c
}

func TestNewChallengeResponseBuilder(t *testing.T) {
	cache := createTestCache(t)

	builder := NewChallengeResponseBuilder(cache)

	assert.NotNil(t, builder)
	assert.Equal(t, cache, builder.cache)
}

func TestNewChallengeResponseBuilder_NilCache(t *testing.T) {
	builder := NewChallengeResponseBuilder(nil)

	assert.NotNil(t, builder)
	assert.Nil(t, builder.cache)
}

func TestBuildChallengesResponse_EmptyChallenges(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	result, err := builder.BuildChallengesResponse([]string{}, map[string]*commonDomain.UserGoalProgress{})

	require.NoError(t, err)
	assert.Equal(t, `{"challenges":[]}`, string(result))
}

func TestBuildChallengesResponse_SingleChallenge_NoProgress(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	challengeIDs := []string{"challenge1"}
	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildChallengesResponse(challengeIDs, userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result to validate structure
	var response map[string]interface{}
	err = json.Unmarshal(result, &response)
	require.NoError(t, err, "Result should be valid JSON")

	challenges, ok := response["challenges"].([]interface{})
	require.True(t, ok, "Response should have 'challenges' array")
	assert.Len(t, challenges, 1)

	challenge := challenges[0].(map[string]interface{})
	assert.Equal(t, "challenge1", challenge["challengeId"])
	assert.Equal(t, "Test Challenge 1", challenge["name"])

	// Verify goals have default progress
	goals := challenge["goals"].([]interface{})
	assert.Len(t, goals, 2)

	goal1 := goals[0].(map[string]interface{})
	assert.Equal(t, "goal1", goal1["goalId"])
	assert.Equal(t, float64(0), goal1["progress"])
	assert.Equal(t, "not_started", goal1["status"])
}

func TestBuildChallengesResponse_SingleChallenge_WithProgress(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	challengeIDs := []string{"challenge1"}
	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	userProgress := map[string]*commonDomain.UserGoalProgress{
		"goal1": {
			UserID:      "user123",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Progress:    5,
			Status:      commonDomain.GoalStatusInProgress,
			CompletedAt: &completedAt,
		},
	}

	result, err := builder.BuildChallengesResponse(challengeIDs, userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result to validate progress injection
	var response map[string]interface{}
	err = json.Unmarshal(result, &response)
	require.NoError(t, err)

	challenges := response["challenges"].([]interface{})
	challenge := challenges[0].(map[string]interface{})
	goals := challenge["goals"].([]interface{})
	goal1 := goals[0].(map[string]interface{})

	assert.Equal(t, float64(5), goal1["progress"])
	assert.Equal(t, "in_progress", goal1["status"])
	assert.Equal(t, "2025-01-15T10:30:00Z", goal1["completedAt"])
}

func TestBuildChallengesResponse_MultipleChallenges(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	challengeIDs := []string{"challenge1", "challenge2"}
	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildChallengesResponse(challengeIDs, userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var response map[string]interface{}
	err = json.Unmarshal(result, &response)
	require.NoError(t, err)

	challenges := response["challenges"].([]interface{})
	assert.Len(t, challenges, 2)

	challenge1 := challenges[0].(map[string]interface{})
	assert.Equal(t, "challenge1", challenge1["challengeId"])

	challenge2 := challenges[1].(map[string]interface{})
	assert.Equal(t, "challenge2", challenge2["challengeId"])
}

func TestBuildChallengesResponse_NilCache(t *testing.T) {
	builder := NewChallengeResponseBuilder(nil)

	challengeIDs := []string{"challenge1"}
	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildChallengesResponse(challengeIDs, userProgress)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache is nil")
	assert.Nil(t, result)
}

func TestBuildChallengesResponse_ChallengeNotFound(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	challengeIDs := []string{"nonexistent-challenge"}
	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildChallengesResponse(challengeIDs, userProgress)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in serialization cache")
	assert.Nil(t, result)
}

func TestBuildSingleChallenge_Success(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildSingleChallenge("challenge1", userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var challenge map[string]interface{}
	err = json.Unmarshal(result, &challenge)
	require.NoError(t, err)

	assert.Equal(t, "challenge1", challenge["challengeId"])
	assert.Equal(t, "Test Challenge 1", challenge["name"])

	goals := challenge["goals"].([]interface{})
	assert.Len(t, goals, 2)
}

func TestBuildSingleChallenge_WithProgress(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	claimedAt := time.Date(2025, 1, 15, 11, 00, 0, 0, time.UTC)
	userProgress := map[string]*commonDomain.UserGoalProgress{
		"goal1": {
			UserID:      "user123",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Progress:    10,
			Status:      commonDomain.GoalStatusClaimed,
			CompletedAt: &completedAt,
			ClaimedAt:   &claimedAt,
		},
		"goal2": {
			UserID:      "user123",
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Progress:    3,
			Status:      commonDomain.GoalStatusInProgress,
			CompletedAt: nil,
			ClaimedAt:   nil,
		},
	}

	result, err := builder.BuildSingleChallenge("challenge1", userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var challenge map[string]interface{}
	err = json.Unmarshal(result, &challenge)
	require.NoError(t, err)

	goals := challenge["goals"].([]interface{})
	goal1 := goals[0].(map[string]interface{})
	goal2 := goals[1].(map[string]interface{})

	// Verify goal1 progress
	assert.Equal(t, float64(10), goal1["progress"])
	assert.Equal(t, "claimed", goal1["status"])
	assert.Equal(t, "2025-01-15T10:30:00Z", goal1["completedAt"])
	assert.Equal(t, "2025-01-15T11:00:00Z", goal1["claimedAt"])

	// Verify goal2 progress
	assert.Equal(t, float64(3), goal2["progress"])
	assert.Equal(t, "in_progress", goal2["status"])
}

func TestBuildSingleChallenge_NotFound(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	userProgress := map[string]*commonDomain.UserGoalProgress{}

	result, err := builder.BuildSingleChallenge("nonexistent", userProgress)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in serialization cache")
	assert.Nil(t, result)
}

func TestBuildGoalResponse_Success(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	userProgress := &commonDomain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal1",
		ChallengeID: "challenge1",
		Progress:    0,
		Status:      commonDomain.GoalStatusNotStarted,
		CompletedAt: nil,
		ClaimedAt:   nil,
	}

	result, err := builder.BuildGoalResponse("goal1", userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var goal map[string]interface{}
	err = json.Unmarshal(result, &goal)
	require.NoError(t, err)

	assert.Equal(t, "goal1", goal["goalId"])
	assert.Equal(t, "Test Goal 1", goal["name"])
	assert.Equal(t, float64(0), goal["progress"])
	assert.Equal(t, "not_started", goal["status"])
}

func TestBuildGoalResponse_WithProgress(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	userProgress := &commonDomain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal2",
		ChallengeID: "challenge1",
		Progress:    5,
		Status:      commonDomain.GoalStatusCompleted,
		CompletedAt: &completedAt,
		ClaimedAt:   nil,
	}

	result, err := builder.BuildGoalResponse("goal2", userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var goal map[string]interface{}
	err = json.Unmarshal(result, &goal)
	require.NoError(t, err)

	assert.Equal(t, "goal2", goal["goalId"])
	assert.Equal(t, float64(5), goal["progress"])
	assert.Equal(t, "completed", goal["status"])
	assert.Equal(t, "2025-01-15T10:30:00Z", goal["completedAt"])
	assert.Equal(t, "", goal["claimedAt"])
}

func TestBuildGoalResponse_Claimed(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	claimedAt := time.Date(2025, 1, 15, 11, 00, 0, 0, time.UTC)
	userProgress := &commonDomain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal3",
		ChallengeID: "challenge2",
		Progress:    3,
		Status:      commonDomain.GoalStatusClaimed,
		CompletedAt: &completedAt,
		ClaimedAt:   &claimedAt,
	}

	result, err := builder.BuildGoalResponse("goal3", userProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var goal map[string]interface{}
	err = json.Unmarshal(result, &goal)
	require.NoError(t, err)

	assert.Equal(t, "goal3", goal["goalId"])
	assert.Equal(t, float64(3), goal["progress"])
	assert.Equal(t, "claimed", goal["status"])
	assert.Equal(t, "2025-01-15T10:30:00Z", goal["completedAt"])
	assert.Equal(t, "2025-01-15T11:00:00Z", goal["claimedAt"])
}

func TestBuildGoalResponse_NilProgress(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	result, err := builder.BuildGoalResponse("goal1", nil)

	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse result
	var goal map[string]interface{}
	err = json.Unmarshal(result, &goal)
	require.NoError(t, err)

	// Should have default values
	assert.Equal(t, float64(0), goal["progress"])
	assert.Equal(t, "not_started", goal["status"])
	assert.Equal(t, "", goal["completedAt"])
	assert.Equal(t, "", goal["claimedAt"])
}

func TestBuildGoalResponse_NotFound(t *testing.T) {
	cache := createTestCache(t)
	builder := NewChallengeResponseBuilder(cache)

	userProgress := &commonDomain.UserGoalProgress{
		UserID:   "user123",
		GoalID:   "nonexistent",
		Progress: 5,
		Status:   commonDomain.GoalStatusInProgress,
	}

	result, err := builder.BuildGoalResponse("nonexistent", userProgress)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in serialization cache")
	assert.Nil(t, result)
}
