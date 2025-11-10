package mapper

import (
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChallengeToProto_Success(t *testing.T) {
	challenge := &domain.Challenge{
		ID:          "winter-challenge",
		Name:        "Winter Challenge",
		Description: "Complete winter goals",
		Goals: []*domain.Goal{
			{
				ID:          "goal-1",
				Name:        "Goal 1",
				Description: "First goal",
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
				Prerequisites: []string{},
			},
		},
	}

	userProgress := map[string]*domain.UserGoalProgress{
		"goal-1": {
			UserID:      "user123",
			GoalID:      "goal-1",
			ChallengeID: "winter-challenge",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		},
	}

	pbChallenge, err := ChallengeToProto(challenge, userProgress)

	require.NoError(t, err)
	assert.Equal(t, "winter-challenge", pbChallenge.ChallengeId)
	assert.Equal(t, "Winter Challenge", pbChallenge.Name)
	assert.Equal(t, "Complete winter goals", pbChallenge.Description)
	assert.Len(t, pbChallenge.Goals, 1)
	assert.Equal(t, "goal-1", pbChallenge.Goals[0].GoalId)
	assert.Equal(t, int32(5), pbChallenge.Goals[0].Progress)
}

func TestChallengeToProto_NilChallenge(t *testing.T) {
	_, err := ChallengeToProto(nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "challenge cannot be nil")
}

func TestGoalToProto_WithProgress(t *testing.T) {
	goal := &domain.Goal{
		ID:          "goal-1",
		Name:        "Test Goal",
		Description: "Test description",
		Type:        domain.GoalTypeAbsolute,
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
		Prerequisites: []string{"goal-0"},
	}

	completedAt := time.Now().UTC()
	userProgress := map[string]*domain.UserGoalProgress{
		"goal-1": {
			UserID:      "user123",
			GoalID:      "goal-1",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &completedAt,
		},
	}

	pbGoal, err := GoalToProto(goal, userProgress)

	require.NoError(t, err)
	assert.Equal(t, "goal-1", pbGoal.GoalId)
	assert.Equal(t, "Test Goal", pbGoal.Name)
	assert.Equal(t, int32(10), pbGoal.Progress)
	assert.Equal(t, string(domain.GoalStatusCompleted), pbGoal.Status)
	assert.NotEmpty(t, pbGoal.CompletedAt)
	assert.Empty(t, pbGoal.ClaimedAt)
}

func TestGoalToProto_NoProgress(t *testing.T) {
	goal := &domain.Goal{
		ID:          "goal-1",
		Name:        "Test Goal",
		Description: "Test description",
		Type:        domain.GoalTypeAbsolute,
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
		Prerequisites: []string{},
	}

	pbGoal, err := GoalToProto(goal, map[string]*domain.UserGoalProgress{})

	require.NoError(t, err)
	assert.Equal(t, int32(0), pbGoal.Progress)
	assert.Equal(t, string(domain.GoalStatusNotStarted), pbGoal.Status)
	assert.Empty(t, pbGoal.CompletedAt)
	assert.Empty(t, pbGoal.ClaimedAt)
	assert.False(t, pbGoal.Locked) // No prerequisites
}

func TestGoalToProto_NilGoal(t *testing.T) {
	_, err := GoalToProto(nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal cannot be nil")
}

func TestComputeProgress_DailyGoal_CompletedToday(t *testing.T) {
	goal := &domain.Goal{
		Type: domain.GoalTypeDaily,
		Requirement: domain.Requirement{
			TargetValue: 1,
		},
	}

	// Completed today
	now := time.Now().UTC()
	progress := &domain.UserGoalProgress{
		Progress:    1,
		CompletedAt: &now,
	}

	result := ComputeProgress(goal, progress)

	assert.Equal(t, int32(1), result)
}

func TestComputeProgress_DailyGoal_CompletedYesterday(t *testing.T) {
	goal := &domain.Goal{
		Type: domain.GoalTypeDaily,
		Requirement: domain.Requirement{
			TargetValue: 1,
		},
	}

	// Completed yesterday
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	progress := &domain.UserGoalProgress{
		Progress:    1,
		CompletedAt: &yesterday,
	}

	result := ComputeProgress(goal, progress)

	assert.Equal(t, int32(0), result) // Reset to 0 because not completed today
}

func TestComputeProgress_DailyGoal_NotCompleted(t *testing.T) {
	goal := &domain.Goal{
		Type: domain.GoalTypeDaily,
		Requirement: domain.Requirement{
			TargetValue: 1,
		},
	}

	progress := &domain.UserGoalProgress{
		Progress: 0,
	}

	result := ComputeProgress(goal, progress)

	assert.Equal(t, int32(0), result)
}

func TestComputeProgress_AbsoluteGoal(t *testing.T) {
	goal := &domain.Goal{
		Type: domain.GoalTypeAbsolute,
		Requirement: domain.Requirement{
			TargetValue: 10,
		},
	}

	progress := &domain.UserGoalProgress{
		Progress: 7,
	}

	result := ComputeProgress(goal, progress)

	assert.Equal(t, int32(7), result)
}

func TestComputeProgress_IncrementGoal(t *testing.T) {
	goal := &domain.Goal{
		Type: domain.GoalTypeIncrement,
		Requirement: domain.Requirement{
			TargetValue: 10,
		},
	}

	progress := &domain.UserGoalProgress{
		Progress: 5,
	}

	result := ComputeProgress(goal, progress)

	assert.Equal(t, int32(5), result)
}

func TestComputeProgress_NilInputs(t *testing.T) {
	assert.Equal(t, int32(0), ComputeProgress(nil, nil))
	assert.Equal(t, int32(0), ComputeProgress(&domain.Goal{}, nil))
	assert.Equal(t, int32(0), ComputeProgress(nil, &domain.UserGoalProgress{}))
}

func TestRequirementToProto_Success(t *testing.T) {
	req := &domain.Requirement{
		StatCode:    "kills",
		Operator:    ">=",
		TargetValue: 10,
	}

	pbReq, err := RequirementToProto(req)

	require.NoError(t, err)
	assert.Equal(t, "kills", pbReq.StatCode)
	assert.Equal(t, ">=", pbReq.Operator)
	assert.Equal(t, int32(10), pbReq.TargetValue)
}

func TestRequirementToProto_Nil(t *testing.T) {
	_, err := RequirementToProto(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requirement cannot be nil")
}

func TestRewardToProto_ItemReward(t *testing.T) {
	reward := &domain.Reward{
		Type:     string(domain.RewardTypeItem),
		RewardID: "sword",
		Quantity: 1,
	}

	pbReward, err := RewardToProto(reward)

	require.NoError(t, err)
	assert.Equal(t, string(domain.RewardTypeItem), pbReward.Type)
	assert.Equal(t, "sword", pbReward.RewardId)
	assert.Equal(t, int32(1), pbReward.Quantity)
}

func TestRewardToProto_WalletReward(t *testing.T) {
	reward := &domain.Reward{
		Type:     string(domain.RewardTypeWallet),
		RewardID: "GOLD",
		Quantity: 100,
	}

	pbReward, err := RewardToProto(reward)

	require.NoError(t, err)
	assert.Equal(t, string(domain.RewardTypeWallet), pbReward.Type)
	assert.Equal(t, "GOLD", pbReward.RewardId)
	assert.Equal(t, int32(100), pbReward.Quantity)
}

func TestRewardToProto_InvalidType(t *testing.T) {
	reward := &domain.Reward{
		Type:     "INVALID",
		RewardID: "test",
		Quantity: 1,
	}

	_, err := RewardToProto(reward)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid reward type")
}

func TestRewardToProto_Nil(t *testing.T) {
	_, err := RewardToProto(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reward cannot be nil")
}

func TestChallengesToProto_Success(t *testing.T) {
	challenges := []*domain.Challenge{
		{
			ID:          "challenge-1",
			Name:        "Challenge 1",
			Description: "First challenge",
			Goals:       []*domain.Goal{},
		},
		{
			ID:          "challenge-2",
			Name:        "Challenge 2",
			Description: "Second challenge",
			Goals:       []*domain.Goal{},
		},
	}

	pbChallenges, err := ChallengesToProto(challenges, map[string]*domain.UserGoalProgress{})

	require.NoError(t, err)
	assert.Len(t, pbChallenges, 2)
	assert.Equal(t, "challenge-1", pbChallenges[0].ChallengeId)
	assert.Equal(t, "challenge-2", pbChallenges[1].ChallengeId)
}

func TestChallengesToProto_EmptySlice(t *testing.T) {
	pbChallenges, err := ChallengesToProto([]*domain.Challenge{}, map[string]*domain.UserGoalProgress{})

	require.NoError(t, err)
	assert.Empty(t, pbChallenges)
}

func TestChallengesToProto_Nil(t *testing.T) {
	_, err := ChallengesToProto(nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "challenges cannot be nil")
}

func TestGoalToProto_WithClaimedProgress(t *testing.T) {
	goal := &domain.Goal{
		ID:          "goal-1",
		Name:        "Test Goal",
		Description: "Test description",
		Type:        domain.GoalTypeAbsolute,
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
		Prerequisites: []string{},
	}

	completedAt := time.Now().UTC().Add(-1 * time.Hour)
	claimedAt := time.Now().UTC()
	userProgress := map[string]*domain.UserGoalProgress{
		"goal-1": {
			UserID:      "user123",
			GoalID:      "goal-1",
			Progress:    10,
			Status:      domain.GoalStatusClaimed,
			CompletedAt: &completedAt,
			ClaimedAt:   &claimedAt,
		},
	}

	pbGoal, err := GoalToProto(goal, userProgress)

	require.NoError(t, err)
	assert.Equal(t, string(domain.GoalStatusClaimed), pbGoal.Status)
	assert.NotEmpty(t, pbGoal.CompletedAt)
	assert.NotEmpty(t, pbGoal.ClaimedAt)
}

func TestGoalToProto_WithPrerequisites(t *testing.T) {
	goal := &domain.Goal{
		ID:          "goal-2",
		Name:        "Goal 2",
		Description: "Second goal",
		Type:        domain.GoalTypeAbsolute,
		Requirement: domain.Requirement{
			StatCode:    "level",
			Operator:    ">=",
			TargetValue: 5,
		},
		Reward: domain.Reward{
			Type:     string(domain.RewardTypeWallet),
			RewardID: "GOLD",
			Quantity: 100,
		},
		Prerequisites: []string{"goal-1"},
	}

	pbGoal, err := GoalToProto(goal, map[string]*domain.UserGoalProgress{})

	require.NoError(t, err)
	assert.Len(t, pbGoal.Prerequisites, 1)
	assert.Equal(t, "goal-1", pbGoal.Prerequisites[0])
	assert.True(t, pbGoal.Locked) // Has prerequisites but no progress
}
