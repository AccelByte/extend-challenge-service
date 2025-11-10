package service

import (
	"context"
	"fmt"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/sirupsen/logrus"
)

// InitializeResponse represents the result of player initialization.
type InitializeResponse struct {
	AssignedGoals  []*AssignedGoal
	NewAssignments int
	TotalActive    int
}

// AssignedGoal represents a goal assigned to a player during initialization.
type AssignedGoal struct {
	ChallengeID string
	GoalID      string
	Name        string
	Description string
	IsActive    bool
	AssignedAt  *time.Time
	ExpiresAt   *time.Time
	Progress    int
	Target      int
	Status      string
	Type        domain.GoalType
	EventSource domain.EventSource
	Requirement domain.Requirement
	Reward      domain.Reward
}

// InitializePlayer creates database rows for ALL goals on first login or config sync on subsequent logins.
//
// This function implements the initialization logic from TECH_SPEC_M3.md (Phase 2, lines 573-671).
//
// M3 Phase 6 Critical Behavior:
//   - Creates rows for ALL goals (not just default_assigned = true)
//   - Sets is_active = true for default_assigned = true goals
//   - Sets is_active = false for default_assigned = false goals
//   - This ensures all goals have rows before events arrive, allowing
//     the WHERE clause (is_active = true) in UPSERT to protect inactive goals
//
// Flow:
// 1. Get ALL goals from config cache
// 2. Check which goals player already has (SELECT query)
// 3. Find missing goals (set difference)
// 4. Fast path: if no missing goals, return existing assignments
// 5. Bulk insert missing goals with is_active based on default_assigned, assigned_at=NOW()
// 6. Return all assigned goals (existing + new)
//
// Performance:
// - First login (20 total goals): 1 SELECT + 1 INSERT, ~15ms
// - Subsequent login (already initialized): 1 SELECT, 0 INSERT, ~1-2ms
// - Config updated (2 new goals): 1 SELECT + 1 INSERT, ~3ms
//
// Parameters:
// - ctx: Context for cancellation and timeout
// - userID: User ID from JWT claims (never trust request body)
// - namespace: Namespace from JWT claims
// - goalCache: In-memory goal cache for config lookup
// - repo: Database repository for goal progress
//
// Returns:
// - *InitializeResponse: Contains assigned goals, new assignments count, total active count
// - error: Database error or validation error
func InitializePlayer(
	ctx context.Context,
	userID string,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
) (*InitializeResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	if goalCache == nil {
		return nil, fmt.Errorf("goal cache cannot be nil")
	}

	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// M3: Get ALL goals (both default_assigned = true and false)
	// We create rows for ALL goals during initialization:
	// - is_active = true for default_assigned = true goals
	// - is_active = false for default_assigned = false goals
	// This ensures all goals have rows before events arrive, allowing
	// the WHERE clause (is_active = true) in UPSERT to protect inactive goals
	allGoals := goalCache.GetAllGoals()

	// Early return if no goals configured
	if len(allGoals) == 0 {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": namespace,
		}).Info("No goals configured, initialization skipped")

		return &InitializeResponse{
			AssignedGoals:  []*AssignedGoal{},
			NewAssignments: 0,
			TotalActive:    0,
		}, nil
	}

	// 2. Extract goal IDs for query
	allGoalIDs := make([]string, len(allGoals))
	for i, goal := range allGoals {
		allGoalIDs[i] = goal.ID
	}

	// 3. Check which goals player already has
	existing, err := repo.GetGoalsByIDs(ctx, userID, allGoalIDs)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": namespace,
			"error":     err,
		}).Error("Failed to get existing goals")
		return nil, fmt.Errorf("failed to get existing goals: %w", err)
	}

	// 4. Find missing goals (set difference)
	existingMap := make(map[string]bool)
	for _, progress := range existing {
		existingMap[progress.GoalID] = true
	}

	var missing []*domain.Goal
	for _, goal := range allGoals {
		if !existingMap[goal.ID] {
			missing = append(missing, goal)
		}
	}

	// 5. Fast path: nothing to insert
	if len(missing) == 0 {
		// M3: Count only active goals
		activeCount := 0
		for _, progress := range existing {
			if progress.IsActive {
				activeCount++
			}
		}

		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"namespace":    namespace,
			"total_goals":  len(existing),
			"total_active": activeCount,
		}).Info("Player already initialized, no new assignments")

		return &InitializeResponse{
			AssignedGoals:  mapToAssignedGoals(existing, allGoals, goalCache),
			NewAssignments: 0,
			TotalActive:    activeCount,
		}, nil
	}

	// 6. Bulk insert missing goals
	newAssignments := make([]*domain.UserGoalProgress, len(missing))
	now := time.Now()

	for i, goal := range missing {
		// M3: Always return nil for expires_at (permanent assignment)
		// M5: Will calculate based on rotation config
		var expiresAt *time.Time = nil

		// M3: Set is_active based on default_assigned from config
		// - true for default_assigned = true goals (immediately active)
		// - false for default_assigned = false goals (inactive until user activates)
		newAssignments[i] = &domain.UserGoalProgress{
			UserID:      userID,
			GoalID:      goal.ID,
			ChallengeID: goal.ChallengeID,
			Namespace:   namespace,
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    goal.DefaultAssigned, // M3: Set based on config
			AssignedAt:  &now,
			ExpiresAt:   expiresAt,
		}
	}

	err = repo.BulkInsert(ctx, newAssignments)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":       userID,
			"namespace":     namespace,
			"missing_count": len(missing),
			"error":         err,
		}).Error("Failed to bulk insert goals")
		return nil, fmt.Errorf("failed to bulk insert goals: %w", err)
	}

	// M3: Count newly assigned active goals
	newActiveCount := 0
	for _, assignment := range newAssignments {
		if assignment.IsActive {
			newActiveCount++
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id":         userID,
		"namespace":       namespace,
		"new_assignments": len(missing),
		"new_active":      newActiveCount,
	}).Info("Successfully initialized player with goals")

	// 7. Fetch all assigned goals (existing + new)
	allAssigned, err := repo.GetGoalsByIDs(ctx, userID, allGoalIDs)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":   userID,
			"namespace": namespace,
			"error":     err,
		}).Error("Failed to fetch assigned goals after insertion")
		return nil, fmt.Errorf("failed to fetch assigned goals: %w", err)
	}

	// M3: Count total active goals
	totalActive := 0
	for _, progress := range allAssigned {
		if progress.IsActive {
			totalActive++
		}
	}

	return &InitializeResponse{
		AssignedGoals:  mapToAssignedGoals(allAssigned, allGoals, goalCache),
		NewAssignments: len(missing),
		TotalActive:    totalActive,
	}, nil
}

// mapToAssignedGoals converts UserGoalProgress and Goal domain models to AssignedGoal response models.
//
// This function enriches progress data with goal configuration details for the API response.
//
// Parameters:
// - progresses: User goal progress from database
// - goals: Goal configurations from cache (for enrichment)
// - goalCache: Goal cache for looking up goal details
//
// Returns:
// - []*AssignedGoal: Array of assigned goals with complete information
func mapToAssignedGoals(
	progresses []*domain.UserGoalProgress,
	goals []*domain.Goal,
	goalCache cache.GoalCache,
) []*AssignedGoal {
	result := make([]*AssignedGoal, 0, len(progresses))

	for _, progress := range progresses {
		goal := goalCache.GetGoalByID(progress.GoalID)
		if goal == nil {
			// Skip goals that are no longer in config (defensive)
			logrus.WithField("goal_id", progress.GoalID).Warn("Goal not found in cache during mapping")
			continue
		}

		result = append(result, &AssignedGoal{
			ChallengeID: progress.ChallengeID,
			GoalID:      progress.GoalID,
			Name:        goal.Name,
			Description: goal.Description,
			IsActive:    progress.IsActive,
			AssignedAt:  progress.AssignedAt,
			ExpiresAt:   progress.ExpiresAt,
			Progress:    progress.Progress,
			Target:      goal.Requirement.TargetValue,
			Status:      string(progress.Status),
			Type:        goal.Type,
			EventSource: goal.EventSource,
			Requirement: goal.Requirement,
			Reward:      goal.Reward,
		})
	}

	return result
}
