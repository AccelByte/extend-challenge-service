package mapper

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapErrorToGRPCStatus_GoalNotFoundError(t *testing.T) {
	err := &GoalNotFoundError{
		GoalID:      "goal-1",
		ChallengeID: "challenge-1",
	}

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "goal-1")
	assert.Contains(t, st.Message(), "challenge-1")
}

func TestMapErrorToGRPCStatus_GoalNotCompletedError(t *testing.T) {
	err := &GoalNotCompletedError{
		GoalID: "goal-1",
		Status: "in_progress",
	}

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Contains(t, st.Message(), "not completed")
	assert.Contains(t, st.Message(), "goal-1")
}

func TestMapErrorToGRPCStatus_GoalAlreadyClaimedError(t *testing.T) {
	err := &GoalAlreadyClaimedError{
		GoalID:    "goal-1",
		ClaimedAt: "2025-10-18T10:00:00Z",
	}

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
	assert.Contains(t, st.Message(), "already been claimed")
	assert.Contains(t, st.Message(), "goal-1")
}

func TestMapErrorToGRPCStatus_PrerequisitesNotMetError(t *testing.T) {
	err := &PrerequisitesNotMetError{
		GoalID:         "goal-2",
		MissingGoalIDs: []string{"goal-1"},
	}

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Contains(t, st.Message(), "Prerequisites not completed")
}

func TestMapErrorToGRPCStatus_RewardGrantError(t *testing.T) {
	err := &RewardGrantError{
		GoalID: "goal-1",
		Err:    errors.New("platform service timeout"),
	}

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Failed to grant reward")
	assert.Contains(t, st.Message(), "3 retries")
}

func TestMapErrorToGRPCStatus_SentinelErrGoalNotFound(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrGoalNotFound)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrGoalNotCompleted(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrGoalNotCompleted)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrGoalAlreadyClaimed(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrGoalAlreadyClaimed)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrPrerequisitesNotMet(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrPrerequisitesNotMet)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrRewardGrantFailed(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrRewardGrantFailed)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrDatabaseError(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrDatabaseError)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrInvalidGoalType(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrInvalidGoalType)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrInvalidRewardType(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrInvalidRewardType)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMapErrorToGRPCStatus_SentinelErrChallengeNotFound(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(ErrChallengeNotFound)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMapErrorToGRPCStatus_UnknownError(t *testing.T) {
	err := errors.New("unknown error")

	grpcErr := MapErrorToGRPCStatus(err)

	st, ok := status.FromError(grpcErr)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Internal server error")
}

func TestMapErrorToGRPCStatus_NilError(t *testing.T) {
	grpcErr := MapErrorToGRPCStatus(nil)

	assert.Nil(t, grpcErr)
}

func TestGoalNotFoundError_Error(t *testing.T) {
	err := &GoalNotFoundError{
		GoalID:      "goal-1",
		ChallengeID: "challenge-1",
	}

	assert.Contains(t, err.Error(), "goal-1")
}

func TestGoalNotCompletedError_Error(t *testing.T) {
	err := &GoalNotCompletedError{
		GoalID: "goal-1",
		Status: "in_progress",
	}

	assert.Contains(t, err.Error(), "goal-1")
	assert.Contains(t, err.Error(), "in_progress")
}

func TestGoalAlreadyClaimedError_Error(t *testing.T) {
	err := &GoalAlreadyClaimedError{
		GoalID:    "goal-1",
		ClaimedAt: "2025-10-18",
	}

	assert.Contains(t, err.Error(), "goal-1")
	assert.Contains(t, err.Error(), "2025-10-18")
}

func TestPrerequisitesNotMetError_Error(t *testing.T) {
	err := &PrerequisitesNotMetError{
		GoalID:         "goal-2",
		MissingGoalIDs: []string{"goal-1"},
	}

	assert.Contains(t, err.Error(), "goal-2")
}

func TestRewardGrantError_Error(t *testing.T) {
	err := &RewardGrantError{
		GoalID: "goal-1",
		Err:    errors.New("timeout"),
	}

	assert.Contains(t, err.Error(), "goal-1")
	assert.Contains(t, err.Error(), "timeout")
}
