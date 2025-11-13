package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"extend-challenge-service/pkg/mapper"

	"github.com/AccelByte/extend-challenge-common/pkg/client"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRewardClient is a mock implementation of client.RewardClient
type MockRewardClient struct {
	mock.Mock
}

func (m *MockRewardClient) GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error {
	args := m.Called(ctx, namespace, userID, itemID, quantity)
	return args.Error(0)
}

func (m *MockRewardClient) GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error {
	args := m.Called(ctx, namespace, userID, currencyCode, amount)
	return args.Error(0)
}

func (m *MockRewardClient) GrantReward(ctx context.Context, namespace, userID string, reward domain.Reward) error {
	args := m.Called(ctx, namespace, userID, reward)
	return args.Error(0)
}

// MockTxRepository is a mock implementation of repository.TxRepository
type MockTxRepository struct {
	mock.Mock
}

// Compile-time interface check
var _ repository.TxRepository = (*MockTxRepository)(nil)

func (m *MockTxRepository) GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockTxRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockTxRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockTxRepository) BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockTxRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*domain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockTxRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string,
	delta, targetValue int, isDailyIncrement bool) error {
	args := m.Called(ctx, userID, goalID, challengeID, namespace, delta, targetValue, isDailyIncrement)
	return args.Error(0)
}

func (m *MockTxRepository) BatchIncrementProgress(ctx context.Context, increments []repository.ProgressIncrement) error {
	args := m.Called(ctx, increments)
	return args.Error(0)
}

func (m *MockTxRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockTxRepository) BeginTx(ctx context.Context) (repository.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.TxRepository), args.Error(1)
}

func (m *MockTxRepository) GetProgressForUpdate(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxRepository) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTxRepository) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// M3: Goal assignment control methods
func (m *MockTxRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxRepository) BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

// M3 Phase 9: Fast path optimization methods
func (m *MockTxRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockTxRepository) GetActiveGoals(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// Test Fixtures

func createClaimableGoal(goalID, challengeID string) *domain.Goal {
	return &domain.Goal{
		ID:            goalID,
		Name:          "Claimable Goal",
		Description:   "Test goal",
		ChallengeID:   challengeID,
		Type:          domain.GoalTypeAbsolute,
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
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
	}
}

func createCompletedProgress(userID, goalID, challengeID string) *domain.UserGoalProgress {
	now := time.Now().UTC()
	return &domain.UserGoalProgress{
		UserID:      userID,
		GoalID:      goalID,
		ChallengeID: challengeID,
		Namespace:   "test-namespace",
		Progress:    10,
		Status:      domain.GoalStatusCompleted,
		CompletedAt: &now,
		IsActive:    true, // M3 Phase 6: Default to active
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Test ClaimGoalReward - Success

func TestClaimGoalReward_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(nil)
	mockTxRepo.On("MarkAsClaimed", mock.Anything, userID, goalID).Return(nil)
	mockTxRepo.On("Commit").Return(nil)

	result, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	require.NoError(t, err)
	assert.Equal(t, goalID, result.GoalID)
	assert.Equal(t, string(domain.GoalStatusClaimed), result.Status)
	assert.Equal(t, goal.Reward, result.Reward)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, challengeID, result.ChallengeID)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockRewardClient.AssertExpectations(t)
}

// Test ClaimGoalReward - Validation Errors

func TestClaimGoalReward_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "", "goal-1", "challenge-1", "namespace", mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestClaimGoalReward_EmptyGoalID(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "user123", "", "challenge-1", "namespace", mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal ID cannot be empty")
}

func TestClaimGoalReward_EmptyChallengeID(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "user123", "goal-1", "", "namespace", mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "challenge ID cannot be empty")
}

func TestClaimGoalReward_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "user123", "goal-1", "challenge-1", "", mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

func TestClaimGoalReward_NilCache(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "user123", "goal-1", "challenge-1", "namespace", nil, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal cache cannot be nil")
}

func TestClaimGoalReward_NilRepository(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRewardClient := new(MockRewardClient)

	_, err := ClaimGoalReward(ctx, "user123", "goal-1", "challenge-1", "namespace", mockCache, nil, mockRewardClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

func TestClaimGoalReward_NilRewardClient(t *testing.T) {
	ctx := context.Background()
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := ClaimGoalReward(ctx, "user123", "goal-1", "challenge-1", "namespace", mockCache, mockRepo, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reward client cannot be nil")
}

// Test ClaimGoalReward - Goal Not Found

func TestClaimGoalReward_GoalNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "nonexistent"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(nil)

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var goalNotFoundErr *mapper.GoalNotFoundError
	assert.True(t, errors.As(err, &goalNotFoundErr))
	assert.Equal(t, goalID, goalNotFoundErr.GoalID)
	assert.Equal(t, challengeID, goalNotFoundErr.ChallengeID)

	mockCache.AssertExpectations(t)
}

func TestClaimGoalReward_GoalWrongChallenge(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "wrong-challenge"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, "challenge-1")

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var goalNotFoundErr *mapper.GoalNotFoundError
	assert.True(t, errors.As(err, &goalNotFoundErr))

	mockCache.AssertExpectations(t)
}

// Test ClaimGoalReward - Transaction Errors

func TestClaimGoalReward_TransactionStartFailed(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(nil, errors.New("database error"))

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Equal(t, mapper.ErrDatabaseError, err)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test ClaimGoalReward - Progress Errors

func TestClaimGoalReward_ProgressNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(nil, nil)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var goalNotCompletedErr *mapper.GoalNotCompletedError
	assert.True(t, errors.As(err, &goalNotCompletedErr))
	assert.Equal(t, goalID, goalNotCompletedErr.GoalID)
	assert.Equal(t, string(domain.GoalStatusNotStarted), goalNotCompletedErr.Status)
}

func TestClaimGoalReward_GoalNotCompleted(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)
	progress.Status = domain.GoalStatusInProgress

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var goalNotCompletedErr *mapper.GoalNotCompletedError
	assert.True(t, errors.As(err, &goalNotCompletedErr))
	assert.Equal(t, goalID, goalNotCompletedErr.GoalID)
	assert.Equal(t, string(domain.GoalStatusInProgress), goalNotCompletedErr.Status)
}

func TestClaimGoalReward_AlreadyClaimed(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)
	progress.Status = domain.GoalStatusClaimed
	claimedAt := time.Now().UTC()
	progress.ClaimedAt = &claimedAt

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var alreadyClaimedErr *mapper.GoalAlreadyClaimedError
	assert.True(t, errors.As(err, &alreadyClaimedErr))
	assert.Equal(t, goalID, alreadyClaimedErr.GoalID)
}

// M3 Phase 6: Test claim validation with inactive goal
func TestClaimGoalReward_GoalNotActive(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)
	progress.IsActive = false // Goal is completed but inactive

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var goalNotActiveErr *mapper.GoalNotActiveError
	assert.True(t, errors.As(err, &goalNotActiveErr))
	assert.Equal(t, goalID, goalNotActiveErr.GoalID)
	assert.Equal(t, challengeID, goalNotActiveErr.ChallengeID)
}

// Test ClaimGoalReward - Prerequisites

func TestClaimGoalReward_PrerequisitesNotMet(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-2"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	goal.Prerequisites = []string{"goal-1"}

	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var prereqsNotMetErr *mapper.PrerequisitesNotMetError
	assert.True(t, errors.As(err, &prereqsNotMetErr))
	assert.Equal(t, goalID, prereqsNotMetErr.GoalID)
	assert.Contains(t, prereqsNotMetErr.MissingGoalIDs, "goal-1")
}

// Test ClaimGoalReward - Reward Grant Errors

func TestClaimGoalReward_RewardGrantFailed(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(errors.New("AGS error"))
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var rewardGrantErr *mapper.RewardGrantError
	assert.True(t, errors.As(err, &rewardGrantErr))
	assert.Equal(t, goalID, rewardGrantErr.GoalID)

	// Should have attempted 4 times (1 initial + 3 retries)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 4)
}

func TestClaimGoalReward_RewardGrantSuccessAfterRetry(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)

	// Fail first 2 attempts, succeed on 3rd
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).
		Return(errors.New("temporary error")).Once()
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).
		Return(errors.New("temporary error")).Once()
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).
		Return(nil).Once()

	mockTxRepo.On("MarkAsClaimed", mock.Anything, userID, goalID).Return(nil)
	mockTxRepo.On("Commit").Return(nil)

	result, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	require.NoError(t, err)
	assert.Equal(t, goalID, result.GoalID)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 3)
}

// Test ClaimGoalReward - Database Errors After Reward Grant

func TestClaimGoalReward_MarkAsClaimedFailed(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(nil)
	mockTxRepo.On("MarkAsClaimed", mock.Anything, userID, goalID).Return(errors.New("database error"))
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Equal(t, mapper.ErrDatabaseError, err)
}

func TestClaimGoalReward_CommitFailed(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(nil)
	mockTxRepo.On("MarkAsClaimed", mock.Anything, userID, goalID).Return(nil)
	mockTxRepo.On("Commit").Return(errors.New("commit failed"))

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	assert.Equal(t, mapper.ErrDatabaseError, err)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockRewardClient.AssertExpectations(t)
}

// Test Error Classification - Non-Retryable Errors

func TestClaimGoalReward_NonRetryableError_BadRequest(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return non-retryable error (bad request)
	badRequestErr := &client.BadRequestError{Message: "invalid item ID"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(badRequestErr).Once()
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)
	var rewardGrantErr *mapper.RewardGrantError
	assert.True(t, errors.As(err, &rewardGrantErr))

	// Should only attempt once (no retries for non-retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

func TestClaimGoalReward_NonRetryableError_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return non-retryable error (not found)
	notFoundErr := &client.NotFoundError{Resource: "item_sword"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(notFoundErr).Once()
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should only attempt once (no retries for non-retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

func TestClaimGoalReward_NonRetryableError_Forbidden(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return non-retryable error (forbidden)
	forbiddenErr := &client.ForbiddenError{Message: "namespace mismatch"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(forbiddenErr).Once()
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should only attempt once (no retries for non-retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

func TestClaimGoalReward_NonRetryableError_Authentication(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return non-retryable error (authentication)
	authErr := &client.AuthenticationError{Message: "invalid token"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(authErr).Once()
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should only attempt once (no retries for non-retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

func TestClaimGoalReward_NonRetryableError_MessagePattern(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return non-retryable error with pattern match
	patternErr := errors.New("invalid currency code: XYZ not found")

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(patternErr).Once()
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should only attempt once (no retries - "not found" pattern matches)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 1)
}

// Test Retryable Error with HTTP Status Code

func TestClaimGoalReward_RetryableError_BadGateway(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return retryable error (502 Bad Gateway)
	badGatewayErr := &client.AGSError{StatusCode: 502, Message: "bad gateway"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(badGatewayErr)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should attempt 4 times (1 initial + 3 retries for retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 4)
}

func TestClaimGoalReward_RetryableError_ServiceUnavailable(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	goalID := "goal-1"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	goal := createClaimableGoal(goalID, challengeID)
	progress := createCompletedProgress(userID, goalID, challengeID)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxRepository)
	mockRewardClient := new(MockRewardClient)

	// Return retryable error (503 Service Unavailable)
	serviceUnavailableErr := &client.AGSError{StatusCode: 503, Message: "service unavailable"}

	mockCache.On("GetGoalByID", goalID).Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, userID, goalID).Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, userID, false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, namespace, userID, goal.Reward).Return(serviceUnavailableErr)
	mockTxRepo.On("Rollback").Return(nil).Maybe()

	_, err := ClaimGoalReward(ctx, userID, goalID, challengeID, namespace, mockCache, mockRepo, mockRewardClient)

	assert.Error(t, err)

	// Should attempt 4 times (1 initial + 3 retries for retryable errors)
	mockRewardClient.AssertNumberOfCalls(t, "GrantReward", 4)
}
