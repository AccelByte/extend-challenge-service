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

// SetGoalActiveResponse represents the result of setting a goal active/inactive.
type SetGoalActiveResponse struct {
	ChallengeID string
	GoalID      string
	IsActive    bool
	AssignedAt  *time.Time
	Message     string
}

// SetGoalActive allows players to manually control goal assignment.
//
// This function implements the manual activation logic from TECH_SPEC_M3.md (Phase 3, lines 701-736).
//
// Flow:
// 1. Validate goal exists in config
// 2. UPSERT goal progress with is_active status
// 3. Update assigned_at only when activating (not when deactivating)
//
// Parameters:
//   - ctx: Request context for cancellation and timeout
//   - userID: User identifier (extracted from JWT)
//   - challengeID: Challenge identifier (from URL path)
//   - goalID: Goal identifier (from URL path)
//   - namespace: Namespace (extracted from JWT)
//   - isActive: Whether to activate (true) or deactivate (false) the goal
//   - goalCache: In-memory config cache for goal validation
//   - repo: Database repository for persisting changes
//
// Returns:
//   - SetGoalActiveResponse with updated status
//   - error if validation fails or database error occurs
//
// Error Cases:
//   - Goal not found in config: returns ErrGoalNotFound
//   - Database error: returns wrapped error
func SetGoalActive(
	ctx context.Context,
	userID string,
	challengeID string,
	goalID string,
	namespace string,
	isActive bool,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
) (*SetGoalActiveResponse, error) {
	// Early return validation
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if challengeID == "" {
		return nil, fmt.Errorf("challenge ID cannot be empty")
	}

	if goalID == "" {
		return nil, fmt.Errorf("goal ID cannot be empty")
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

	// 1. Validate goal exists in config
	goal := goalCache.GetGoalByID(goalID)
	if goal == nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"goal_id":      goalID,
			"namespace":    namespace,
		}).Warn("Goal not found in config")
		return nil, fmt.Errorf("goal '%s' not found in challenge '%s'", goalID, challengeID)
	}

	// Verify goal belongs to the specified challenge
	if goal.ChallengeID != challengeID {
		logrus.WithFields(logrus.Fields{
			"user_id":             userID,
			"requested_challenge": challengeID,
			"actual_challenge":    goal.ChallengeID,
			"goal_id":             goalID,
			"namespace":           namespace,
		}).Warn("Goal does not belong to specified challenge")
		return nil, fmt.Errorf("goal '%s' does not belong to challenge '%s'", goalID, challengeID)
	}

	// 2. UPSERT goal progress
	now := time.Now().UTC() // Always use UTC for consistency across timezones
	progress := &domain.UserGoalProgress{
		UserID:      userID,
		GoalID:      goalID,
		ChallengeID: challengeID,
		Namespace:   namespace,
		Progress:    0,
		Status:      domain.GoalStatusNotStarted,
		IsActive:    isActive,
		AssignedAt:  &now,
	}

	err := repo.UpsertGoalActive(ctx, progress)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"goal_id":      goalID,
			"namespace":    namespace,
			"is_active":    isActive,
			"error":        err,
		}).Error("Failed to update goal active status")
		return nil, fmt.Errorf("failed to update goal active status: %w", err)
	}

	var message string
	if isActive {
		message = "Goal activated successfully"
	} else {
		message = "Goal deactivated successfully"
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"challenge_id": challengeID,
		"goal_id":      goalID,
		"namespace":    namespace,
		"is_active":    isActive,
	}).Info("Successfully updated goal active status")

	return &SetGoalActiveResponse{
		ChallengeID: challengeID,
		GoalID:      goalID,
		IsActive:    isActive,
		AssignedAt:  &now,
		Message:     message,
	}, nil
}
