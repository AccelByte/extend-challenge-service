package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/cache"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGoalCache is a mock implementation of cache.GoalCache
type MockGoalCache struct {
	mock.Mock
}

// Compile-time interface check
var _ cache.GoalCache = (*MockGoalCache)(nil)

func (m *MockGoalCache) GetGoalByID(goalID string) *domain.Goal {
	args := m.Called(goalID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*domain.Goal)
}

func (m *MockGoalCache) GetGoalsByStatCode(statCode string) []*domain.Goal {
	args := m.Called(statCode)
	return args.Get(0).([]*domain.Goal)
}

func (m *MockGoalCache) GetChallengeByChallengeID(challengeID string) *domain.Challenge {
	args := m.Called(challengeID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*domain.Challenge)
}

func (m *MockGoalCache) GetAllChallenges() []*domain.Challenge {
	args := m.Called()
	return args.Get(0).([]*domain.Challenge)
}

func (m *MockGoalCache) GetAllGoals() []*domain.Goal {
	args := m.Called()
	return args.Get(0).([]*domain.Goal)
}

func (m *MockGoalCache) GetGoalsWithDefaultAssigned() []*domain.Goal {
	args := m.Called()
	return args.Get(0).([]*domain.Goal)
}

func (m *MockGoalCache) Reload() error {
	args := m.Called()
	return args.Error(0)
}

// MockGoalRepository is a mock implementation of repository.GoalRepository
type MockGoalRepository struct {
	mock.Mock
}

// Compile-time interface check
var _ repository.GoalRepository = (*MockGoalRepository)(nil)

func (m *MockGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*domain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGoalRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string,
	delta, targetValue int, isDailyIncrement bool) error {
	args := m.Called(ctx, userID, goalID, challengeID, namespace, delta, targetValue, isDailyIncrement)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchIncrementProgress(ctx context.Context, increments []repository.ProgressIncrement) error {
	args := m.Called(ctx, increments)
	return args.Error(0)
}

func (m *MockGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockGoalRepository) BeginTx(ctx context.Context) (repository.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.TxRepository), args.Error(1)
}

// M3: Goal assignment control methods
func (m *MockGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGoalRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// Test Fixtures
func createTestChallenge(id, name string, goalCount int) *domain.Challenge {
	goals := make([]*domain.Goal, 0, goalCount)
	for i := 0; i < goalCount; i++ {
		goals = append(goals, createTestGoal(
			id+"-goal-"+string(rune('0'+i)),
			"Goal "+string(rune('A'+i)),
			id,
		))
	}

	return &domain.Challenge{
		ID:          id,
		Name:        name,
		Description: "Test challenge",
		Goals:       goals,
	}
}

func createTestGoal(id, name, challengeID string) *domain.Goal {
	return &domain.Goal{
		ID:          id,
		Name:        name,
		Description: "Test goal",
		ChallengeID: challengeID,
		Type:        domain.GoalTypeAbsolute,
		EventSource: domain.EventSourceStatistic,
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
}

func createTestProgress(userID, goalID, challengeID string, progress int, status domain.GoalStatus) *domain.UserGoalProgress {
	now := time.Now().UTC()
	return &domain.UserGoalProgress{
		UserID:      userID,
		GoalID:      goalID,
		ChallengeID: challengeID,
		Namespace:   "test-namespace",
		Progress:    progress,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Test GetUserChallengesWithProgress

func TestGetUserChallengesWithProgress_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	challenge1 := createTestChallenge("challenge-1", "Challenge 1", 2)
	challenge2 := createTestChallenge("challenge-2", "Challenge 2", 3)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{challenge1, challenge2})

	userProgress := []*domain.UserGoalProgress{
		createTestProgress(userID, "challenge-1-goal-0", "challenge-1", 5, domain.GoalStatusInProgress),
		createTestProgress(userID, "challenge-2-goal-0", "challenge-2", 10, domain.GoalStatusCompleted),
	}
	mockRepo.On("GetUserProgress", ctx, userID, false).Return(userProgress, nil)

	result, err := GetUserChallengesWithProgress(ctx, userID, namespace, mockCache, mockRepo, false)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "challenge-1", result[0].Challenge.ID)
	assert.Equal(t, "challenge-2", result[1].Challenge.ID)
	assert.Len(t, result[0].UserProgress, 2)
	assert.Equal(t, 5, result[0].UserProgress["challenge-1-goal-0"].Progress)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestGetUserChallengesWithProgress_NoChallenges(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{})

	result, err := GetUserChallengesWithProgress(ctx, userID, namespace, mockCache, mockRepo, false)

	require.NoError(t, err)
	assert.Empty(t, result)

	mockCache.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "GetUserProgress")
}

func TestGetUserChallengesWithProgress_NoUserProgress(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	challenge1 := createTestChallenge("challenge-1", "Challenge 1", 2)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{challenge1})
	mockRepo.On("GetUserProgress", ctx, userID, false).Return([]*domain.UserGoalProgress{}, nil)

	result, err := GetUserChallengesWithProgress(ctx, userID, namespace, mockCache, mockRepo, false)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Empty(t, result[0].UserProgress)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestGetUserChallengesWithProgress_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengesWithProgress(ctx, "", namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestGetUserChallengesWithProgress_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	userID := "user123"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengesWithProgress(ctx, userID, "", mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

func TestGetUserChallengesWithProgress_NilCache(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengesWithProgress(ctx, userID, namespace, nil, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal cache cannot be nil")
}

func TestGetUserChallengesWithProgress_NilRepository(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)

	_, err := GetUserChallengesWithProgress(ctx, userID, namespace, mockCache, nil, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

func TestGetUserChallengesWithProgress_RepositoryError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	challenge1 := createTestChallenge("challenge-1", "Challenge 1", 2)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{challenge1})
	mockRepo.On("GetUserProgress", ctx, userID, false).Return([]*domain.UserGoalProgress{}, errors.New("database error"))

	_, err := GetUserChallengesWithProgress(ctx, userID, namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load user progress")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test GetUserChallengeWithProgress

func TestGetUserChallengeWithProgress_Success(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	challenge := createTestChallenge(challengeID, "Challenge 1", 3)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)

	challengeProgress := []*domain.UserGoalProgress{
		createTestProgress(userID, "challenge-1-goal-0", challengeID, 7, domain.GoalStatusInProgress),
		createTestProgress(userID, "challenge-1-goal-1", challengeID, 10, domain.GoalStatusCompleted),
	}
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return(challengeProgress, nil)

	result, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, mockCache, mockRepo, false)

	require.NoError(t, err)
	assert.Equal(t, challengeID, result.Challenge.ID)
	assert.Len(t, result.UserProgress, 2)
	assert.Equal(t, 7, result.UserProgress["challenge-1-goal-0"].Progress)
	assert.Equal(t, 10, result.UserProgress["challenge-1-goal-1"].Progress)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestGetUserChallengeWithProgress_ChallengeNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "nonexistent"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(nil)

	_, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "challenge not found")

	mockCache.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "GetChallengeProgress")
}

func TestGetUserChallengeWithProgress_NoProgress(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	challenge := createTestChallenge(challengeID, "Challenge 1", 2)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress{}, nil)

	result, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, mockCache, mockRepo, false)

	require.NoError(t, err)
	assert.Equal(t, challengeID, result.Challenge.ID)
	assert.Empty(t, result.UserProgress)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestGetUserChallengeWithProgress_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	challengeID := "challenge-1"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengeWithProgress(ctx, "", challengeID, namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestGetUserChallengeWithProgress_EmptyChallengeID(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengeWithProgress(ctx, userID, "", namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "challenge ID cannot be empty")
}

func TestGetUserChallengeWithProgress_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengeWithProgress(ctx, userID, challengeID, "", mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace cannot be empty")
}

func TestGetUserChallengeWithProgress_NilCache(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	mockRepo := new(MockGoalRepository)

	_, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, nil, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal cache cannot be nil")
}

func TestGetUserChallengeWithProgress_NilRepository(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	mockCache := new(MockGoalCache)

	_, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, mockCache, nil, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository cannot be nil")
}

func TestGetUserChallengeWithProgress_RepositoryError(t *testing.T) {
	ctx := context.Background()
	userID := "user123"
	challengeID := "challenge-1"
	namespace := "test-namespace"

	challenge := createTestChallenge(challengeID, "Challenge 1", 2)

	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)

	mockCache.On("GetChallengeByChallengeID", challengeID).Return(challenge)
	mockRepo.On("GetChallengeProgress", ctx, userID, challengeID, false).Return([]*domain.UserGoalProgress{}, errors.New("database error"))

	_, err := GetUserChallengeWithProgress(ctx, userID, challengeID, namespace, mockCache, mockRepo, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load challenge progress")

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Test buildProgressMap

func TestBuildProgressMap_Success(t *testing.T) {
	progress := []*domain.UserGoalProgress{
		createTestProgress("user123", "goal-1", "challenge-1", 5, domain.GoalStatusInProgress),
		createTestProgress("user123", "goal-2", "challenge-1", 10, domain.GoalStatusCompleted),
		createTestProgress("user123", "goal-3", "challenge-2", 3, domain.GoalStatusInProgress),
	}

	progressMap := buildProgressMap(progress)

	assert.Len(t, progressMap, 3)
	assert.Equal(t, 5, progressMap["goal-1"].Progress)
	assert.Equal(t, 10, progressMap["goal-2"].Progress)
	assert.Equal(t, 3, progressMap["goal-3"].Progress)
}

func TestBuildProgressMap_EmptySlice(t *testing.T) {
	progressMap := buildProgressMap([]*domain.UserGoalProgress{})

	assert.Empty(t, progressMap)
}

func TestBuildProgressMap_Duplicates(t *testing.T) {
	// Last one wins in case of duplicates
	progress := []*domain.UserGoalProgress{
		createTestProgress("user123", "goal-1", "challenge-1", 5, domain.GoalStatusInProgress),
		createTestProgress("user123", "goal-1", "challenge-1", 10, domain.GoalStatusCompleted),
	}

	progressMap := buildProgressMap(progress)

	assert.Len(t, progressMap, 1)
	assert.Equal(t, 10, progressMap["goal-1"].Progress)
}
