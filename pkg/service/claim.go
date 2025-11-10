package service

import (
	"context"
	"fmt"
	"time"

	"extend-challenge-service/pkg/mapper"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/client"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/sirupsen/logrus"
)

// ClaimResult represents the result of a successful claim operation.
type ClaimResult struct {
	GoalID      string
	Status      string
	Reward      domain.Reward
	ClaimedAt   time.Time
	UserID      string
	ChallengeID string
}

// ClaimGoalReward handles the reward claim flow with transaction and row-level locking.
// This is the main entry point for the claim RPC handler.
//
// Flow (Decision Q3):
// 1. Start transaction with 10s timeout
// 2. Lock user progress row (SELECT ... FOR UPDATE)
// 3. Validate goal is completed and not claimed
// 4. Call AGS Platform Service (inside transaction with retry)
// 5. Mark as claimed in database
// 6. Commit transaction
//
// Error Handling:
// - Returns mapper.ErrGoalNotFound if goal doesn't exist in config
// - Returns mapper.ErrGoalNotCompleted if goal not completed
// - Returns mapper.ErrGoalAlreadyClaimed if already claimed
// - Returns mapper.ErrPrerequisitesNotMet if prerequisites not met
// - Returns mapper.ErrRewardGrantFailed if AGS call fails after retries
// - Returns mapper.ErrDatabaseError for database failures
func ClaimGoalReward(
	ctx context.Context,
	userID string,
	goalID string,
	challengeID string,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
	rewardClient client.RewardClient,
) (*ClaimResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if goalID == "" {
		return nil, fmt.Errorf("goal ID cannot be empty")
	}

	if challengeID == "" {
		return nil, fmt.Errorf("challenge ID cannot be empty")
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

	if rewardClient == nil {
		return nil, fmt.Errorf("reward client cannot be nil")
	}

	// Get goal from cache
	goal := goalCache.GetGoalByID(goalID)
	if goal == nil {
		return nil, &mapper.GoalNotFoundError{
			GoalID:      goalID,
			ChallengeID: challengeID,
		}
	}

	// Verify goal belongs to the specified challenge
	if goal.ChallengeID != challengeID {
		return nil, &mapper.GoalNotFoundError{
			GoalID:      goalID,
			ChallengeID: challengeID,
		}
	}

	// Start transaction with 10s timeout (Decision Q3, FQ1)
	txCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	txRepo, err := repo.BeginTx(txCtx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"error":        err,
		}).Error("Failed to start transaction")
		return nil, mapper.ErrDatabaseError
	}

	// Ensure transaction is rolled back on error
	defer func() {
		if err != nil {
			if rbErr := txRepo.Rollback(); rbErr != nil {
				logrus.WithFields(logrus.Fields{
					"user_id":      userID,
					"goal_id":      goalID,
					"challenge_id": challengeID,
					"error":        rbErr,
				}).Error("Failed to rollback transaction")
			}
		}
	}()

	// Lock user progress row (SELECT ... FOR UPDATE)
	progress, err := txRepo.GetProgressForUpdate(txCtx, userID, goalID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"error":        err,
		}).Error("Failed to lock progress row")
		return nil, mapper.ErrDatabaseError
	}

	// Validate progress exists
	if progress == nil {
		return nil, &mapper.GoalNotCompletedError{
			GoalID: goalID,
			Status: string(domain.GoalStatusNotStarted),
		}
	}

	// M3 Phase 6: Validate goal is active
	if !progress.IsActive {
		return nil, &mapper.GoalNotActiveError{
			GoalID:      goalID,
			ChallengeID: challengeID,
		}
	}

	// Validate goal is completed
	if !progress.CanClaim() {
		if progress.IsClaimed() {
			return nil, &mapper.GoalAlreadyClaimedError{
				GoalID:    goalID,
				ClaimedAt: progress.ClaimedAt.UTC().Format(time.RFC3339),
			}
		}

		return nil, &mapper.GoalNotCompletedError{
			GoalID: goalID,
			Status: string(progress.Status),
		}
	}

	// Check prerequisites (Decision Q7)
	// Load all user progress for prerequisite checking
	// M3 Phase 4: Get all goals (activeOnly = false) for prerequisite checking
	allProgress, err := txRepo.GetUserProgress(txCtx, userID, false)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"error":        err,
		}).Error("Failed to load user progress for prerequisite check")
		return nil, mapper.ErrDatabaseError
	}

	// Build progress map and check prerequisites
	progressMap := buildProgressMap(allProgress)
	prereqChecker := NewPrerequisiteChecker(progressMap)

	if !prereqChecker.CheckAllPrerequisitesMet(goal) {
		missingPrereqs := prereqChecker.GetMissingPrerequisites(goal)
		return nil, &mapper.PrerequisitesNotMetError{
			GoalID:         goalID,
			MissingGoalIDs: missingPrereqs,
		}
	}

	// Grant reward via AGS Platform Service with retry (Decision Q3, FQ1)
	if err := grantRewardWithRetry(txCtx, namespace, userID, goal.Reward, rewardClient); err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"reward_type":  goal.Reward.Type,
			"reward_id":    goal.Reward.RewardID,
			"error":        err,
		}).Error("Failed to grant reward after retries")
		return nil, &mapper.RewardGrantError{
			GoalID: goalID,
			Err:    err,
		}
	}

	// Mark as claimed in database
	if err := txRepo.MarkAsClaimed(txCtx, userID, goalID); err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"error":        err,
		}).Error("Failed to mark goal as claimed")
		return nil, mapper.ErrDatabaseError
	}

	// Commit transaction
	if err := txRepo.Commit(); err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"goal_id":      goalID,
			"challenge_id": challengeID,
			"error":        err,
		}).Error("Failed to commit transaction")
		return nil, mapper.ErrDatabaseError
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"goal_id":      goalID,
		"challenge_id": challengeID,
		"reward_type":  goal.Reward.Type,
		"reward_id":    goal.Reward.RewardID,
	}).Info("Successfully claimed goal reward")

	// Return result
	return &ClaimResult{
		GoalID:      goalID,
		Status:      string(domain.GoalStatusClaimed),
		Reward:      goal.Reward,
		ClaimedAt:   time.Now().UTC(),
		UserID:      userID,
		ChallengeID: challengeID,
	}, nil
}

// grantRewardWithRetry calls AGS Platform Service with exponential backoff retry.
// Decision FQ1: 3 retries with 500ms base delay
//
// Error Classification:
// - Non-retryable errors (400, 404, 403, 401) fail immediately
// - Retryable errors (network, 502, 503) use exponential backoff
//
// Retry Schedule (for retryable errors):
// - Attempt 1: Immediate
// - Attempt 2: 500ms delay
// - Attempt 3: 1000ms delay
// - Attempt 4: 2000ms delay
//
// Total delays: ~3.5s + AGS call times (4-8s) = 7.5-11.5s (fits in 10s timeout)
func grantRewardWithRetry(
	ctx context.Context,
	namespace string,
	userID string,
	reward domain.Reward,
	rewardClient client.RewardClient,
) error {
	const (
		maxRetries  = 3
		baseDelay   = 500 * time.Millisecond
		maxDelay    = 2 * time.Second
		backoffRate = 2.0
	)

	var lastErr error
	delay := baseDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Add delay before retry (skip on first attempt)
		if attempt > 0 {
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			}

			// Exponential backoff with max cap
			delay = time.Duration(float64(delay) * backoffRate)
			if delay > maxDelay {
				delay = maxDelay
			}
		}

		// Attempt to grant reward
		err := rewardClient.GrantReward(ctx, namespace, userID, reward)
		if err == nil {
			// Success
			if attempt > 0 {
				logrus.WithFields(logrus.Fields{
					"user_id":     userID,
					"reward_type": reward.Type,
					"reward_id":   reward.RewardID,
					"attempt":     attempt + 1,
				}).Info("Reward granted successfully after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable (Decision FQ1 enhancement)
		if !client.IsRetryableError(err) {
			logrus.WithFields(logrus.Fields{
				"user_id":     userID,
				"reward_type": reward.Type,
				"reward_id":   reward.RewardID,
				"attempt":     attempt + 1,
				"error":       err,
			}).Error("Reward grant failed with non-retryable error")
			return fmt.Errorf("reward grant failed (non-retryable): %w", err)
		}

		// Log retry attempt for retryable errors
		if attempt < maxRetries {
			logrus.WithFields(logrus.Fields{
				"user_id":     userID,
				"reward_type": reward.Type,
				"reward_id":   reward.RewardID,
				"attempt":     attempt + 1,
				"next_delay":  delay,
				"error":       err,
			}).Warn("Reward grant failed (retryable), retrying")
		}
	}

	// All retries exhausted
	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"reward_type": reward.Type,
		"reward_id":   reward.RewardID,
		"attempts":    maxRetries + 1,
		"error":       lastErr,
	}).Error("Reward grant failed after all retries")

	return fmt.Errorf("reward grant failed after %d retries: %w", maxRetries, lastErr)
}
