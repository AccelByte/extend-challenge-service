package service

import (
	"context"
	"fmt"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// ChallengeWithProgress represents a challenge with user progress data.
// This is returned by progress query helpers to combine config and progress.
type ChallengeWithProgress struct {
	Challenge    *domain.Challenge
	UserProgress map[string]*domain.UserGoalProgress // Key: goal_id
}

// GetUserChallengesWithProgress retrieves all challenges with user progress.
// This helper function is used by the GetUserChallenges RPC handler.
//
// Steps:
// 1. Load all challenges from cache (O(1))
// 2. Load all user progress from repository (single DB query)
// 3. Build map of progress for efficient lookup
// 4. Return combined data
//
// Performance: ~10-20ms for 50 challenges with 200 goals
//
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func GetUserChallengesWithProgress(
	ctx context.Context,
	userID string,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
	activeOnly bool,
) ([]*ChallengeWithProgress, error) {
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

	// Get all challenges from cache (O(1))
	challenges := goalCache.GetAllChallenges()
	if len(challenges) == 0 {
		// No challenges configured, return empty result
		return []*ChallengeWithProgress{}, nil
	}

	// Load all user progress from DB (single query)
	// M3 Phase 4: Pass activeOnly parameter to filter goals
	allProgress, err := repo.GetUserProgress(ctx, userID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to load user progress: %w", err)
	}

	// Build map for O(1) progress lookups
	progressMap := buildProgressMap(allProgress)

	// Combine challenges with progress
	result := make([]*ChallengeWithProgress, 0, len(challenges))
	for _, challenge := range challenges {
		result = append(result, &ChallengeWithProgress{
			Challenge:    challenge,
			UserProgress: progressMap,
		})
	}

	return result, nil
}

// GetUserChallengeWithProgress retrieves a single challenge with user progress.
// This helper function is used by the GetUserChallenge RPC handler.
//
// Steps:
// 1. Load challenge from cache (O(1))
// 2. Load challenge-specific progress from repository (indexed query)
// 3. Build map of progress for efficient lookup
// 4. Return combined data
//
// Performance: ~5-10ms for 10 goals per challenge
//
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func GetUserChallengeWithProgress(
	ctx context.Context,
	userID string,
	challengeID string,
	namespace string,
	goalCache cache.GoalCache,
	repo repository.GoalRepository,
	activeOnly bool,
) (*ChallengeWithProgress, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
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

	// Get challenge from cache (O(1))
	challenge := goalCache.GetChallengeByChallengeID(challengeID)
	if challenge == nil {
		return nil, fmt.Errorf("challenge not found: %s", challengeID)
	}

	// Load challenge-specific progress from DB (indexed query)
	// M3 Phase 4: Pass activeOnly parameter to filter goals
	challengeProgress, err := repo.GetChallengeProgress(ctx, userID, challengeID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to load challenge progress: %w", err)
	}

	// Build map for O(1) progress lookups
	progressMap := buildProgressMap(challengeProgress)

	return &ChallengeWithProgress{
		Challenge:    challenge,
		UserProgress: progressMap,
	}, nil
}

// buildProgressMap creates a map from progress slice for O(1) lookups.
// This is a simple function-scoped helper, not a persistent cache.
//
// Example: For 100 progress records → O(100) to build map, then O(1) for each lookup
// Without map: Each lookup would be O(100) linear search → 50 goals * 100 = 5,000 comparisons
func buildProgressMap(progress []*domain.UserGoalProgress) map[string]*domain.UserGoalProgress {
	progressMap := make(map[string]*domain.UserGoalProgress, len(progress))
	for _, p := range progress {
		progressMap[p.GoalID] = p
	}
	return progressMap
}
