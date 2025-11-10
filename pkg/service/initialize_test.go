package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// NOTE: MockGoalCache and MockGoalRepository are defined in progress_query_test.go
// to avoid duplication across test files.

// Test InitializePlayer - Happy Path: First Login (creates ALL goals, both active and inactive)
// M3: This test verifies that initialization creates rows for ALL goals,
// with is_active set based on default_assigned config field
func TestInitializePlayer_FirstLogin(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	// M3: Create mock goals with mix of default_assigned true/false
	allGoals := []*domain.Goal{
		{
			ID:              "goal1",
			ChallengeID:     "challenge1",
			Name:            "First Login",
			Description:     "Login for the first time",
			DefaultAssigned: true, // Will be is_active = true
			Type:            domain.GoalTypeAbsolute,
			EventSource:     domain.EventSourceLogin,
			Requirement: domain.Requirement{
				StatCode:    "login_count",
				Operator:    ">=",
				TargetValue: 1,
			},
			Reward: domain.Reward{
				Type:     string(domain.RewardTypeItem),
				RewardID: "starter_pack",
				Quantity: 1,
			},
		},
		{
			ID:              "goal2",
			ChallengeID:     "challenge1",
			Name:            "Complete Tutorial",
			Description:     "Finish the tutorial",
			DefaultAssigned: true, // Will be is_active = true
			Type:            domain.GoalTypeAbsolute,
			EventSource:     domain.EventSourceStatistic,
			Requirement: domain.Requirement{
				StatCode:    "tutorial_complete",
				Operator:    ">=",
				TargetValue: 1,
			},
			Reward: domain.Reward{
				Type:     string(domain.RewardTypeWallet),
				RewardID: "GEMS",
				Quantity: 50,
			},
		},
		{
			ID:              "goal3",
			ChallengeID:     "challenge1",
			Name:            "Advanced Goal",
			Description:     "An advanced goal",
			DefaultAssigned: false, // Will be is_active = false
			Type:            domain.GoalTypeAbsolute,
			EventSource:     domain.EventSourceStatistic,
			Requirement: domain.Requirement{
				StatCode:    "advanced_stat",
				Operator:    ">=",
				TargetValue: 100,
			},
			Reward: domain.Reward{
				Type:     string(domain.RewardTypeItem),
				RewardID: "rare_item",
				Quantity: 1,
			},
		},
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// M3: Use GetAllGoals() instead of GetGoalsWithDefaultAssigned()
	mockCache.On("GetAllGoals").Return(allGoals)

	// First call - no existing goals
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1", "goal2", "goal3"}).Return([]*domain.UserGoalProgress{}, nil).Once()

	// Expect bulk insert call for ALL 3 goals (2 active, 1 inactive)
	mockRepo.On("BulkInsert", ctx, mock.MatchedBy(func(progresses []*domain.UserGoalProgress) bool {
		if len(progresses) != 3 {
			return false
		}
		// Validate first progress (active)
		p1 := progresses[0]
		assert.Equal(t, userID, p1.UserID)
		assert.Equal(t, "goal1", p1.GoalID)
		assert.Equal(t, "challenge1", p1.ChallengeID)
		assert.Equal(t, namespace, p1.Namespace)
		assert.Equal(t, 0, p1.Progress)
		assert.Equal(t, domain.GoalStatusNotStarted, p1.Status)
		assert.True(t, p1.IsActive) // default_assigned = true
		assert.NotNil(t, p1.AssignedAt)
		assert.Nil(t, p1.ExpiresAt) // M3: NULL for permanent assignment

		// Validate second progress (active)
		p2 := progresses[1]
		assert.Equal(t, userID, p2.UserID)
		assert.Equal(t, "goal2", p2.GoalID)
		assert.True(t, p2.IsActive) // default_assigned = true

		// M3: Validate third progress (INACTIVE)
		p3 := progresses[2]
		assert.Equal(t, userID, p3.UserID)
		assert.Equal(t, "goal3", p3.GoalID)
		assert.False(t, p3.IsActive) // default_assigned = false

		return true
	})).Return(nil)

	// After insert, return the new goals
	now := time.Now()
	newProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   namespace,
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   nil,
		},
		{
			UserID:      userID,
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Namespace:   namespace,
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   nil,
		},
		{
			UserID:      userID,
			GoalID:      "goal3",
			ChallengeID: "challenge1",
			Namespace:   namespace,
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    false, // M3: Inactive goal
			AssignedAt:  &now,
			ExpiresAt:   nil,
		},
	}
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1", "goal2", "goal3"}).Return(newProgress, nil).Once()

	// Mock GetGoalByID for mapToAssignedGoals
	mockCache.On("GetGoalByID", "goal1").Return(allGoals[0])
	mockCache.On("GetGoalByID", "goal2").Return(allGoals[1])
	mockCache.On("GetGoalByID", "goal3").Return(allGoals[2])

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.NewAssignments)    // 3 new goal rows created
	assert.Equal(t, 2, result.TotalActive)       // Only 2 are active
	assert.Len(t, result.AssignedGoals, 3)       // All 3 goals returned

	// Validate first assigned goal (active)
	ag1 := result.AssignedGoals[0]
	assert.Equal(t, "goal1", ag1.GoalID)
	assert.Equal(t, "challenge1", ag1.ChallengeID)
	assert.Equal(t, "First Login", ag1.Name)
	assert.True(t, ag1.IsActive)
	assert.NotNil(t, ag1.AssignedAt)
	assert.Nil(t, ag1.ExpiresAt)
	assert.Equal(t, 0, ag1.Progress)
	assert.Equal(t, 1, ag1.Target)
	assert.Equal(t, "not_started", ag1.Status)

	// M3: Validate third assigned goal (INACTIVE)
	ag3 := result.AssignedGoals[2]
	assert.Equal(t, "goal3", ag3.GoalID)
	assert.False(t, ag3.IsActive) // Inactive goal still returned in response

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - Happy Path: Subsequent Login (fast path)
func TestInitializePlayer_SubsequentLogin_FastPath(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	allGoals := []*domain.Goal{
		{
			ID:              "goal1",
			ChallengeID:     "challenge1",
			Name:            "First Login",
			DefaultAssigned: true,
			Type:            domain.GoalTypeAbsolute,
			EventSource:     domain.EventSourceLogin,
			Requirement: domain.Requirement{
				StatCode:    "login_count",
				Operator:    ">=",
				TargetValue: 1,
			},
			Reward: domain.Reward{
				Type:     string(domain.RewardTypeItem),
				RewardID: "starter_pack",
				Quantity: 1,
			},
		},
	}

	now := time.Now()
	existingProgress := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   namespace,
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   nil,
		},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllGoals").Return(allGoals)
	mockCache.On("GetGoalByID", "goal1").Return(allGoals[0])

	// User already has all goals - fast path!
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1"}).Return(existingProgress, nil)

	// BulkInsert should NOT be called (fast path)

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.NewAssignments) // No new assignments
	assert.Equal(t, 1, result.TotalActive)
	assert.Len(t, result.AssignedGoals, 1)

	// Validate existing progress preserved
	ag := result.AssignedGoals[0]
	assert.Equal(t, "goal1", ag.GoalID)
	assert.Equal(t, 5, ag.Progress) // Existing progress preserved
	assert.Equal(t, "in_progress", ag.Status)
	assert.True(t, ag.IsActive)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	// Verify BulkInsert was NOT called
	mockRepo.AssertNotCalled(t, "BulkInsert")
}

// Test InitializePlayer - Config Updated (2 new goals added)
func TestInitializePlayer_ConfigUpdated_NewGoalsAdded(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	// Config now has 3 goals (user only has 1)
	allGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Old Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
		{ID: "goal2", ChallengeID: "challenge1", Name: "New Goal 2", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceStatistic, Requirement: domain.Requirement{StatCode: "stat1", Operator: ">=", TargetValue: 5}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item2", Quantity: 1}},
		{ID: "goal3", ChallengeID: "challenge2", Name: "New Goal 3", DefaultAssigned: false, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceStatistic, Requirement: domain.Requirement{StatCode: "stat2", Operator: ">=", TargetValue: 10}, Reward: domain.Reward{Type: string(domain.RewardTypeWallet), RewardID: "GEMS", Quantity: 100}},
	}

	now := time.Now()
	existingProgress := []*domain.UserGoalProgress{
		{UserID: userID, GoalID: "goal1", ChallengeID: "challenge1", Namespace: namespace, Progress: 1, Status: domain.GoalStatusCompleted, IsActive: true, AssignedAt: &now, ExpiresAt: nil},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllGoals").Return(allGoals)
	mockCache.On("GetGoalByID", "goal1").Return(allGoals[0])
	mockCache.On("GetGoalByID", "goal2").Return(allGoals[1])
	mockCache.On("GetGoalByID", "goal3").Return(allGoals[2])

	// First query - user has only goal1
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1", "goal2", "goal3"}).Return(existingProgress, nil).Once()

	// Expect bulk insert for 2 missing goals (1 active, 1 inactive)
	mockRepo.On("BulkInsert", ctx, mock.MatchedBy(func(progresses []*domain.UserGoalProgress) bool {
		if len(progresses) != 2 {
			return false
		}
		// Verify goal2 is active, goal3 is inactive
		assert.Equal(t, "goal2", progresses[0].GoalID)
		assert.True(t, progresses[0].IsActive)
		assert.Equal(t, "goal3", progresses[1].GoalID)
		assert.False(t, progresses[1].IsActive)
		return true
	})).Return(nil)

	// After insert, return all 3 goals
	allProgress := []*domain.UserGoalProgress{
		existingProgress[0],
		{UserID: userID, GoalID: "goal2", ChallengeID: "challenge1", Namespace: namespace, Progress: 0, Status: domain.GoalStatusNotStarted, IsActive: true, AssignedAt: &now, ExpiresAt: nil},
		{UserID: userID, GoalID: "goal3", ChallengeID: "challenge2", Namespace: namespace, Progress: 0, Status: domain.GoalStatusNotStarted, IsActive: false, AssignedAt: &now, ExpiresAt: nil},
	}
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1", "goal2", "goal3"}).Return(allProgress, nil).Once()

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.NewAssignments) // 2 new goals added
	assert.Equal(t, 2, result.TotalActive)    // Only goal1 and goal2 are active
	assert.Len(t, result.AssignedGoals, 3)    // All 3 goals returned

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - No Goals Configured
func TestInitializePlayer_NoDefaultGoals(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// No goals configured
	mockCache.On("GetAllGoals").Return([]*domain.Goal{})

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.NewAssignments)
	assert.Equal(t, 0, result.TotalActive)
	assert.Len(t, result.AssignedGoals, 0)

	mockCache.AssertExpectations(t)
	// Repository should NOT be called at all
	mockRepo.AssertNotCalled(t, "GetGoalsByIDs")
	mockRepo.AssertNotCalled(t, "BulkInsert")
}

// Test InitializePlayer - Validation: Empty User ID
func TestInitializePlayer_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Execute
	result, err := InitializePlayer(ctx, "", namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

// Test InitializePlayer - Validation: Empty Namespace
func TestInitializePlayer_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	userID := "user123"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// Execute
	result, err := InitializePlayer(ctx, userID, "", mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

// Test InitializePlayer - Validation: Nil Goal Cache
func TestInitializePlayer_NilGoalCache(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockRepo := new(MockGoalRepository)

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, nil, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "goal cache cannot be nil")
}

// Test InitializePlayer - Validation: Nil Repository
func TestInitializePlayer_NilRepository(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

// Test InitializePlayer - Database Error: GetGoalsByIDs fails
func TestInitializePlayer_GetGoalsByIDsError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	allGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllGoals").Return(allGoals)

	// Simulate database error
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1"}).Return(nil, errors.New("database connection failed"))

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get existing goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - Database Error: BulkInsert fails
func TestInitializePlayer_BulkInsertError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	allGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllGoals").Return(allGoals)

	// No existing goals
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1"}).Return([]*domain.UserGoalProgress{}, nil).Once()

	// Simulate bulk insert failure
	mockRepo.On("BulkInsert", ctx, mock.Anything).Return(errors.New("unique constraint violation"))

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to bulk insert goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - Database Error: Final GetGoalsByIDs fails
func TestInitializePlayer_FinalGetGoalsByIDsError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	allGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllGoals").Return(allGoals)

	// First query - no existing goals
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1"}).Return([]*domain.UserGoalProgress{}, nil).Once()

	// Bulk insert succeeds
	mockRepo.On("BulkInsert", ctx, mock.Anything).Return(nil)

	// Final query fails
	mockRepo.On("GetGoalsByIDs", ctx, userID, []string{"goal1"}).Return(nil, errors.New("connection lost")).Once()

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch assigned goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test mapToAssignedGoals - Goal Not Found in Cache (defensive)
func TestMapToAssignedGoals_GoalNotFoundInCache(t *testing.T) {
	now := time.Now()
	progresses := []*domain.UserGoalProgress{
		{
			UserID:      "user123",
			GoalID:      "missing_goal",
			ChallengeID: "challenge1",
			Namespace:   "test-namespace",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
		},
	}

	goals := []*domain.Goal{} // No goals in list

	mockCache := new(MockGoalCache)
	// Goal not found in cache
	mockCache.On("GetGoalByID", "missing_goal").Return(nil)

	// Execute
	result := mapToAssignedGoals(progresses, goals, mockCache)

	// Assert - should skip goals not found in cache
	assert.Len(t, result, 0)

	mockCache.AssertExpectations(t)
}

// Test mapToAssignedGoals - Complete Mapping
func TestMapToAssignedGoals_CompleteMapping(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	progresses := []*domain.UserGoalProgress{
		{
			UserID:      "user123",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test-namespace",
			Progress:    7,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   &expiresAt,
		},
	}

	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Test Goal",
		Description: "Test Description",
		Type:        domain.GoalTypeIncrement,
		EventSource: domain.EventSourceStatistic,
		Requirement: domain.Requirement{
			StatCode:    "test_stat",
			Operator:    ">=",
			TargetValue: 10,
		},
		Reward: domain.Reward{
			Type:     string(domain.RewardTypeWallet),
			RewardID: "GEMS",
			Quantity: 100,
		},
	}

	mockCache := new(MockGoalCache)
	mockCache.On("GetGoalByID", "goal1").Return(goal)

	// Execute
	result := mapToAssignedGoals(progresses, []*domain.Goal{goal}, mockCache)

	// Assert
	require.Len(t, result, 1)
	ag := result[0]

	assert.Equal(t, "goal1", ag.GoalID)
	assert.Equal(t, "challenge1", ag.ChallengeID)
	assert.Equal(t, "Test Goal", ag.Name)
	assert.Equal(t, "Test Description", ag.Description)
	assert.True(t, ag.IsActive)
	assert.Equal(t, &now, ag.AssignedAt)
	assert.Equal(t, &expiresAt, ag.ExpiresAt)
	assert.Equal(t, 7, ag.Progress)
	assert.Equal(t, 10, ag.Target)
	assert.Equal(t, "in_progress", ag.Status)
	assert.Equal(t, domain.GoalTypeIncrement, ag.Type)
	assert.Equal(t, domain.EventSourceStatistic, ag.EventSource)

	// Verify requirement mapping
	assert.Equal(t, "test_stat", ag.Requirement.StatCode)
	assert.Equal(t, ">=", ag.Requirement.Operator)
	assert.Equal(t, 10, ag.Requirement.TargetValue)

	// Verify reward mapping
	assert.Equal(t, string(domain.RewardTypeWallet), ag.Reward.Type)
	assert.Equal(t, "GEMS", ag.Reward.RewardID)
	assert.Equal(t, 100, ag.Reward.Quantity)

	mockCache.AssertExpectations(t)
}
