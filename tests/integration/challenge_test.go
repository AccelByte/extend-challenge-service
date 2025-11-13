package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"

	pb "extend-challenge-service/pkg/pb"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// createAuthContext creates a context with user ID and namespace in gRPC metadata
// The test auth interceptor will extract these and inject into context
func createAuthContext(userID, namespace string) context.Context {
	md := metadata.Pairs(
		"user-id", userID,
		"namespace", namespace,
	)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// TestGetUserChallenges_EmptyProgress tests getting challenges with no user progress
func TestGetUserChallenges_EmptyProgress(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// No seeded data - user has no progress

	// Get challenges
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.GetChallengesRequest{}

	resp, err := client.GetUserChallenges(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Should return all challenges from test config (winter-challenge-2025 + daily-quests)
	assert.Len(t, resp.Challenges, 2)

	// Verify challenges are present
	winterChallenge := findChallenge(resp.Challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)
	assert.Equal(t, "Winter Challenge 2025", winterChallenge.Name)
	assert.Len(t, winterChallenge.Goals, 3) // complete-tutorial, kill-10-snowmen, reach-level-5

	dailyQuests := findChallenge(resp.Challenges, "daily-quests")
	assert.NotNil(t, dailyQuests)
	assert.Equal(t, "Daily Quests", dailyQuests.Name)
	assert.Len(t, dailyQuests.Goals, 2) // login-today, play-3-matches

	// Verify MockRewardClient was not called
	mockRewardClient.AssertNotCalled(t, "GrantReward")
	mockRewardClient.AssertNotCalled(t, "GrantItemReward")
	mockRewardClient.AssertNotCalled(t, "GrantWalletReward")
}

// TestGetUserChallenges_WithProgress tests getting challenges with user progress
func TestGetUserChallenges_WithProgress(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Get challenges
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.GetChallengesRequest{}

	resp, err := client.GetUserChallenges(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Challenges, 2) // winter-challenge-2025 + daily-quests

	// Find winter-challenge-2025
	winterChallenge := findChallenge(resp.Challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)
	assert.Len(t, winterChallenge.Goals, 3) // complete-tutorial + kill-10-snowmen + reach-level-5

	// Verify completed goal
	completedGoal := findGoal(winterChallenge.Goals, "kill-10-snowmen")
	assert.NotNil(t, completedGoal)
	assert.Equal(t, "completed", completedGoal.Status)
	assert.Equal(t, int32(10), completedGoal.Progress)
	assert.NotNil(t, completedGoal.Requirement)
	assert.Equal(t, int32(10), completedGoal.Requirement.TargetValue)
	assert.NotEmpty(t, completedGoal.CompletedAt)

	// Verify MockRewardClient was not called (just querying, not claiming)
	mockRewardClient.AssertNotCalled(t, "GrantReward")
}

// TestClaimGoalReward_HappyPath tests successful reward claiming
func TestClaimGoalReward_HappyPath(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed prerequisite goal as claimed (kill-10-snowmen requires complete-tutorial)
	seedClaimedGoal(t, testDB, "test-user-123", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goal
	seedCompletedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting to succeed (ITEM reward: 767d2217abe241aab2245794761e9dc4, quantity 1)
	// Note: This UUID matches the rewardId in config/challenges.test.json for kill-10-snowmen goal
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"test-user-123",
		mock.MatchedBy(func(reward commonDomain.Reward) bool {
			return reward.Type == "ITEM" && reward.RewardID == "767d2217abe241aab2245794761e9dc4" && reward.Quantity == 1
		}),
	).Return(nil)

	// Claim reward
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	resp, err := client.ClaimGoalReward(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "claimed", resp.Status)
	assert.NotEmpty(t, resp.ClaimedAt)

	// Verify reward was granted
	mockRewardClient.AssertExpectations(t)

	// Verify database updated to claimed
	var status string
	err = testDB.QueryRow(
		"SELECT status FROM user_goal_progress WHERE user_id = $1 AND goal_id = $2",
		"test-user-123", "kill-10-snowmen",
	).Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "claimed", status)
}

// TestClaimGoalReward_Idempotency tests claiming an already-claimed goal
func TestClaimGoalReward_Idempotency(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed already-claimed goal
	seedClaimedGoal(t, testDB, "test-user-123", "kill-10-snowmen", "winter-challenge-2025")

	// Try to claim again (should fail)
	ctx := createAuthContext("test-user-123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}

	_, err := client.ClaimGoalReward(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been claimed")

	// Verify reward was NOT granted
	mockRewardClient.AssertNotCalled(t, "GrantReward")
	mockRewardClient.AssertNotCalled(t, "GrantItemReward")
	mockRewardClient.AssertNotCalled(t, "GrantWalletReward")
}

// TestClaimGoalReward_MultipleUsers tests isolation between users
func TestClaimGoalReward_MultipleUsers(t *testing.T) {
	client, mockRewardClient, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed prerequisite goals for both users (kill-10-snowmen requires complete-tutorial)
	seedClaimedGoal(t, testDB, "user-1", "complete-tutorial", "winter-challenge-2025")
	seedClaimedGoal(t, testDB, "user-2", "complete-tutorial", "winter-challenge-2025")

	// Seed completed goals for two different users
	seedCompletedGoal(t, testDB, "user-1", "kill-10-snowmen", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, "user-2", "kill-10-snowmen", "winter-challenge-2025")

	// Mock reward granting for both users
	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"user-1",
		mock.Anything,
	).Return(nil).Once()

	mockRewardClient.On("GrantReward",
		mock.Anything,
		"test-namespace",
		"user-2",
		mock.Anything,
	).Return(nil).Once()

	// User 1 claims
	ctx1 := createAuthContext("user-1", "test-namespace")
	req1 := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}
	resp1, err1 := client.ClaimGoalReward(ctx1, req1)
	assert.NoError(t, err1)
	assert.Equal(t, "claimed", resp1.Status)

	// User 2 can still claim (different user)
	ctx2 := createAuthContext("user-2", "test-namespace")
	req2 := &pb.ClaimRewardRequest{
		ChallengeId: "winter-challenge-2025",
		GoalId:      "kill-10-snowmen",
	}
	resp2, err2 := client.ClaimGoalReward(ctx2, req2)
	assert.NoError(t, err2)
	assert.Equal(t, "claimed", resp2.Status)

	// Verify both rewards were granted
	mockRewardClient.AssertExpectations(t)
}
