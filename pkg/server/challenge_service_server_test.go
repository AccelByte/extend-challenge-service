// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"extend-challenge-service/pkg/cleanup"
	"extend-challenge-service/pkg/common"
	pb "extend-challenge-service/pkg/pb"
	"extend-challenge-service/pkg/service"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Mock implementations
type MockGoalCache struct {
	mock.Mock
}

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

func (m *MockGoalCache) GetChallengeByChallengeID(challengeID string) *domain.Challenge {
	args := m.Called(challengeID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*domain.Challenge)
}

func (m *MockGoalCache) Reload() error {
	args := m.Called()
	return args.Error(0)
}

type MockGoalRepository struct {
	mock.Mock
}

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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgress(ctx context.Context, progressList []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progressList)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, rows []repository.CopyRow) error {
	args := m.Called(ctx, rows)
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

func (m *MockGoalRepository) BulkInsertWithCOPY(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGoalRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// M4: Batch goal activation
func (m *MockGoalRepository) BatchUpsertGoalActive(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

// M3 Phase 9: Fast path optimization methods
func (m *MockGoalRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockGoalRepository) GetActiveGoals(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) DeleteExpiredRows(ctx context.Context, namespace string, cutoff time.Time, batchSize int) (int64, error) {
	args := m.Called(ctx, namespace, cutoff, batchSize)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGoalRepository) DeleteUserData(ctx context.Context, namespace string, userID string) (int64, error) {
	args := m.Called(ctx, namespace, userID)
	return args.Get(0).(int64), args.Error(1)
}

type MockTxGoalRepository struct {
	mock.Mock
}

func (m *MockTxGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockTxGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockTxGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxGoalRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockTxGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockTxGoalRepository) BatchUpsertProgress(ctx context.Context, progressList []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progressList)
	return args.Error(0)
}

func (m *MockTxGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, rows []repository.CopyRow) error {
	args := m.Called(ctx, rows)
	return args.Error(0)
}

func (m *MockTxGoalRepository) GetProgressForUpdate(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxGoalRepository) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTxGoalRepository) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTxGoalRepository) BeginTx(ctx context.Context) (repository.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repository.TxRepository), args.Error(1)
}

// M3: Goal assignment control methods
func (m *MockTxGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxGoalRepository) BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockTxGoalRepository) BulkInsertWithCOPY(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockTxGoalRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// M4: Batch goal activation
func (m *MockTxGoalRepository) BatchUpsertGoalActive(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

// M3 Phase 9: Fast path optimization methods
func (m *MockTxGoalRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockTxGoalRepository) GetActiveGoals(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserGoalProgress), args.Error(1)
}

func (m *MockTxGoalRepository) DeleteExpiredRows(ctx context.Context, namespace string, cutoff time.Time, batchSize int) (int64, error) {
	args := m.Called(ctx, namespace, cutoff, batchSize)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTxGoalRepository) DeleteUserData(ctx context.Context, namespace string, userID string) (int64, error) {
	args := m.Called(ctx, namespace, userID)
	return args.Get(0).(int64), args.Error(1)
}

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

// Helper function to create authenticated context with user ID
// This simulates what the auth interceptor does after JWT validation
func createAuthContext(userID, namespace string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, common.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, common.ContextKeyNamespace, namespace)
	return ctx
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// Tests for extractUserIDFromContext
// Note: These tests verify context extraction, not JWT decoding
// JWT decoding is now handled by the auth interceptor (see authServerInterceptor_test.go)
func TestExtractUserIDFromContext_Success(t *testing.T) {
	ctx := createAuthContext("user123", "test-namespace")

	userID, err := extractUserIDFromContext(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "user123", userID)
}

func TestExtractUserIDFromContext_MissingUserID(t *testing.T) {
	ctx := context.Background() // No user ID in context

	userID, err := extractUserIDFromContext(ctx)

	assert.Error(t, err)
	assert.Equal(t, "", userID)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "user ID not found in context")
}

func TestExtractUserIDFromContext_EmptyUserID(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, common.ContextKeyUserID, "") // Empty user ID

	userID, err := extractUserIDFromContext(ctx)

	assert.Error(t, err)
	assert.Equal(t, "", userID)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "user ID not found in context")
}

// Tests for GetUserChallenges
func TestGetUserChallenges_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create test data
	challenge := &domain.Challenge{
		ID:          "challenge1",
		Name:        "Test Challenge",
		Description: "Test Description",
		Goals: []*domain.Goal{
			{
				ID:          "goal1",
				ChallengeID: "challenge1",
				Name:        "Test Goal",
				Description: "Test Goal Description",
				Requirement: domain.Requirement{
					StatCode:     "kills",
					Operator:     ">=",
					TargetValue:  10,
					ProgressMode: domain.ProgressModeAbsolute,
				},
				Reward: domain.Reward{
					Type:     "ITEM",
					RewardID: "sword",
					Quantity: 1,
				},
				EventSource:   domain.EventSourceStatistic,
				Prerequisites: []string{},
			},
		},
	}

	userProgress := []*domain.UserGoalProgress{
		{
			UserID:      "user123",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test-namespace",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{challenge})
	mockRepo.On("GetUserProgress", mock.Anything, "user123", false).Return(userProgress, nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetChallengesRequest{}

	resp, err := server.GetUserChallenges(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Challenges, 1)
	assert.Equal(t, "challenge1", resp.Challenges[0].ChallengeId)
	assert.Len(t, resp.Challenges[0].Goals, 1)
	assert.Equal(t, "goal1", resp.Challenges[0].Goals[0].GoalId)
	assert.Equal(t, int32(5), resp.Challenges[0].Goals[0].Progress)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestGetUserChallenges_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.GetChallengesRequest{}

	resp, err := server.GetUserChallenges(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetUserChallenges_ServiceError(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create a challenge so GetUserProgress is called
	challenge := &domain.Challenge{
		ID:          "challenge1",
		Name:        "Test Challenge",
		Description: "Test Description",
		Goals:       []*domain.Goal{},
	}

	mockCache.On("GetAllChallenges").Return([]*domain.Challenge{challenge})
	mockRepo.On("GetUserProgress", mock.Anything, "user123", false).Return(nil, errors.New("database error"))

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetChallengesRequest{}

	resp, err := server.GetUserChallenges(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Tests for InitializePlayer
func TestInitializePlayer_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create test data - goals with default_assigned = true
	defaultGoals := []*domain.Goal{
		{
			ID:              "daily-login",
			ChallengeID:     "daily-challenge",
			Name:            "Login Daily",
			Description:     "Login to the game",
			EventSource:     domain.EventSourceLogin,
			DefaultAssigned: true,
			Requirement: domain.Requirement{
				StatCode:     "login_count",
				Operator:     ">=",
				TargetValue:  1,
				ProgressMode: domain.ProgressModeAbsolute,
			},
			Reward: domain.Reward{
				Type:     "ITEM",
				RewardID: "login-bonus",
				Quantity: 1,
			},
		},
	}

	// Mock: No existing progress (new player)
	mockCache.On("GetGoalsWithDefaultAssigned").Return(defaultGoals)
	mockCache.On("GetGoalByID", "daily-login").Return(defaultGoals[0])
	// M3 Phase 9: Fast path check - user not initialized
	mockRepo.On("GetUserGoalCount", mock.Anything, "new-user").Return(0, nil)

	// Phase 10: No GetGoalsByIDs() call when count == 0 (optimization)

	mockRepo.On("BulkInsert", mock.Anything, mock.MatchedBy(func(progress []*domain.UserGoalProgress) bool {
		return len(progress) == 1 && progress[0].GoalID == "daily-login" && progress[0].IsActive
	})).Return(nil)

	// Phase 10: No GetGoalsByIDs() after insert (return created data directly)

	ctx := createAuthContext("new-user", "test-namespace")
	req := &pb.InitializeRequest{}

	resp, err := server.InitializePlayer(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.NewAssignments)
	assert.Equal(t, int32(1), resp.TotalActive)
	assert.Len(t, resp.AssignedGoals, 1)
	assert.Equal(t, "daily-login", resp.AssignedGoals[0].GoalId)
	assert.True(t, resp.AssignedGoals[0].IsActive)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestInitializePlayer_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background() // No auth context
	req := &pb.InitializeRequest{}

	resp, err := server.InitializePlayer(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestInitializePlayer_ServiceError(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Mock service error
	mockCache.On("GetGoalsWithDefaultAssigned").Return([]*domain.Goal{
		{
			ID:              "test-goal",
			ChallengeID:     "test-challenge",
			DefaultAssigned: true,
		},
	})
	// Phase 10: No GetGoalByID() mock needed - when BulkInsert fails, function returns early
	// without calling mapToAssignedGoals() which would use GetGoalByID()

	// M3 Phase 9: Fast path check - user not initialized
	mockRepo.On("GetUserGoalCount", mock.Anything, "user123").Return(0, nil)

	// Phase 10: No GetGoalsByIDs() call, test BulkInsert error instead
	mockRepo.On("BulkInsert", mock.Anything, mock.Anything).Return(errors.New("database error"))

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.InitializeRequest{}

	resp, err := server.InitializePlayer(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// Tests for SetGoalActive
func TestSetGoalActive_ActivateGoal(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create test goal
	goal := &domain.Goal{
		ID:              "optional-quest",
		ChallengeID:     "side-quests",
		Name:            "Optional Quest",
		Description:     "Complete optional quest",
		EventSource:     domain.EventSourceStatistic,
		DefaultAssigned: false,
		Requirement: domain.Requirement{
			StatCode:     "quests_completed",
			Operator:     ">=",
			TargetValue:  5,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "WALLET",
			RewardID: "gold",
			Quantity: 100,
		},
	}

	assignedAt := time.Now()

	mockCache.On("GetGoalByID", "optional-quest").Return(goal)
	mockRepo.On("GetProgress", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil) // No existing progress
	mockRepo.On("UpsertGoalActive", mock.Anything, mock.Anything).Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "side-quests",
		GoalId:      "optional-quest",
		IsActive:    true,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "side-quests", resp.ChallengeId)
	assert.Equal(t, "optional-quest", resp.GoalId)
	assert.True(t, resp.IsActive)
	assert.NotEmpty(t, resp.AssignedAt)
	assert.Contains(t, resp.Message, "activated")

	// Verify assigned_at is recent (within last 5 seconds)
	parsedTime, parseErr := time.Parse(time.RFC3339, resp.AssignedAt)
	assert.NoError(t, parseErr)
	assert.WithinDuration(t, assignedAt, parsedTime, 5*time.Second)
}

func TestSetGoalActive_DeactivateGoal(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	goal := &domain.Goal{
		ID:          "active-quest",
		ChallengeID: "daily-quests",
		Name:        "Active Quest",
	}

	existingProgress := &domain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "active-quest",
		ChallengeID: "daily-quests",
		Namespace:   "test-namespace",
		Progress:    3,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
		AssignedAt:  timePtr(time.Now().Add(-24 * time.Hour)),
	}

	mockCache.On("GetGoalByID", "active-quest").Return(goal)
	mockRepo.On("GetProgress", mock.Anything, mock.Anything, mock.Anything).Return(existingProgress, nil)
	mockRepo.On("UpsertGoalActive", mock.Anything, mock.Anything).Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "daily-quests",
		GoalId:      "active-quest",
		IsActive:    false,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "daily-quests", resp.ChallengeId)
	assert.Equal(t, "active-quest", resp.GoalId)
	assert.False(t, resp.IsActive)
	assert.Contains(t, resp.Message, "deactivated")
}

func TestSetGoalActive_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background() // No auth context
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
		IsActive:    true,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSetGoalActive_MissingChallengeID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "", // Missing
		GoalId:      "goal1",
		IsActive:    true,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "challenge_id")
}

func TestSetGoalActive_MissingGoalID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "challenge1",
		GoalId:      "", // Missing
		IsActive:    true,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "goal_id")
}

func TestSetGoalActive_ServiceError(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Mock service error
	mockCache.On("GetGoalByID", "goal1").Return(nil) // Goal not found

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.SetGoalActiveRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
		IsActive:    true,
	}

	resp, err := server.SetGoalActive(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))

	mockCache.AssertExpectations(t)
}

// Tests for ClaimGoalReward
func TestClaimGoalReward_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create test data
	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Test Goal",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  10,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "ITEM",
			RewardID: "sword",
			Quantity: 1,
		},
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
	}

	progress := &domain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal1",
		ChallengeID: "challenge1",
		Namespace:   "test-namespace",
		Progress:    10,
		Status:      domain.GoalStatusCompleted,
		IsActive:    true, // M3 Phase 6: Goal must be active to claim reward
		CompletedAt: func() *time.Time { t := time.Now(); return &t }(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockCache.On("GetGoalByID", "goal1").Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, "user123", "goal1").Return(progress, nil)
	mockTxRepo.On("GetUserProgress", mock.Anything, "user123", false).Return([]*domain.UserGoalProgress{progress}, nil)
	mockRewardClient.On("GrantReward", mock.Anything, "test-namespace", "user123", goal.Reward).Return(nil)
	mockTxRepo.On("MarkAsClaimed", mock.Anything, "user123", "goal1").Return(nil)
	mockTxRepo.On("Commit").Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "goal1", resp.GoalId)
	assert.Equal(t, "claimed", resp.Status)
	assert.Equal(t, "ITEM", resp.Reward.Type)
	assert.Equal(t, "sword", resp.Reward.RewardId)
	assert.Equal(t, int32(1), resp.Reward.Quantity)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
	mockRewardClient.AssertExpectations(t)
}

func TestClaimGoalReward_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestClaimGoalReward_MissingChallengeID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "", // Missing
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "challenge_id is required")
}

func TestClaimGoalReward_MissingGoalID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "", // Missing
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "goal_id is required")
}

func TestClaimGoalReward_GoalNotFound(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	mockCache.On("GetGoalByID", "goal1").Return(nil) // Goal not found

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))

	mockCache.AssertExpectations(t)
}

func TestClaimGoalReward_GoalNotCompleted(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Test Goal",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  10,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "ITEM",
			RewardID: "sword",
			Quantity: 1,
		},
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
	}

	progress := &domain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal1",
		ChallengeID: "challenge1",
		Namespace:   "test-namespace",
		Progress:    5, // Not completed
		Status:      domain.GoalStatusInProgress,
		IsActive:    true, // M3 Phase 6: Goal must be active to claim reward
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockCache.On("GetGoalByID", "goal1").Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, "user123", "goal1").Return(progress, nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestClaimGoalReward_AlreadyClaimed(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Test Goal",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  10,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "ITEM",
			RewardID: "sword",
			Quantity: 1,
		},
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
	}

	claimedTime := time.Now()
	progress := &domain.UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal1",
		ChallengeID: "challenge1",
		Namespace:   "test-namespace",
		Progress:    10,
		Status:      domain.GoalStatusClaimed,
		IsActive:    true, // M3 Phase 6: Goal must be active to claim reward
		ClaimedAt:   &claimedTime,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockCache.On("GetGoalByID", "goal1").Return(goal)
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	mockTxRepo.On("GetProgressForUpdate", mock.Anything, "user123", "goal1").Return(progress, nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.ClaimRewardRequest{
		ChallengeId: "challenge1",
		GoalId:      "goal1",
	}

	resp, err := server.ClaimGoalReward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

// Tests for HealthCheck
func TestHealthCheck_Healthy(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Expect ping
	mock.ExpectPing()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthCheck_DatabaseDown(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Expect ping to fail
	mock.ExpectPing().WillReturnError(errors.New("connection refused"))

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Contains(t, err.Error(), "database connectivity check failed")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthCheck_Timeout(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Simulate slow database by delaying ping
	mock.ExpectPing().WillDelayFor(3 * time.Second)

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestHealthCheck_StaleCleanupStatus(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, dbMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Expect ping to succeed
	dbMock.ExpectPing()

	// Create CleanupStatus with old creation time so startup grace is expired
	cleanupSt := cleanup.NewCleanupStatusWithCreatedAt(time.Now().Add(-24 * time.Hour))
	cleanupInterval := 60 * time.Minute

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", cleanupSt, cleanupInterval)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	// Should return unhealthy — cleanup goroutine is stale
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unavailable, status.Code(err))

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestHealthCheck_CleanupStartupGrace(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, dbMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Expect ping to succeed
	dbMock.ExpectPing()

	// Fresh CleanupStatus with no heartbeat — within startup grace period
	cleanupSt := cleanup.NewCleanupStatus()
	cleanupInterval := 60 * time.Minute

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", cleanupSt, cleanupInterval)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	// Should return healthy — within startup grace period
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestHealthCheck_HealthyCleanupStatus(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, dbMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Expect ping to succeed
	dbMock.ExpectPing()

	// Create CleanupStatus with a recent heartbeat
	cleanupSt := cleanup.NewCleanupStatus()
	cleanupSt.RecordHeartbeat()
	cleanupInterval := 60 * time.Minute

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", cleanupSt, cleanupInterval)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestHealthCheck_CleanupStatusNonNil_ZeroInterval(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, dbMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	dbMock.ExpectPing()

	// Non-nil cleanupStatus but zero interval — cleanup check should be skipped
	cleanupSt := cleanup.NewCleanupStatusWithCreatedAt(time.Now().Add(-24 * time.Hour))
	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", cleanupSt, 0)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	resp, err := server.HealthCheck(ctx, req)

	// Should return healthy — zero interval skips cleanup check even though status is stale
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestHealthCheck_CleanupRecoveryAfterHeartbeat(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, dbMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Stale status (old creation, no heartbeat) — first call should fail
	cleanupSt := cleanup.NewCleanupStatusWithCreatedAt(time.Now().Add(-24 * time.Hour))
	cleanupInterval := 60 * time.Minute
	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", cleanupSt, cleanupInterval)

	ctx := context.Background()
	req := &pb.HealthCheckRequest{}

	dbMock.ExpectPing()
	resp, err := server.HealthCheck(ctx, req)
	assert.Error(t, err, "should be unhealthy before heartbeat")
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unavailable, status.Code(err))

	// Record heartbeat — second call should succeed (recovery)
	cleanupSt.RecordHeartbeat()

	dbMock.ExpectPing()
	resp, err = server.HealthCheck(ctx, req)
	assert.NoError(t, err, "should be healthy after heartbeat")
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// ============================================================================
// Tests for BatchSelectGoals
// ============================================================================

func TestBatchSelectGoals_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create a challenge with one goal
	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Test Goal",
		Description: "Kill 10 enemies",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  10,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "ITEM",
			RewardID: "sword",
			Quantity: 1,
		},
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
	}

	challenge := &domain.Challenge{
		ID:          "challenge1",
		Name:        "Test Challenge",
		Description: "Test Description",
		Goals:       []*domain.Goal{goal},
	}

	// service.BatchSelectGoals calls:
	// 1. cache.GetChallengeByChallengeID
	mockCache.On("GetChallengeByChallengeID", "challenge1").Return(challenge)
	// 2. cache.GetGoalByID for each goalID (validation)
	mockCache.On("GetGoalByID", "goal1").Return(goal)
	// 3. repo.GetChallengeProgress
	mockRepo.On("GetChallengeProgress", mock.Anything, "user123", "challenge1", false).
		Return([]*domain.UserGoalProgress{}, nil)
	// 4. repo.BeginTx
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	// 5. tx.BatchUpsertGoalActive (activate selected goals)
	mockTxRepo.On("BatchUpsertGoalActive", mock.Anything, mock.Anything).Return(nil)
	// 6. tx.Commit
	mockTxRepo.On("Commit").Return(nil)
	// 7. tx.Rollback (deferred, expected after commit)
	mockTxRepo.On("Rollback").Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.BatchSelectRequest{
		ChallengeId:     "challenge1",
		GoalIds:         []string{"goal1"},
		ReplaceExisting: false,
	}

	resp, err := server.BatchSelectGoals(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "challenge1", resp.ChallengeId)
	assert.Len(t, resp.SelectedGoals, 1)
	assert.Equal(t, "goal1", resp.SelectedGoals[0].GoalId)
	assert.Equal(t, "Test Goal", resp.SelectedGoals[0].Name)
	assert.True(t, resp.SelectedGoals[0].IsActive)
	assert.Equal(t, int32(10), resp.SelectedGoals[0].Target)
	assert.Equal(t, int32(1), resp.TotalActiveGoals)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestBatchSelectGoals_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.BatchSelectRequest{
		ChallengeId: "challenge1",
		GoalIds:     []string{"goal1"},
	}

	resp, err := server.BatchSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestBatchSelectGoals_MissingChallengeID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.BatchSelectRequest{
		ChallengeId: "",
		GoalIds:     []string{"goal1"},
	}

	resp, err := server.BatchSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "challenge_id")
}

func TestBatchSelectGoals_EmptyGoalIds(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.BatchSelectRequest{
		ChallengeId: "challenge1",
		GoalIds:     []string{},
	}

	resp, err := server.BatchSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "goal_ids")
}

func TestBatchSelectGoals_ServiceError_NotFound(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Challenge not found in cache
	mockCache.On("GetChallengeByChallengeID", "nonexistent").Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.BatchSelectRequest{
		ChallengeId: "nonexistent",
		GoalIds:     []string{"goal1"},
	}

	resp, err := server.BatchSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))

	mockCache.AssertExpectations(t)
}

// ============================================================================
// Tests for RandomSelectGoals
// ============================================================================

func TestRandomSelectGoals_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockTxRepo := new(MockTxGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Create a challenge with one goal (deterministic random selection)
	goal := &domain.Goal{
		ID:          "goal1",
		ChallengeID: "challenge1",
		Name:        "Random Goal",
		Description: "A random goal",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  5,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "WALLET",
			RewardID: "gold",
			Quantity: 100,
		},
		EventSource:   domain.EventSourceStatistic,
		Prerequisites: []string{},
	}

	challenge := &domain.Challenge{
		ID:          "challenge1",
		Name:        "Random Challenge",
		Description: "Challenge with random selection",
		Goals:       []*domain.Goal{goal},
	}

	// service.RandomSelectGoals calls:
	// 1. cache.GetChallengeByChallengeID
	mockCache.On("GetChallengeByChallengeID", "challenge1").Return(challenge)
	// 2. repo.GetChallengeProgress (no prior progress)
	mockRepo.On("GetChallengeProgress", mock.Anything, "user123", "challenge1", false).
		Return([]*domain.UserGoalProgress{}, nil)
	// 3. repo.BeginTx
	mockRepo.On("BeginTx", mock.Anything).Return(mockTxRepo, nil)
	// 4. cache.GetGoalByID (for ExpiresAt calculation during batch build)
	mockCache.On("GetGoalByID", "goal1").Return(goal)
	// 5. tx.BatchUpsertGoalActive
	mockTxRepo.On("BatchUpsertGoalActive", mock.Anything, mock.Anything).Return(nil)
	// 6. tx.Commit
	mockTxRepo.On("Commit").Return(nil)
	// 7. tx.Rollback (deferred)
	mockTxRepo.On("Rollback").Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.RandomSelectRequest{
		ChallengeId:     "challenge1",
		Count:           1,
		ReplaceExisting: false,
		ExcludeActive:   false,
	}

	resp, err := server.RandomSelectGoals(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "challenge1", resp.ChallengeId)
	assert.Len(t, resp.SelectedGoals, 1)
	assert.Equal(t, "goal1", resp.SelectedGoals[0].GoalId)
	assert.Equal(t, "Random Goal", resp.SelectedGoals[0].Name)
	assert.True(t, resp.SelectedGoals[0].IsActive)
	assert.Equal(t, int32(1), resp.TotalActiveGoals)

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockTxRepo.AssertExpectations(t)
}

func TestRandomSelectGoals_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := context.Background()
	req := &pb.RandomSelectRequest{
		ChallengeId: "challenge1",
		Count:       1,
	}

	resp, err := server.RandomSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRandomSelectGoals_MissingChallengeID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.RandomSelectRequest{
		ChallengeId: "",
		Count:       1,
	}

	resp, err := server.RandomSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "challenge_id")
}

func TestRandomSelectGoals_InvalidCount(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.RandomSelectRequest{
		ChallengeId: "challenge1",
		Count:       0,
	}

	resp, err := server.RandomSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "count")
}

func TestRandomSelectGoals_InsufficientGoals(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Challenge with one goal that is already claimed
	goal := &domain.Goal{
		ID:            "goal1",
		ChallengeID:   "challenge1",
		Name:          "Claimed Goal",
		Prerequisites: []string{},
	}

	challenge := &domain.Challenge{
		ID:    "challenge1",
		Name:  "Test Challenge",
		Goals: []*domain.Goal{goal},
	}

	// All goals are claimed, so zero available after filtering
	claimedProgress := []*domain.UserGoalProgress{
		{
			UserID:      "user123",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test-namespace",
			Status:      domain.GoalStatusClaimed,
			IsActive:    true,
		},
	}

	mockCache.On("GetChallengeByChallengeID", "challenge1").Return(challenge)
	mockRepo.On("GetChallengeProgress", mock.Anything, "user123", "challenge1", false).
		Return(claimedProgress, nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.RandomSelectRequest{
		ChallengeId: "challenge1",
		Count:       1,
	}

	resp, err := server.RandomSelectGoals(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// ============================================================================
// Tests for GetRotationStatus
// ============================================================================

func TestGetRotationStatus_MissingChallengeID(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetRotationStatusRequest{
		ChallengeId: "",
	}

	resp, err := server.GetRotationStatus(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "challenge_id")
}

func TestGetRotationStatus_ChallengeNotFound(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	mockCache.On("GetChallengeByChallengeID", "nonexistent").Return(nil)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetRotationStatusRequest{
		ChallengeId: "nonexistent",
	}

	resp, err := server.GetRotationStatus(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))

	mockCache.AssertExpectations(t)
}

func TestGetRotationStatus_NoRotation(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Challenge with goals that have no rotation config
	challenge := &domain.Challenge{
		ID:   "challenge1",
		Name: "No Rotation Challenge",
		Goals: []*domain.Goal{
			{
				ID:          "goal1",
				ChallengeID: "challenge1",
				Name:        "Static Goal",
				Rotation:    nil, // No rotation
			},
		},
	}

	mockCache.On("GetChallengeByChallengeID", "challenge1").Return(challenge)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetRotationStatusRequest{
		ChallengeId: "challenge1",
	}

	resp, err := server.GetRotationStatus(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "challenge1", resp.ChallengeId)
	assert.NotNil(t, resp.Rotation)
	assert.False(t, resp.Rotation.Enabled)

	mockCache.AssertExpectations(t)
}

func TestGetRotationStatus_WithRotation(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// Challenge with a goal that has daily rotation enabled
	challenge := &domain.Challenge{
		ID:   "challenge1",
		Name: "Rotating Challenge",
		Goals: []*domain.Goal{
			{
				ID:          "goal1",
				ChallengeID: "challenge1",
				Name:        "Daily Goal",
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Type:     domain.RotationTypeGlobal,
					Schedule: domain.RotationScheduleDaily,
					OnExpiry: domain.OnExpiryConfig{
						ResetProgress:    true,
						AllowReselection: false,
					},
				},
			},
		},
	}

	mockCache.On("GetChallengeByChallengeID", "challenge1").Return(challenge)

	ctx := createAuthContext("user123", "test-namespace")
	req := &pb.GetRotationStatusRequest{
		ChallengeId: "challenge1",
	}

	resp, err := server.GetRotationStatus(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "challenge1", resp.ChallengeId)
	assert.NotNil(t, resp.Rotation)
	assert.True(t, resp.Rotation.Enabled)
	assert.Equal(t, "global", resp.Rotation.Type)
	assert.Equal(t, "daily", resp.Rotation.Schedule)
	assert.NotNil(t, resp.Rotation.CurrentPeriod)
	assert.NotEmpty(t, resp.Rotation.CurrentPeriod.StartTime)
	assert.NotEmpty(t, resp.Rotation.CurrentPeriod.EndTime)
	assert.Greater(t, resp.Rotation.CurrentPeriod.ExpiresInSeconds, int32(0))
	assert.NotNil(t, resp.Rotation.NextPeriod)
	assert.NotEmpty(t, resp.Rotation.NextPeriod.StartTime)
	assert.NotEmpty(t, resp.Rotation.NextPeriod.EndTime)

	mockCache.AssertExpectations(t)
}

// ============================================================================
// Tests for buildRotationInfo
// ============================================================================

func TestBuildRotationInfo_NilConfig(t *testing.T) {
	now := time.Now().UTC()
	info := buildRotationInfo(nil, now)

	assert.NotNil(t, info)
	assert.False(t, info.Enabled)
	assert.Empty(t, info.Type)
	assert.Empty(t, info.Schedule)
	assert.Nil(t, info.CurrentPeriod)
	assert.Nil(t, info.NextPeriod)
}

func TestBuildRotationInfo_DailySchedule(t *testing.T) {
	cfg := &domain.RotationConfig{
		Enabled:  true,
		Type:     domain.RotationTypeGlobal,
		Schedule: domain.RotationScheduleDaily,
		OnExpiry: domain.OnExpiryConfig{
			ResetProgress: true,
		},
	}

	// Use a fixed time: 2026-03-01 12:00:00 UTC (noon)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	info := buildRotationInfo(cfg, now)

	assert.NotNil(t, info)
	assert.True(t, info.Enabled)
	assert.Equal(t, "global", info.Type)
	assert.Equal(t, "daily", info.Schedule)

	// Current period should be 2026-03-01 00:00 to 2026-03-02 00:00
	assert.NotNil(t, info.CurrentPeriod)
	expectedStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedStart.Format(time.RFC3339), info.CurrentPeriod.StartTime)
	assert.Equal(t, expectedEnd.Format(time.RFC3339), info.CurrentPeriod.EndTime)

	// ExpiresInSeconds should be 12 hours (43200 seconds)
	assert.Equal(t, int32(43200), info.CurrentPeriod.ExpiresInSeconds)

	// Next period should be 2026-03-02 00:00 to 2026-03-03 00:00
	assert.NotNil(t, info.NextPeriod)
	expectedNextStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	expectedNextEnd := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedNextStart.Format(time.RFC3339), info.NextPeriod.StartTime)
	assert.Equal(t, expectedNextEnd.Format(time.RFC3339), info.NextPeriod.EndTime)
}

// ============================================================================
// Tests for selectedGoalToProto
// ============================================================================

func TestSelectedGoalToProto_NilGoal(t *testing.T) {
	result, err := selectedGoalToProto(nil)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestSelectedGoalToProto_FullConversion(t *testing.T) {
	assignedAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	goal := &service.SelectedGoalInfo{
		GoalID:      "goal1",
		Name:        "Kill Enemies",
		Description: "Kill 10 enemies in battle",
		Requirement: domain.Requirement{
			StatCode:     "kills",
			Operator:     ">=",
			TargetValue:  10,
			ProgressMode: domain.ProgressModeAbsolute,
		},
		Reward: domain.Reward{
			Type:     "ITEM",
			RewardID: "sword",
			Quantity: 1,
		},
		Status:     "not_started",
		Progress:   3,
		Target:     10,
		IsActive:   true,
		AssignedAt: &assignedAt,
		ExpiresAt:  &expiresAt,
	}

	result, err := selectedGoalToProto(goal)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "goal1", result.GoalId)
	assert.Equal(t, "Kill Enemies", result.Name)
	assert.Equal(t, "Kill 10 enemies in battle", result.Description)
	assert.Equal(t, "not_started", result.Status)
	assert.Equal(t, int32(3), result.Progress)
	assert.Equal(t, int32(10), result.Target)
	assert.True(t, result.IsActive)
	assert.Equal(t, assignedAt.Format(time.RFC3339), result.AssignedAt)
	assert.Equal(t, expiresAt.Format(time.RFC3339), result.ExpiresAt)

	// Verify requirement was converted
	assert.NotNil(t, result.Requirement)
	assert.Equal(t, "kills", result.Requirement.StatCode)
	assert.Equal(t, ">=", result.Requirement.Operator)
	assert.Equal(t, int32(10), result.Requirement.TargetValue)

	// Verify reward was converted
	assert.NotNil(t, result.Reward)
	assert.Equal(t, "ITEM", result.Reward.Type)
	assert.Equal(t, "sword", result.Reward.RewardId)
	assert.Equal(t, int32(1), result.Reward.Quantity)
}

// ========================
// DeleteUserData (M6 GDPR)
// ========================

func TestDeleteUserData_Success(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "user123").Return(int64(5), nil)

	ctx := createAuthContext("user123", "test-namespace")
	resp, err := server.DeleteUserData(ctx, &pb.DeleteUserDataRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "user123", resp.UserId)
	assert.Equal(t, int64(5), resp.RowsDeleted)
	mockRepo.AssertExpectations(t)
}

func TestDeleteUserData_NoAuthContext(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	// No user ID in context
	resp, err := server.DeleteUserData(context.Background(), &pb.DeleteUserDataRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestDeleteUserData_DBError(t *testing.T) {
	mockCache := new(MockGoalCache)
	mockRepo := new(MockGoalRepository)
	mockRewardClient := new(MockRewardClient)

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	server := NewChallengeServiceServer(mockCache, mockRepo, mockRewardClient, db, "test-namespace", nil, 0)

	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "user123").Return(int64(0), errors.New("db error"))

	ctx := createAuthContext("user123", "test-namespace")
	resp, err := server.DeleteUserData(ctx, &pb.DeleteUserDataRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	mockRepo.AssertExpectations(t)
}
