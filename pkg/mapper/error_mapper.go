package mapper

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common domain error types (Decision Q6)
var (
	ErrGoalNotFound        = errors.New("goal not found")
	ErrGoalNotCompleted    = errors.New("goal not completed")
	ErrGoalAlreadyClaimed  = errors.New("goal already claimed")
	ErrGoalNotActive       = errors.New("goal not active") // M3 Phase 6
	ErrPrerequisitesNotMet = errors.New("prerequisites not completed")
	ErrRewardGrantFailed   = errors.New("failed to grant reward")
	ErrDatabaseError       = errors.New("database error")
	ErrInvalidGoalType     = errors.New("invalid goal type")
	ErrInvalidRewardType   = errors.New("invalid reward type")
	ErrChallengeNotFound   = errors.New("challenge not found")
)

// Structured domain error types
type GoalNotFoundError struct {
	GoalID      string
	ChallengeID string
}

func (e *GoalNotFoundError) Error() string {
	return "goal not found: " + e.GoalID
}

type GoalNotCompletedError struct {
	GoalID string
	Status string
}

func (e *GoalNotCompletedError) Error() string {
	return "goal not completed: " + e.GoalID + " (status: " + e.Status + ")"
}

type GoalAlreadyClaimedError struct {
	GoalID    string
	ClaimedAt string
}

func (e *GoalAlreadyClaimedError) Error() string {
	return "goal already claimed: " + e.GoalID + " at " + e.ClaimedAt
}

// M3 Phase 6: Goal not active error
type GoalNotActiveError struct {
	GoalID      string
	ChallengeID string
}

func (e *GoalNotActiveError) Error() string {
	return "goal not active: " + e.GoalID
}

type PrerequisitesNotMetError struct {
	GoalID         string
	MissingGoalIDs []string
}

func (e *PrerequisitesNotMetError) Error() string {
	return "prerequisites not met for goal: " + e.GoalID
}

type RewardGrantError struct {
	GoalID string
	Err    error
}

func (e *RewardGrantError) Error() string {
	return "failed to grant reward for goal " + e.GoalID + ": " + e.Err.Error()
}

// MapErrorToGRPCStatus converts domain errors to gRPC status codes (Decision Q6)
func MapErrorToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	// Check for structured error types
	var goalNotFound *GoalNotFoundError
	if errors.As(err, &goalNotFound) {
		return status.Errorf(codes.NotFound,
			"Goal not found or removed from config (goal_id: %s, challenge_id: %s)",
			goalNotFound.GoalID, goalNotFound.ChallengeID)
	}

	var goalNotCompleted *GoalNotCompletedError
	if errors.As(err, &goalNotCompleted) {
		return status.Errorf(codes.FailedPrecondition,
			"Goal not completed. Please wait 1 second and try again. (goal_id: %s, status: %s)",
			goalNotCompleted.GoalID, goalNotCompleted.Status)
	}

	var goalAlreadyClaimed *GoalAlreadyClaimedError
	if errors.As(err, &goalAlreadyClaimed) {
		return status.Errorf(codes.AlreadyExists,
			"Reward has already been claimed (goal_id: %s, claimed_at: %s)",
			goalAlreadyClaimed.GoalID, goalAlreadyClaimed.ClaimedAt)
	}

	// M3 Phase 6: Goal not active error
	var goalNotActive *GoalNotActiveError
	if errors.As(err, &goalNotActive) {
		return status.Errorf(codes.FailedPrecondition,
			"Goal must be active to claim reward. Activate it first via PUT /v1/challenges/%s/goals/%s/active (goal_id: %s)",
			goalNotActive.ChallengeID, goalNotActive.GoalID, goalNotActive.GoalID)
	}

	var prerequisitesNotMet *PrerequisitesNotMetError
	if errors.As(err, &prerequisitesNotMet) {
		return status.Errorf(codes.FailedPrecondition,
			"Prerequisites not completed (goal_id: %s)",
			prerequisitesNotMet.GoalID)
	}

	var rewardGrantErr *RewardGrantError
	if errors.As(err, &rewardGrantErr) {
		return status.Errorf(codes.Internal,
			"Failed to grant reward via Platform Service after 3 retries (goal_id: %s)",
			rewardGrantErr.GoalID)
	}

	// Check for sentinel errors
	switch {
	case errors.Is(err, ErrGoalNotFound):
		return status.Error(codes.NotFound, "Goal not found or removed from config")
	case errors.Is(err, ErrGoalNotCompleted):
		return status.Error(codes.FailedPrecondition, "Goal not completed. Please wait 1 second and try again.")
	case errors.Is(err, ErrGoalAlreadyClaimed):
		return status.Error(codes.AlreadyExists, "Reward has already been claimed")
	case errors.Is(err, ErrGoalNotActive): // M3 Phase 6
		return status.Error(codes.FailedPrecondition, "Goal must be active to claim reward. Activate it first.")
	case errors.Is(err, ErrPrerequisitesNotMet):
		return status.Error(codes.FailedPrecondition, "Prerequisites not completed")
	case errors.Is(err, ErrRewardGrantFailed):
		return status.Error(codes.Internal, "Failed to grant reward via Platform Service after 3 retries")
	case errors.Is(err, ErrDatabaseError):
		return status.Error(codes.Internal, "Database error occurred")
	case errors.Is(err, ErrInvalidGoalType):
		return status.Error(codes.InvalidArgument, "Invalid goal type")
	case errors.Is(err, ErrInvalidRewardType):
		return status.Error(codes.InvalidArgument, "Invalid reward type")
	case errors.Is(err, ErrChallengeNotFound):
		return status.Error(codes.NotFound, "Challenge not found")
	}

	// Default to internal error for unknown errors
	return status.Error(codes.Internal, "Internal server error")
}
