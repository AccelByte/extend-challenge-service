package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGDPRGoalRepository is a mock for GDPR handler tests.
type MockGDPRGoalRepository struct {
	mock.Mock
}

func (m *MockGDPRGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGDPRGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGDPRGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGDPRGoalRepository) UpsertProgress(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) BatchUpsertProgress(ctx context.Context, updates []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, rows []commonRepo.CopyRow) error {
	args := m.Called(ctx, rows)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) BeginTx(ctx context.Context) (commonRepo.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(commonRepo.TxRepository), args.Error(1)
}

func (m *MockGDPRGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGDPRGoalRepository) BulkInsert(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) BulkInsertWithCOPY(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) UpsertGoalActive(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) BatchUpsertGoalActive(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGDPRGoalRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockGDPRGoalRepository) GetActiveGoals(ctx context.Context, userID string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGDPRGoalRepository) DeleteExpiredRows(ctx context.Context, namespace string, cutoff time.Time, batchSize int) (int64, error) {
	args := m.Called(ctx, namespace, cutoff, batchSize)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGDPRGoalRepository) DeleteUserData(ctx context.Context, namespace string, userID string) (int64, error) {
	args := m.Called(ctx, namespace, userID)
	return args.Get(0).(int64), args.Error(1)
}

func testGDPRLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestGDPRDeletionHandler_Success(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "test-user-id").Return(int64(5), nil)

	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req.Header.Set("x-mock-user-id", "test-user-id")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"userId":"test-user-id"`)
	assert.Contains(t, rr.Body.String(), `"rowsDeleted":5`)
	mockRepo.AssertExpectations(t)
}

func TestGDPRDeletionHandler_SuccessNoRows(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "new-user").Return(int64(0), nil)

	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req.Header.Set("x-mock-user-id", "new-user")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"rowsDeleted":0`)
	mockRepo.AssertExpectations(t)
}

func TestGDPRDeletionHandler_MethodNotAllowed(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/v1/users/me/data", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "method %s should be rejected", method)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "method %s should return JSON", method)

		var errResp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &errResp)
		assert.NoError(t, err, "method %s response should be valid JSON", method)
		assert.Equal(t, "METHOD_NOT_ALLOWED", errResp["errorCode"], "method %s should have errorCode", method)
		assert.NotEmpty(t, errResp["message"], "method %s should have message", method)
	}
}

func TestGDPRDeletionHandler_AuthEnabled_NoHeader(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", true, nil, testGDPRLogger())

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var errResp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &errResp)
	assert.NoError(t, err, "response should be valid JSON")
	assert.Equal(t, "UNAUTHORIZED", errResp["errorCode"])
	assert.NotEmpty(t, errResp["message"])
}

func TestGDPRDeletionHandler_DBError(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "test-user-id").Return(int64(0), errors.New("db error"))

	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req.Header.Set("x-mock-user-id", "test-user-id")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var errResp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &errResp)
	assert.NoError(t, err, "response should be valid JSON")
	assert.Equal(t, "INTERNAL_ERROR", errResp["errorCode"])
	assert.NotEmpty(t, errResp["message"])
	mockRepo.AssertExpectations(t)
}

func TestGDPRDeletionHandler_RateLimit(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "user-a").Return(int64(3), nil)
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "user-b").Return(int64(1), nil)

	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	// First call for user-a succeeds
	req1 := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req1.Header.Set("x-mock-user-id", "user-a")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Immediate second call for same user gets 429
	req2 := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req2.Header.Set("x-mock-user-id", "user-a")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)

	var errResp map[string]string
	err := json.Unmarshal(rr2.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "RATE_LIMITED", errResp["errorCode"])

	// Different user succeeds
	req3 := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	req3.Header.Set("x-mock-user-id", "user-b")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusOK, rr3.Code)
}

func TestGDPRDeletionHandler_DefaultTestUser(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	mockRepo.On("DeleteUserData", mock.Anything, "test-namespace", "test-user-id").Return(int64(0), nil)

	handler := NewGDPRDeletionHandler(context.Background(), mockRepo, "test-namespace", false, nil, testGDPRLogger())

	// No x-mock-user-id header, should default to "test-user-id"
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/data", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"userId":"test-user-id"`)
	mockRepo.AssertExpectations(t)
}

func TestGDPRDeletionHandler_EvictionContextCancel(t *testing.T) {
	mockRepo := &MockGDPRGoalRepository{}
	ctx, cancel := context.WithCancel(context.Background())

	// Create handler with cancellable context
	_ = NewGDPRDeletionHandler(ctx, mockRepo, "test-namespace", false, nil, testGDPRLogger())

	// Cancel should not panic — eviction goroutine exits cleanly
	cancel()
	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)
}
