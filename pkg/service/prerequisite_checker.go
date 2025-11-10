package service

import (
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// PrerequisiteChecker checks if goal prerequisites are met using efficient per-request map optimization.
// This is NOT a persistent cache - it's a function-scoped helper for O(1) lookups.
//
// Performance improvement (Decision Q7, FQ4):
// - With map: 100 goals * 2 prereqs = 100 map lookups = O(100)
// - Without map: 100 goals * 2 prereqs * 50 progress records = 10,000 linear search comparisons = O(n²)
// - Result: 100x performance improvement for typical workloads
type PrerequisiteChecker struct {
	// progressMap provides O(1) lookup of user progress by goal ID
	// Built from userProgress array once per request
	progressMap map[string]*domain.UserGoalProgress
}

// NewPrerequisiteChecker creates a new prerequisite checker with per-request map optimization.
// The map is built once from the userProgress array and discarded after the request completes.
func NewPrerequisiteChecker(userProgress map[string]*domain.UserGoalProgress) *PrerequisiteChecker {
	return &PrerequisiteChecker{
		progressMap: userProgress,
	}
}

// CheckGoalLocked determines if a goal is locked based on its prerequisites.
// A goal is locked if:
// 1. It has prerequisites AND
// 2. ANY of the prerequisites are not completed (status != 'completed' and status != 'claimed')
//
// Performance: O(p) where p is number of prerequisites (typically 1-3)
// Each prerequisite lookup is O(1) via map
func (pc *PrerequisiteChecker) CheckGoalLocked(goal *domain.Goal) bool {
	if goal == nil {
		return false
	}

	// No prerequisites = not locked
	if len(goal.Prerequisites) == 0 {
		return false
	}

	// Check if all prerequisites are completed
	for _, prereqGoalID := range goal.Prerequisites {
		prereqProgress := pc.progressMap[prereqGoalID]

		// If prerequisite has no progress, it's not completed → goal is locked
		if prereqProgress == nil {
			return true
		}

		// If prerequisite is not completed or claimed → goal is locked
		if !prereqProgress.IsCompleted() {
			return true
		}
	}

	// All prerequisites are completed → goal is not locked
	return false
}

// GetMissingPrerequisites returns the list of prerequisite goal IDs that are not completed.
// This is used for detailed error messages in the claim flow.
//
// Performance: O(p) where p is number of prerequisites
func (pc *PrerequisiteChecker) GetMissingPrerequisites(goal *domain.Goal) []string {
	if goal == nil || len(goal.Prerequisites) == 0 {
		return []string{}
	}

	missing := make([]string, 0, len(goal.Prerequisites))

	for _, prereqGoalID := range goal.Prerequisites {
		prereqProgress := pc.progressMap[prereqGoalID]

		// Missing progress or not completed
		if prereqProgress == nil || !prereqProgress.IsCompleted() {
			missing = append(missing, prereqGoalID)
		}
	}

	return missing
}

// CheckAllPrerequisitesMet returns true if all prerequisites are completed.
// This is the inverse of CheckGoalLocked and is used for validation in the claim flow.
//
// Performance: O(p) where p is number of prerequisites
func (pc *PrerequisiteChecker) CheckAllPrerequisitesMet(goal *domain.Goal) bool {
	return !pc.CheckGoalLocked(goal)
}
