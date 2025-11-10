// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package cache

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "extend-challenge-service/pkg/pb"
)

// createTestChallenges creates sample challenges for testing
func createTestChallenges() []*pb.Challenge {
	return []*pb.Challenge{
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
}

func TestNewSerializedChallengeCache(t *testing.T) {
	cache := NewSerializedChallengeCache()

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.challenges)
	assert.NotNil(t, cache.goals)
	assert.Equal(t, 0, len(cache.challenges), "Cache should be empty initially")
	assert.Equal(t, 0, len(cache.goals), "Cache should be empty initially")
}

func TestWarmUp_Success(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()

	err := cache.WarmUp(challenges)

	require.NoError(t, err)

	// Verify all challenges are cached
	challengeJSON1, ok := cache.GetChallengeJSON("challenge1")
	assert.True(t, ok, "Challenge1 should be in cache")
	assert.NotEmpty(t, challengeJSON1)

	challengeJSON2, ok := cache.GetChallengeJSON("challenge2")
	assert.True(t, ok, "Challenge2 should be in cache")
	assert.NotEmpty(t, challengeJSON2)

	// Verify all goals are cached
	goal1JSON, ok := cache.GetGoalJSON("goal1")
	assert.True(t, ok, "Goal1 should be in cache")
	assert.NotEmpty(t, goal1JSON)

	goal2JSON, ok := cache.GetGoalJSON("goal2")
	assert.True(t, ok, "Goal2 should be in cache")
	assert.NotEmpty(t, goal2JSON)

	goal3JSON, ok := cache.GetGoalJSON("goal3")
	assert.True(t, ok, "Goal3 should be in cache")
	assert.NotEmpty(t, goal3JSON)
}

func TestWarmUp_EmptyChallenges(t *testing.T) {
	cache := NewSerializedChallengeCache()

	err := cache.WarmUp([]*pb.Challenge{})

	require.NoError(t, err)
	challengeCount, goalCount, _ := cache.GetStats()
	assert.Equal(t, 0, challengeCount)
	assert.Equal(t, 0, goalCount)
}

func TestWarmUp_NilChallenges(t *testing.T) {
	cache := NewSerializedChallengeCache()

	// Should skip nil challenges gracefully
	challenges := []*pb.Challenge{
		nil,
		{
			ChallengeId: "challenge1",
			Name:        "Valid Challenge",
			Goals: []*pb.Goal{
				{
					GoalId: "goal1",
					Name:   "Valid Goal",
				},
			},
		},
		nil,
	}

	err := cache.WarmUp(challenges)

	require.NoError(t, err)

	// Only the valid challenge should be cached
	_, ok := cache.GetChallengeJSON("challenge1")
	assert.True(t, ok, "Valid challenge should be cached")

	challengeCount, goalCount, _ := cache.GetStats()
	assert.Equal(t, 1, challengeCount)
	assert.Equal(t, 1, goalCount)
}

func TestWarmUp_NilGoals(t *testing.T) {
	cache := NewSerializedChallengeCache()

	challenges := []*pb.Challenge{
		{
			ChallengeId: "challenge1",
			Name:        "Challenge with nil goal",
			Goals: []*pb.Goal{
				nil,
				{
					GoalId: "goal1",
					Name:   "Valid Goal",
				},
				nil,
			},
		},
	}

	err := cache.WarmUp(challenges)

	require.NoError(t, err)

	// Challenge should be cached
	_, ok := cache.GetChallengeJSON("challenge1")
	assert.True(t, ok)

	// Only the valid goal should be cached
	_, ok = cache.GetGoalJSON("goal1")
	assert.True(t, ok)

	challengeCount, goalCount, _ := cache.GetStats()
	assert.Equal(t, 1, challengeCount)
	assert.Equal(t, 1, goalCount)
}

func TestWarmUp_JSONFormat(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()

	err := cache.WarmUp(challenges)
	require.NoError(t, err)

	// Verify goal JSON has correct structure
	goalJSON, ok := cache.GetGoalJSON("goal1")
	require.True(t, ok)

	var goal map[string]interface{}
	err = json.Unmarshal(goalJSON, &goal)
	require.NoError(t, err, "Goal JSON should be valid")

	// Verify camelCase field names (UseProtoNames: false)
	assert.Equal(t, "goal1", goal["goalId"])
	assert.Equal(t, "Test Goal 1", goal["name"])
	assert.Equal(t, "First test goal", goal["description"])

	// Verify default progress values
	// Note: EmitUnpopulated: false means zero values won't be in JSON
	// They'll either be omitted or appear as nil when unmarshaled
	// All these fields have zero/empty default values, so they may be omitted
	if goal["progress"] != nil {
		assert.Equal(t, float64(0), goal["progress"])
	}
	if goal["status"] != nil {
		assert.Equal(t, "", goal["status"])
	}
	if goal["locked"] != nil {
		assert.Equal(t, false, goal["locked"])
	}
	if goal["completedAt"] != nil { // camelCase
		assert.Equal(t, "", goal["completedAt"])
	}
	if goal["claimedAt"] != nil { // camelCase
		assert.Equal(t, "", goal["claimedAt"])
	}
}

func TestGetGoalJSON_Found(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	goalJSON, ok := cache.GetGoalJSON("goal1")

	assert.True(t, ok)
	assert.NotEmpty(t, goalJSON)

	// Verify it's valid JSON
	var goal map[string]interface{}
	err := json.Unmarshal(goalJSON, &goal)
	require.NoError(t, err)
	assert.Equal(t, "goal1", goal["goalId"]) // camelCase
}

func TestGetGoalJSON_NotFound(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	goalJSON, ok := cache.GetGoalJSON("nonexistent")

	assert.False(t, ok)
	assert.Nil(t, goalJSON)
}

func TestGetGoalJSON_EmptyCache(t *testing.T) {
	cache := NewSerializedChallengeCache()

	goalJSON, ok := cache.GetGoalJSON("goal1")

	assert.False(t, ok)
	assert.Nil(t, goalJSON)
}

func TestGetChallengeJSON_Found(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	challengeJSON, ok := cache.GetChallengeJSON("challenge1")

	assert.True(t, ok)
	assert.NotEmpty(t, challengeJSON)

	// Verify it's valid JSON
	var challenge map[string]interface{}
	err := json.Unmarshal(challengeJSON, &challenge)
	require.NoError(t, err)
	assert.Equal(t, "challenge1", challenge["challengeId"]) // camelCase
	assert.Equal(t, "Test Challenge 1", challenge["name"])
}

func TestGetChallengeJSON_NotFound(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	challengeJSON, ok := cache.GetChallengeJSON("nonexistent")

	assert.False(t, ok)
	assert.Nil(t, challengeJSON)
}

func TestGetChallengeJSON_EmptyCache(t *testing.T) {
	cache := NewSerializedChallengeCache()

	challengeJSON, ok := cache.GetChallengeJSON("challenge1")

	assert.False(t, ok)
	assert.Nil(t, challengeJSON)
}

func TestRefresh_Success(t *testing.T) {
	cache := NewSerializedChallengeCache()
	initialChallenges := createTestChallenges()
	cache.WarmUp(initialChallenges)

	// Verify initial state
	_, ok := cache.GetChallengeJSON("challenge1")
	assert.True(t, ok)

	// Create new challenges for refresh
	newChallenges := []*pb.Challenge{
		{
			ChallengeId: "challenge3",
			Name:        "New Challenge",
			Goals: []*pb.Goal{
				{
					GoalId: "goal4",
					Name:   "New Goal",
				},
			},
		},
	}

	err := cache.Refresh(newChallenges)
	require.NoError(t, err)

	// Old challenges should be gone
	_, ok = cache.GetChallengeJSON("challenge1")
	assert.False(t, ok, "Old challenge should be removed after refresh")

	_, ok = cache.GetChallengeJSON("challenge2")
	assert.False(t, ok, "Old challenge should be removed after refresh")

	// Old goals should be gone
	_, ok = cache.GetGoalJSON("goal1")
	assert.False(t, ok, "Old goal should be removed after refresh")

	// New challenges should exist
	_, ok = cache.GetChallengeJSON("challenge3")
	assert.True(t, ok, "New challenge should exist after refresh")

	_, ok = cache.GetGoalJSON("goal4")
	assert.True(t, ok, "New goal should exist after refresh")
}

func TestRefresh_EmptyChallenges(t *testing.T) {
	cache := NewSerializedChallengeCache()
	cache.WarmUp(createTestChallenges())

	// Verify initial state has challenges
	challengeCount, goalCount, _ := cache.GetStats()
	assert.Greater(t, challengeCount, 0)
	assert.Greater(t, goalCount, 0)

	// Refresh with empty list
	err := cache.Refresh([]*pb.Challenge{})
	require.NoError(t, err)

	// Cache should be empty
	challengeCount, goalCount, _ = cache.GetStats()
	assert.Equal(t, 0, challengeCount)
	assert.Equal(t, 0, goalCount)
}

func TestRefresh_NilChallenges(t *testing.T) {
	cache := NewSerializedChallengeCache()
	cache.WarmUp(createTestChallenges())

	challenges := []*pb.Challenge{
		nil,
		{
			ChallengeId: "challenge3",
			Name:        "Valid Challenge",
			Goals: []*pb.Goal{
				{
					GoalId: "goal4",
					Name:   "Valid Goal",
				},
			},
		},
	}

	err := cache.Refresh(challenges)
	require.NoError(t, err)

	// Only valid challenge should exist
	_, ok := cache.GetChallengeJSON("challenge3")
	assert.True(t, ok)

	challengeCount, goalCount, _ := cache.GetStats()
	assert.Equal(t, 1, challengeCount)
	assert.Equal(t, 1, goalCount)
}

func TestGetStats_EmptyCache(t *testing.T) {
	cache := NewSerializedChallengeCache()

	challengeCount, goalCount, totalBytes := cache.GetStats()

	assert.Equal(t, 0, challengeCount)
	assert.Equal(t, 0, goalCount)
	assert.Equal(t, 0, totalBytes)
}

func TestGetStats_PopulatedCache(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	challengeCount, goalCount, totalBytes := cache.GetStats()

	assert.Equal(t, 2, challengeCount)
	assert.Equal(t, 3, goalCount)
	assert.Greater(t, totalBytes, 0, "Total bytes should be > 0 for populated cache")
	assert.Greater(t, totalBytes, 100, "JSON should be at least 100 bytes")
}

func TestGetStats_AfterRefresh(t *testing.T) {
	cache := NewSerializedChallengeCache()
	cache.WarmUp(createTestChallenges())

	// Get initial stats
	initialChallenges, initialGoals, initialBytes := cache.GetStats()
	assert.Equal(t, 2, initialChallenges)
	assert.Equal(t, 3, initialGoals)

	// Refresh with smaller dataset
	newChallenges := []*pb.Challenge{
		{
			ChallengeId: "challenge3",
			Name:        "New Challenge",
			Goals: []*pb.Goal{
				{
					GoalId: "goal4",
					Name:   "New Goal",
				},
			},
		},
	}
	cache.Refresh(newChallenges)

	// Stats should reflect new data
	challengeCount, goalCount, totalBytes := cache.GetStats()
	assert.Equal(t, 1, challengeCount)
	assert.Equal(t, 1, goalCount)
	assert.Less(t, totalBytes, initialBytes, "New cache should be smaller")
}

func TestParseAndMerge_ValidJSON(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	challengeJSON, ok := cache.GetChallengeJSON("challenge1")
	require.True(t, ok)

	userProgress := map[string]interface{}{
		"goal1": map[string]interface{}{
			"progress": 5,
			"status":   "in_progress",
		},
	}

	parsed, err := cache.ParseAndMerge(challengeJSON, userProgress)

	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, "challenge1", parsed["challengeId"]) // camelCase
	assert.Equal(t, "Test Challenge 1", parsed["name"])
}

func TestParseAndMerge_InvalidJSON(t *testing.T) {
	cache := NewSerializedChallengeCache()

	invalidJSON := []byte("{invalid json")
	userProgress := map[string]interface{}{}

	parsed, err := cache.ParseAndMerge(invalidJSON, userProgress)

	assert.Error(t, err)
	assert.Nil(t, parsed)
	assert.Contains(t, err.Error(), "failed to parse cached challenge JSON")
}

func TestParseAndMerge_EmptyJSON(t *testing.T) {
	cache := NewSerializedChallengeCache()

	emptyJSON := []byte("{}")
	userProgress := map[string]interface{}{}

	parsed, err := cache.ParseAndMerge(emptyJSON, userProgress)

	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, 0, len(parsed))
}

func TestParseAndMerge_NilUserProgress(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	challengeJSON, ok := cache.GetChallengeJSON("challenge1")
	require.True(t, ok)

	// Should work with nil userProgress
	parsed, err := cache.ParseAndMerge(challengeJSON, nil)

	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, "challenge1", parsed["challengeId"]) // camelCase
}

// TestConcurrentAccess tests thread-safety of the cache
func TestConcurrentAccess(t *testing.T) {
	cache := NewSerializedChallengeCache()
	challenges := createTestChallenges()
	cache.WarmUp(challenges)

	// Simulate concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = cache.GetChallengeJSON("challenge1")
				_, _ = cache.GetGoalJSON("goal1")
				_, _, _ = cache.GetStats()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Cache should still be consistent
	challengeCount, goalCount, _ := cache.GetStats()
	assert.Equal(t, 2, challengeCount)
	assert.Equal(t, 3, goalCount)
}

// TestConcurrentRefresh tests thread-safety during refresh operations
func TestConcurrentRefresh(t *testing.T) {
	cache := NewSerializedChallengeCache()
	cache.WarmUp(createTestChallenges())

	// Simulate concurrent reads and refresh
	done := make(chan bool)

	// Readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				_, _ = cache.GetChallengeJSON("challenge1")
				_, _ = cache.GetGoalJSON("goal1")
			}
			done <- true
		}()
	}

	// Writer (refresh)
	go func() {
		newChallenges := []*pb.Challenge{
			{
				ChallengeId: "challenge3",
				Name:        "New Challenge",
				Goals: []*pb.Goal{
					{
						GoalId: "goal4",
						Name:   "New Goal",
					},
				},
			},
		}
		_ = cache.Refresh(newChallenges)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 6; i++ {
		<-done
	}

	// After refresh, only new data should exist
	_, ok := cache.GetChallengeJSON("challenge3")
	assert.True(t, ok)
}
