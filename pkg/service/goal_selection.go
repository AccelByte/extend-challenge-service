// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/sirupsen/logrus"
)

// GoalSelectionResult represents the result of batch or random goal selection.
type GoalSelectionResult struct {
	SelectedGoals    []*SelectedGoalInfo
	ChallengeID      string
	TotalActiveGoals int
	ReplacedGoals    []string
}

// SelectedGoalInfo contains detailed information about a selected goal.
type SelectedGoalInfo struct {
	GoalID      string
	Name        string
	Description string
	Requirement domain.Requirement
	Reward      domain.Reward
	Status      string
	Progress    int
	Target      int
	IsActive    bool
	AssignedAt  *time.Time
	ExpiresAt   *time.Time
}

// RandomSelectGoals randomly selects N goals from a challenge and activates them.
//
// This function implements the random selection logic from TECH_SPEC_M4.md (lines 613-715).
//
// Algorithm:
//  1. Validate challenge exists
//  2. Get user's current progress
//  3. Filter available goals (exclude completed/claimed/prerequisites)
//  4. Handle insufficient goals (return partial results)
//  5. Random sample using crypto/rand (Fisher-Yates shuffle)
//  6. Database transaction:
//     a. Deactivate existing (if replace mode)
//     b. Activate selected goals (BATCH operation)
//  7. Build response with goal details
//
// Parameters:
//   - ctx: Request context for cancellation and timeout
//   - userID: User identifier (extracted from JWT)
//   - challengeID: Challenge identifier (from URL path)
//   - count: Number of goals to select
//   - replaceExisting: Whether to deactivate existing active goals first
//   - excludeActive: Whether to exclude already-active goals from selection pool
//   - namespace: Namespace (extracted from JWT)
//   - goalCache: In-memory config cache for goal validation
//   - repo: Database repository for persisting changes
//
// Returns:
//   - GoalSelectionResult with selected goals and metadata
//   - error if validation fails or database error occurs
//
// Error Cases:
//   - Challenge not found: returns error
//   - Invalid count (<= 0): returns error
//   - No goals available (after filtering): returns error
//   - Database error: returns wrapped error
//
// Notes:
//   - If fewer goals available than requested (but > 0), returns partial results
//   - Uses BatchUpsertGoalActive for performance (~10ms for 10 goals)
//   - Deactivated goals keep their progress (not reset)
func RandomSelectGoals(
	ctx context.Context,
	userID string,
	challengeID string,
	count int,
	replaceExisting bool,
	excludeActive bool,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
) (*GoalSelectionResult, error) {
	// Early return validation
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if challengeID == "" {
		return nil, fmt.Errorf("challenge ID cannot be empty")
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	if count <= 0 {
		return nil, fmt.Errorf("count must be greater than 0")
	}

	if goalCache == nil {
		return nil, fmt.Errorf("goal cache cannot be nil")
	}

	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// 1. Validate challenge exists
	challenge := goalCache.GetChallengeByChallengeID(challengeID)
	if challenge == nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
		}).Warn("Challenge not found in config")
		return nil, fmt.Errorf("challenge '%s' not found", challengeID)
	}

	// 2. Get user's current progress
	userProgress, err := repo.GetChallengeProgress(ctx, userID, challengeID, false)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to get user progress")
		return nil, fmt.Errorf("failed to get user progress: %w", err)
	}

	// Convert to map for easier lookup
	progressMap := make(map[string]*domain.UserGoalProgress)
	for _, p := range userProgress {
		progressMap[p.GoalID] = p
	}

	// 3. Filter available goals
	// When replace_existing=true, exclude currently active goals from selection
	// to ensure we select completely new goals (not reselecting ones we'll deactivate)
	shouldExcludeActive := excludeActive || replaceExisting
	availableGoalIDs := filterAvailableGoals(challenge.Goals, progressMap, shouldExcludeActive, goalCache)

	// 4. Handle insufficient goals
	if len(availableGoalIDs) == 0 {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"requested":    count,
			"available":    0,
		}).Warn("No goals available for selection")
		return nil, fmt.Errorf("no goals available for selection")
	}

	// Return partial results if fewer available than requested
	actualCount := count
	if len(availableGoalIDs) < count {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"requested":    count,
			"available":    len(availableGoalIDs),
		}).Info("Fewer goals available than requested, returning partial results")
		actualCount = len(availableGoalIDs)
	}

	// 5. Random sample using crypto/rand
	selectedGoalIDs, err := randomSample(availableGoalIDs, actualCount)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"count":        actualCount,
			"error":        err,
		}).Error("Failed to random sample goals")
		return nil, fmt.Errorf("failed to random sample goals: %w", err)
	}

	// 6. Database transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Rollback can fail if transaction already committed, which is fine
			logrus.WithError(err).Debug("Transaction rollback (expected if already committed)")
		}
	}()

	// 7. Deactivate existing (if replace mode)
	replacedGoals := []string{}
	if replaceExisting {
		activeGoals := getActiveGoalIDs(progressMap)
		if len(activeGoals) > 0 {
			// Deactivate ALL active goals when replacing
			// (selectedGoalIDs will only contain inactive goals since we excluded active ones above)
			deactivateBatch := make([]*domain.UserGoalProgress, len(activeGoals))
			now := time.Now().UTC()
			for i, goalID := range activeGoals {
				deactivateBatch[i] = &domain.UserGoalProgress{
					UserID:      userID,
					GoalID:      goalID,
					ChallengeID: challengeID,
					Namespace:   namespace,
					IsActive:    false,
					AssignedAt:  &now, // Keep timestamp but mark inactive
				}
			}

			// Use batch operation for deactivation
			err = tx.BatchUpsertGoalActive(ctx, deactivateBatch)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"user_id":      userID,
					"challenge_id": challengeID,
					"namespace":    namespace,
					"goal_count":   len(activeGoals),
					"error":        err,
				}).Error("Failed to deactivate goals")
				return nil, fmt.Errorf("failed to deactivate goals: %w", err)
			}

			replacedGoals = activeGoals
		}
	}

	// 8. Activate selected goals (BATCH operation for performance)
	now := time.Now().UTC()
	goalBatch := make([]*domain.UserGoalProgress, len(selectedGoalIDs))
	for i, goalID := range selectedGoalIDs {
		goalBatch[i] = &domain.UserGoalProgress{
			UserID:      userID,
			GoalID:      goalID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   nil, // M4: no rotation yet
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
		}
	}

	// Single batch operation instead of N queries
	err = tx.BatchUpsertGoalActive(ctx, goalBatch)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"goal_count":   len(selectedGoalIDs),
			"error":        err,
		}).Error("Failed to batch activate goals")
		return nil, fmt.Errorf("failed to batch activate goals: %w", err)
	}

	// 9. Commit transaction
	if err = tx.Commit(); err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 10. Build response with goal details
	selectedGoalDetails := buildGoalDetails(challenge, selectedGoalIDs, &now)
	totalActive := len(selectedGoalIDs)
	if !replaceExisting {
		// Add existing active count
		totalActive += len(getActiveGoalIDs(progressMap))
		// Subtract any that were already active and got re-activated
		for _, goalID := range selectedGoalIDs {
			if progressMap[goalID] != nil && progressMap[goalID].IsActive {
				totalActive--
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"challenge_id": challengeID,
		"namespace":    namespace,
		"selected":     len(selectedGoalIDs),
		"total_active": totalActive,
		"replaced":     len(replacedGoals),
	}).Info("Successfully completed random goal selection")

	return &GoalSelectionResult{
		SelectedGoals:    selectedGoalDetails,
		ChallengeID:      challengeID,
		TotalActiveGoals: totalActive,
		ReplacedGoals:    replacedGoals,
	}, nil
}

// BatchSelectGoals activates multiple player-selected goals at once.
//
// This function implements the batch manual selection logic from TECH_SPEC_M4.md (lines 464-525).
//
// Flow:
//  1. Validate all goal IDs exist in the challenge
//  2. Database transaction:
//     a. Deactivate existing (if replace mode)
//     b. Activate selected goals (BATCH operation)
//  3. Build response with goal details
//
// Parameters:
//   - ctx: Request context for cancellation and timeout
//   - userID: User identifier (extracted from JWT)
//   - challengeID: Challenge identifier (from URL path)
//   - goalIDs: List of goal IDs to activate
//   - replaceExisting: Whether to deactivate existing active goals first
//   - namespace: Namespace (extracted from JWT)
//   - goalCache: In-memory config cache for goal validation
//   - repo: Database repository for persisting changes
//
// Returns:
//   - GoalSelectionResult with selected goals and metadata
//   - error if validation fails or database error occurs
//
// Error Cases:
//   - Empty goal list: returns error
//   - Goal not found in config: returns error
//   - Goal not in specified challenge: returns error
//   - Database error: returns wrapped error
func BatchSelectGoals(
	ctx context.Context,
	userID string,
	challengeID string,
	goalIDs []string,
	replaceExisting bool,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
) (*GoalSelectionResult, error) {
	// Early return validation
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if challengeID == "" {
		return nil, fmt.Errorf("challenge ID cannot be empty")
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	if len(goalIDs) == 0 {
		return nil, fmt.Errorf("goal IDs list cannot be empty")
	}

	if goalCache == nil {
		return nil, fmt.Errorf("goal cache cannot be nil")
	}

	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// 1. Validate challenge exists
	challenge := goalCache.GetChallengeByChallengeID(challengeID)
	if challenge == nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
		}).Warn("Challenge not found in config")
		return nil, fmt.Errorf("challenge '%s' not found", challengeID)
	}

	// 2. Validate all goals exist and belong to this challenge
	for _, goalID := range goalIDs {
		goal := goalCache.GetGoalByID(goalID)
		if goal == nil {
			logrus.WithFields(logrus.Fields{
				"user_id":      userID,
				"challenge_id": challengeID,
				"goal_id":      goalID,
				"namespace":    namespace,
			}).Warn("Goal not found in config")
			return nil, fmt.Errorf("goal '%s' not found", goalID)
		}

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
	}

	// 3. Get user's current progress (for replace mode and total count)
	userProgress, err := repo.GetChallengeProgress(ctx, userID, challengeID, false)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to get user progress")
		return nil, fmt.Errorf("failed to get user progress: %w", err)
	}

	// Convert to map for easier lookup
	progressMap := make(map[string]*domain.UserGoalProgress)
	for _, p := range userProgress {
		progressMap[p.GoalID] = p
	}

	// 4. Database transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			logrus.WithError(err).Debug("Transaction rollback (expected if already committed)")
		}
	}()

	// 5. Deactivate existing (if replace mode)
	replacedGoals := []string{}
	if replaceExisting {
		activeGoals := getActiveGoalIDs(progressMap)
		if len(activeGoals) > 0 {
			// Deactivate by setting is_active = false for all active goals
			deactivateBatch := make([]*domain.UserGoalProgress, len(activeGoals))
			now := time.Now().UTC()
			for i, goalID := range activeGoals {
				deactivateBatch[i] = &domain.UserGoalProgress{
					UserID:      userID,
					GoalID:      goalID,
					ChallengeID: challengeID,
					Namespace:   namespace,
					IsActive:    false,
					AssignedAt:  &now,
				}
			}

			err = tx.BatchUpsertGoalActive(ctx, deactivateBatch)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"user_id":      userID,
					"challenge_id": challengeID,
					"namespace":    namespace,
					"goal_count":   len(activeGoals),
					"error":        err,
				}).Error("Failed to deactivate goals")
				return nil, fmt.Errorf("failed to deactivate goals: %w", err)
			}

			replacedGoals = activeGoals
		}
	}

	// 6. Activate selected goals (BATCH operation for performance)
	now := time.Now().UTC()
	goalBatch := make([]*domain.UserGoalProgress, len(goalIDs))
	for i, goalID := range goalIDs {
		goalBatch[i] = &domain.UserGoalProgress{
			UserID:      userID,
			GoalID:      goalID,
			ChallengeID: challengeID,
			Namespace:   namespace,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   nil, // M4: no rotation yet
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
		}
	}

	// Single batch operation instead of N queries
	err = tx.BatchUpsertGoalActive(ctx, goalBatch)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"goal_count":   len(goalIDs),
			"error":        err,
		}).Error("Failed to batch activate goals")
		return nil, fmt.Errorf("failed to batch activate goals: %w", err)
	}

	// 7. Commit transaction
	if err = tx.Commit(); err != nil {
		logrus.WithFields(logrus.Fields{
			"user_id":      userID,
			"challenge_id": challengeID,
			"namespace":    namespace,
			"error":        err,
		}).Error("Failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 8. Build response with goal details
	selectedGoalDetails := buildGoalDetails(challenge, goalIDs, &now)
	totalActive := len(goalIDs)
	if !replaceExisting {
		// Add existing active count
		totalActive += len(getActiveGoalIDs(progressMap))
		// Subtract any that were already active and got re-activated
		for _, goalID := range goalIDs {
			if progressMap[goalID] != nil && progressMap[goalID].IsActive {
				totalActive--
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"challenge_id": challengeID,
		"namespace":    namespace,
		"selected":     len(goalIDs),
		"total_active": totalActive,
		"replaced":     len(replacedGoals),
	}).Info("Successfully completed batch goal selection")

	return &GoalSelectionResult{
		SelectedGoals:    selectedGoalDetails,
		ChallengeID:      challengeID,
		TotalActiveGoals: totalActive,
		ReplacedGoals:    replacedGoals,
	}, nil
}

// filterAvailableGoals filters goals based on user progress and exclusion criteria.
//
// Auto-applied filters (mandatory):
// - Exclude goals with status = 'claimed'
// - Exclude goals with status = 'completed' (should claim first)
// - Exclude goals with unmet prerequisites
//
// Optional filters:
// - excludeActive: Exclude goals with is_active = true
//
// Parameters:
//   - allGoals: All goals in the challenge
//   - userProgress: Map of goal_id -> UserGoalProgress
//   - excludeActive: Whether to exclude already-active goals
//   - goalCache: For prerequisite checking
//
// Returns:
//   - Slice of available goal IDs
func filterAvailableGoals(
	allGoals []*domain.Goal,
	userProgress map[string]*domain.UserGoalProgress,
	excludeActive bool,
	goalCache cache.GoalCache,
) []string {
	available := []string{}

	for _, goal := range allGoals {
		progress := userProgress[goal.ID]

		// Skip completed goals
		if progress != nil && (progress.Status == domain.GoalStatusCompleted || progress.Status == domain.GoalStatusClaimed) {
			continue
		}

		// Skip active goals (if requested)
		if excludeActive && progress != nil && progress.IsActive {
			continue
		}

		// Skip goals with unmet prerequisites
		if hasUnmetPrerequisites(goal, userProgress, goalCache) {
			continue
		}

		available = append(available, goal.ID)
	}

	return available
}

// hasUnmetPrerequisites checks if a goal has any unmet prerequisites.
//
// Parameters:
//   - goal: The goal to check
//   - userProgress: Map of goal_id -> UserGoalProgress
//   - goalCache: For retrieving prerequisite goal definitions
//
// Returns:
//   - true if goal has unmet prerequisites, false otherwise
func hasUnmetPrerequisites(
	goal *domain.Goal,
	userProgress map[string]*domain.UserGoalProgress,
	goalCache cache.GoalCache,
) bool {
	if len(goal.Prerequisites) == 0 {
		return false
	}

	for _, prereqID := range goal.Prerequisites {
		progress := userProgress[prereqID]
		// Prerequisite not met if not completed or claimed
		if progress == nil || (progress.Status != domain.GoalStatusCompleted && progress.Status != domain.GoalStatusClaimed) {
			return true
		}
	}

	return false
}

// randomSample selects N random elements from a slice using Fisher-Yates shuffle.
//
// Uses crypto/rand for quality randomness (industry standard for non-cryptographic random selection).
//
// Parameters:
//   - pool: Slice of goal IDs to sample from
//   - count: Number of elements to select
//
// Returns:
//   - Slice of randomly selected goal IDs
//   - error if crypto/rand fails
func randomSample(pool []string, count int) ([]string, error) {
	if count > len(pool) {
		count = len(pool)
	}

	if count == 0 {
		return []string{}, nil
	}

	// Copy to avoid modifying input
	poolCopy := make([]string, len(pool))
	copy(poolCopy, pool)

	// Fisher-Yates shuffle for first N elements
	for i := 0; i < count; i++ {
		// Generate random index using crypto/rand
		max := big.NewInt(int64(len(poolCopy) - i))
		randomBig, err := rand.Int(rand.Reader, max)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random number: %w", err)
		}

		j := i + int(randomBig.Int64())
		poolCopy[i], poolCopy[j] = poolCopy[j], poolCopy[i]
	}

	return poolCopy[:count], nil
}

// getActiveGoalIDs extracts goal IDs that are currently active.
//
// Parameters:
//   - progressMap: Map of goal_id -> UserGoalProgress
//
// Returns:
//   - Slice of active goal IDs
func getActiveGoalIDs(progressMap map[string]*domain.UserGoalProgress) []string {
	activeGoals := []string{}
	for goalID, progress := range progressMap {
		if progress.IsActive {
			activeGoals = append(activeGoals, goalID)
		}
	}
	return activeGoals
}

// buildGoalDetails constructs detailed goal information for response.
//
// Parameters:
//   - challenge: The challenge containing the goals
//   - goalIDs: List of goal IDs to build details for
//   - assignedAt: Timestamp when goals were assigned
//
// Returns:
//   - Slice of SelectedGoalInfo with full goal details
func buildGoalDetails(
	challenge *domain.Challenge,
	goalIDs []string,
	assignedAt *time.Time,
) []*SelectedGoalInfo {
	goalDetails := make([]*SelectedGoalInfo, 0, len(goalIDs))

	for _, goalID := range goalIDs {
		// Find goal in challenge
		var goal *domain.Goal
		for _, g := range challenge.Goals {
			if g.ID == goalID {
				goal = g
				break
			}
		}

		if goal == nil {
			continue
		}

		goalDetails = append(goalDetails, &SelectedGoalInfo{
			GoalID:      goal.ID,
			Name:        goal.Name,
			Description: goal.Description,
			Requirement: goal.Requirement,
			Reward:      goal.Reward,
			Status:      string(domain.GoalStatusNotStarted), // Newly activated
			Progress:    0,
			Target:      goal.Requirement.TargetValue,
			IsActive:    true,
			AssignedAt:  assignedAt,
			ExpiresAt:   nil, // M4: no rotation yet
		})
	}

	return goalDetails
}
