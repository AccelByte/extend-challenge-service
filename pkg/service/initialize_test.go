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

// Test InitializePlayer - Happy Path: First Login (M3 Phase 9: Lazy Materialization)
// Phase 9: Only creates rows for DEFAULT-ASSIGNED goals (not all goals)
// Non-default goals will be created later via SetGoalActive when user activates them
func TestInitializePlayer_FirstLogin(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	// M3 Phase 9: Only default-assigned goals will be created (goal1, goal2)
	// goal3 is NOT default-assigned, so it won't be created during initialization
	defaultGoals := []*domain.Goal{
		{
			ID:              "goal1",
			ChallengeID:     "challenge1",
			Name:            "First Login",
			Description:     "Login for the first time",
			DefaultAssigned: true, // Will create row with is_active = true
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
			DefaultAssigned: true, // Will create row with is_active = true
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
	}

	// Setup mocks
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// M3 Phase 9: Use GetGoalsWithDefaultAssigned() instead of GetAllGoals()
	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)

	// M3 Phase 9: Fast path check - user has no goals yet (first login)
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(0, nil)

	// Phase 10 Optimization: No GetGoalsByIDs() call when count == 0 (we know user has no goals)
	// We skip the redundant query and directly insert all default goals

	// Expect bulk insert call for 2 default-assigned goals (both active)
	mockRepo.On("BulkInsert", ctx, mock.MatchedBy(func(progresses []*domain.UserGoalProgress) bool {
		if len(progresses) != 2 {
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

		return true
	})).Return(nil)

	// Phase 10 Optimization: No GetGoalsByIDs() after insert
	// We return the data we just created instead of re-fetching from DB

	// Mock GetGoalByID for mapToAssignedGoals
	mockCache.On("GetGoalByID", "goal1").Return(defaultGoals[0])
	mockCache.On("GetGoalByID", "goal2").Return(defaultGoals[1])

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.NewAssignments) // 2 new goal rows created (only default-assigned)
	assert.Equal(t, 2, result.TotalActive)    // Both are active
	assert.Len(t, result.AssignedGoals, 2)    // Only 2 default-assigned goals returned

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

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - Happy Path: Subsequent Login (M3 Phase 9: Fast Path)
// Phase 9: Uses GetUserGoalCount() to detect existing user, then GetActiveGoals() instead of GetGoalsByIDs
func TestInitializePlayer_SubsequentLogin_FastPath(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	defaultGoals := []*domain.Goal{
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

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockCache.On("GetGoalByID", "goal1").Return(defaultGoals[0])

	// M3 Phase 9: Fast path check - user already initialized (count > 0)
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(1, nil)

	// M3 Phase 9: Fast path - use GetActiveGoals() instead of GetGoalsByIDs()
	mockRepo.On("GetActiveGoals", ctx, userID).Return(existingProgress, nil)

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
	// Verify BulkInsert was NOT called (fast path)
	mockRepo.AssertNotCalled(t, "BulkInsert")
	// Verify GetGoalsByIDs was NOT called (fast path uses GetActiveGoals instead)
	mockRepo.AssertNotCalled(t, "GetGoalsByIDs")
}

// Test InitializePlayer - Returning User with Active Goals (M3 Phase 9: Fast Path)
// Phase 9: Fast path returns active goals only, does NOT check for new default goals
// This is the expected behavior - config updates require re-initialization logic elsewhere
func TestInitializePlayer_ReturningUser_ActiveGoals(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	// Config has 3 default-assigned goals
	defaultGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Old Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
		{ID: "goal2", ChallengeID: "challenge1", Name: "Goal 2", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceStatistic, Requirement: domain.Requirement{StatCode: "stat1", Operator: ">=", TargetValue: 5}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item2", Quantity: 1}},
		{ID: "goal3", ChallengeID: "challenge2", Name: "Goal 3", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceStatistic, Requirement: domain.Requirement{StatCode: "stat2", Operator: ">=", TargetValue: 10}, Reward: domain.Reward{Type: string(domain.RewardTypeWallet), RewardID: "GEMS", Quantity: 100}},
	}

	now := time.Now()
	// User has all 3 goals, 2 are active
	activeProgress := []*domain.UserGoalProgress{
		{UserID: userID, GoalID: "goal1", ChallengeID: "challenge1", Namespace: namespace, Progress: 1, Status: domain.GoalStatusCompleted, IsActive: true, AssignedAt: &now, ExpiresAt: nil},
		{UserID: userID, GoalID: "goal2", ChallengeID: "challenge1", Namespace: namespace, Progress: 3, Status: domain.GoalStatusInProgress, IsActive: true, AssignedAt: &now, ExpiresAt: nil},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockCache.On("GetGoalByID", "goal1").Return(defaultGoals[0])
	mockCache.On("GetGoalByID", "goal2").Return(defaultGoals[1])

	// M3 Phase 9: User already initialized (count > 0) - takes fast path
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(3, nil)

	// M3 Phase 9: Fast path - returns only active goals
	mockRepo.On("GetActiveGoals", ctx, userID).Return(activeProgress, nil)

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.NewAssignments) // No new assignments (fast path)
	assert.Equal(t, 2, result.TotalActive)    // 2 active goals
	assert.Len(t, result.AssignedGoals, 2)    // Only active goals returned

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	// Verify GetGoalsByIDs was NOT called (fast path)
	mockRepo.AssertNotCalled(t, "GetGoalsByIDs")
	mockRepo.AssertNotCalled(t, "BulkInsert")
}

// Test InitializePlayer - No Default Goals Configured (M3 Phase 9)
// Phase 9: Early return if no default-assigned goals configured
func TestInitializePlayer_NoDefaultGoals(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	// No default-assigned goals configured
	mockCache.On("GetGoalsWithDefaultAssigned").Return([]*domain.Goal{})

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.NewAssignments)
	assert.Equal(t, 0, result.TotalActive)
	assert.Len(t, result.AssignedGoals, 0)

	mockCache.AssertExpectations(t)
	// Repository should NOT be called at all (early return)
	mockRepo.AssertNotCalled(t, "GetUserGoalCount")
	mockRepo.AssertNotCalled(t, "GetActiveGoals")
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

// Test InitializePlayer - Database Error: GetUserGoalCount fails (M3 Phase 9)
func TestInitializePlayer_GetUserGoalCountError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	defaultGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)

	// Simulate database error on count check
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(0, errors.New("database connection failed"))

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get user goal count")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test InitializePlayer - Database Error: GetActiveGoals fails (M3 Phase 9)
func TestInitializePlayer_GetActiveGoalsError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	defaultGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)

	// User already initialized
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(1, nil)

	// Simulate database error on GetActiveGoals
	mockRepo.On("GetActiveGoals", ctx, userID).Return(nil, errors.New("connection lost"))

	// Execute
	result, err := InitializePlayer(ctx, userID, namespace, mockCache, mockRepo)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get active goals")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Phase 10: TestInitializePlayer_GetGoalsByIDsError removed
// Reason: We no longer call GetGoalsByIDs() when userGoalCount == 0 (optimization)
// The query was redundant since we already know the user has no goals

// Test InitializePlayer - Database Error: BulkInsert fails (M3 Phase 9)
func TestInitializePlayer_BulkInsertError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	defaultGoals := []*domain.Goal{
		{ID: "goal1", ChallengeID: "challenge1", Name: "Goal 1", DefaultAssigned: true, Type: domain.GoalTypeAbsolute, EventSource: domain.EventSourceLogin, Requirement: domain.Requirement{StatCode: "login_count", Operator: ">=", TargetValue: 1}, Reward: domain.Reward{Type: string(domain.RewardTypeItem), RewardID: "item1", Quantity: 1}},
	}

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)

	// User not initialized
	mockRepo.On("GetUserGoalCount", ctx, userID).Return(0, nil)

	// Phase 10: No GetGoalsByIDs call (optimization removed it)

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

// Phase 10: TestInitializePlayer_FinalGetGoalsByIDsError removed
// Reason: We no longer call GetGoalsByIDs() after insert (optimization)
// We return the data we just created instead of re-fetching from database

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
