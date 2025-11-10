package service

import (
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"

	"github.com/stretchr/testify/assert"
)

// Test Fixtures for PrerequisiteChecker

func createGoalWithPrereqs(id string, prereqs []string) *domain.Goal {
	return &domain.Goal{
		ID:            id,
		Name:          "Test Goal",
		Description:   "Test goal with prerequisites",
		ChallengeID:   "test-challenge",
		Type:          domain.GoalTypeAbsolute,
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: prereqs,
		Requirement: domain.Requirement{
			StatCode:    "kills",
			Operator:    ">=",
			TargetValue: 10,
		},
		Reward: domain.Reward{
			Type:     string(domain.RewardTypeItem),
			RewardID: "sword",
			Quantity: 1,
		},
	}
}

func createProgressWithStatus(goalID string, status domain.GoalStatus) *domain.UserGoalProgress {
	now := time.Now().UTC()
	return &domain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      goalID,
		ChallengeID: "test-challenge",
		Namespace:   "test-namespace",
		Progress:    10,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Test CheckGoalLocked

func TestCheckGoalLocked_NoPrerequisites(t *testing.T) {
	goal := createGoalWithPrereqs("goal-1", []string{})
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.False(t, locked, "Goal with no prerequisites should not be locked")
}

func TestCheckGoalLocked_PrerequisitesCompleted(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.False(t, locked, "Goal with completed prerequisites should not be locked")
}

func TestCheckGoalLocked_PrerequisitesClaimed(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusClaimed),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.False(t, locked, "Goal with claimed prerequisites should not be locked")
}

func TestCheckGoalLocked_PrerequisitesInProgress(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusInProgress),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.True(t, locked, "Goal with in-progress prerequisites should be locked")
}

func TestCheckGoalLocked_PrerequisitesNotStarted(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusNotStarted),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.True(t, locked, "Goal with not-started prerequisites should be locked")
}

func TestCheckGoalLocked_PrerequisiteMissingProgress(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := make(map[string]*domain.UserGoalProgress) // Empty map
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.True(t, locked, "Goal with missing prerequisite progress should be locked")
}

func TestCheckGoalLocked_MultiplePrerequisitesAllCompleted(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
		"goal-2": createProgressWithStatus("goal-2", domain.GoalStatusClaimed),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.False(t, locked, "Goal with all prerequisites completed should not be locked")
}

func TestCheckGoalLocked_MultiplePrerequisitesOneMissing(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
		// goal-2 missing
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.True(t, locked, "Goal with any missing prerequisite should be locked")
}

func TestCheckGoalLocked_MultiplePrerequisitesOneInProgress(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
		"goal-2": createProgressWithStatus("goal-2", domain.GoalStatusInProgress),
	}
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(goal)

	assert.True(t, locked, "Goal with any incomplete prerequisite should be locked")
}

func TestCheckGoalLocked_NilGoal(t *testing.T) {
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	locked := checker.CheckGoalLocked(nil)

	assert.False(t, locked, "Nil goal should return false")
}

// Test GetMissingPrerequisites

func TestGetMissingPrerequisites_NoPrerequisites(t *testing.T) {
	goal := createGoalWithPrereqs("goal-1", []string{})
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)

	assert.Empty(t, missing)
}

func TestGetMissingPrerequisites_AllCompleted(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
		"goal-2": createProgressWithStatus("goal-2", domain.GoalStatusClaimed),
	}
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)

	assert.Empty(t, missing)
}

func TestGetMissingPrerequisites_OneMissing(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
	}
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)

	assert.Len(t, missing, 1)
	assert.Contains(t, missing, "goal-2")
}

func TestGetMissingPrerequisites_MultipleInProgress(t *testing.T) {
	goal := createGoalWithPrereqs("goal-4", []string{"goal-1", "goal-2", "goal-3"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusInProgress),
		"goal-2": createProgressWithStatus("goal-2", domain.GoalStatusNotStarted),
		"goal-3": createProgressWithStatus("goal-3", domain.GoalStatusCompleted),
	}
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)

	assert.Len(t, missing, 2)
	assert.Contains(t, missing, "goal-1")
	assert.Contains(t, missing, "goal-2")
	assert.NotContains(t, missing, "goal-3")
}

func TestGetMissingPrerequisites_AllMissing(t *testing.T) {
	goal := createGoalWithPrereqs("goal-3", []string{"goal-1", "goal-2"})
	progressMap := make(map[string]*domain.UserGoalProgress) // Empty map
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)

	assert.Len(t, missing, 2)
	assert.Contains(t, missing, "goal-1")
	assert.Contains(t, missing, "goal-2")
}

func TestGetMissingPrerequisites_NilGoal(t *testing.T) {
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(nil)

	assert.Empty(t, missing)
}

// Test CheckAllPrerequisitesMet

func TestCheckAllPrerequisitesMet_NoPrerequisites(t *testing.T) {
	goal := createGoalWithPrereqs("goal-1", []string{})
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	met := checker.CheckAllPrerequisitesMet(goal)

	assert.True(t, met, "Goal with no prerequisites should have all prerequisites met")
}

func TestCheckAllPrerequisitesMet_AllCompleted(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
	}
	checker := NewPrerequisiteChecker(progressMap)

	met := checker.CheckAllPrerequisitesMet(goal)

	assert.True(t, met, "Goal with completed prerequisites should have all prerequisites met")
}

func TestCheckAllPrerequisitesMet_OneMissing(t *testing.T) {
	goal := createGoalWithPrereqs("goal-2", []string{"goal-1"})
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	met := checker.CheckAllPrerequisitesMet(goal)

	assert.False(t, met, "Goal with missing prerequisites should not have all prerequisites met")
}

// Test NewPrerequisiteChecker

func TestNewPrerequisiteChecker_EmptyMap(t *testing.T) {
	progressMap := make(map[string]*domain.UserGoalProgress)
	checker := NewPrerequisiteChecker(progressMap)

	assert.NotNil(t, checker)
	assert.NotNil(t, checker.progressMap)
	assert.Empty(t, checker.progressMap)
}

func TestNewPrerequisiteChecker_WithData(t *testing.T) {
	progressMap := map[string]*domain.UserGoalProgress{
		"goal-1": createProgressWithStatus("goal-1", domain.GoalStatusCompleted),
		"goal-2": createProgressWithStatus("goal-2", domain.GoalStatusInProgress),
	}
	checker := NewPrerequisiteChecker(progressMap)

	assert.NotNil(t, checker)
	assert.Len(t, checker.progressMap, 2)
	assert.Equal(t, progressMap, checker.progressMap)
}

// Test O(1) Performance Characteristic

func TestPrerequisiteChecker_PerformanceOptimization(t *testing.T) {
	// Create large progress map to simulate real-world scenario
	progressMap := make(map[string]*domain.UserGoalProgress)
	for i := 0; i < 100; i++ {
		goalID := "goal-" + string(rune('a'+i))
		progressMap[goalID] = createProgressWithStatus(goalID, domain.GoalStatusCompleted)
	}

	// Goal with prerequisites (using 'b', 'y', 'z' which correspond to indices 1, 24, 25)
	goal := createGoalWithPrereqs("goal-final", []string{"goal-b", "goal-y", "goal-z"})

	checker := NewPrerequisiteChecker(progressMap)

	// This should be O(3) = O(1) with map, not O(100*3) = O(n²) with linear search
	locked := checker.CheckGoalLocked(goal)

	assert.False(t, locked, "All prerequisites completed")

	// Verify missing prerequisites also performs well
	delete(progressMap, "goal-y")
	checker = NewPrerequisiteChecker(progressMap)

	missing := checker.GetMissingPrerequisites(goal)
	assert.Len(t, missing, 1)
	assert.Contains(t, missing, "goal-y")
}
