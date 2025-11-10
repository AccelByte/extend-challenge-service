package integration

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"extend-challenge-service/pkg/cache"
	"extend-challenge-service/pkg/handler"
	"extend-challenge-service/pkg/mapper"
	pb "extend-challenge-service/pkg/pb"

	commonCache "github.com/AccelByte/extend-challenge-common/pkg/cache"
	commonConfig "github.com/AccelByte/extend-challenge-common/pkg/config"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// setupOptimizedHandler creates an optimized challenges handler for testing
func setupOptimizedHandler(t *testing.T) (*handler.OptimizedChallengesHandler, *MockGoalRepository, func()) {
	// Truncate tables
	truncateTables(t, testDB)

	// Load challenge config
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// Initialize caches
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	serCache := cache.NewSerializedChallengeCache()

	// Convert domain challenges to protobuf format for cache warm-up
	pbChallenges := make([]*pb.Challenge, 0, len(challengeConfig.Challenges))
	for _, domainChallenge := range goalCache.GetAllChallenges() {
		pbChallenge, err := mapper.ChallengeToProto(domainChallenge, nil)
		if err != nil {
			t.Fatalf("Failed to convert challenge %s: %v", domainChallenge.ID, err)
		}
		pbChallenges = append(pbChallenges, pbChallenge)
	}

	// Warm up serialization cache
	if err := serCache.WarmUp(pbChallenges); err != nil {
		t.Fatalf("Failed to warm up cache: %v", err)
	}

	// Use mock repository for controlled testing
	mockRepo := new(MockGoalRepository)

	// Create optimized handler (auth disabled for testing)
	h := handler.NewOptimizedChallengesHandler(
		goalCache,
		mockRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,   // no token validator needed when auth is disabled
	)

	cleanup := func() {
		// No cleanup needed
	}

	return h, mockRepo, cleanup
}

// setupOptimizedHandlerWithRealDB creates handler with real database
func setupOptimizedHandlerWithRealDB(t *testing.T) (*handler.OptimizedChallengesHandler, func()) {
	// Truncate tables
	truncateTables(t, testDB)

	// Load challenge config
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// Initialize caches
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	serCache := cache.NewSerializedChallengeCache()

	// Convert domain challenges to protobuf format for cache warm-up
	pbChallenges := make([]*pb.Challenge, 0, len(challengeConfig.Challenges))
	for _, domainChallenge := range goalCache.GetAllChallenges() {
		pbChallenge, err := mapper.ChallengeToProto(domainChallenge, nil)
		if err != nil {
			t.Fatalf("Failed to convert challenge %s: %v", domainChallenge.ID, err)
		}
		pbChallenges = append(pbChallenges, pbChallenge)
	}

	// Warm up serialization cache
	if err := serCache.WarmUp(pbChallenges); err != nil {
		t.Fatalf("Failed to warm up cache: %v", err)
	}

	// Use real repository
	realRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Create optimized handler (auth disabled for testing)
	h := handler.NewOptimizedChallengesHandler(
		goalCache,
		realRepo,
		serCache,
		"test-namespace",
		false, // auth disabled
		nil,   // no token validator needed
	)

	cleanup := func() {
		// No cleanup needed
	}

	return h, cleanup
}

// TestOptimizedHandler_HappyPath_HTTP tests optimized handler with real database
func TestOptimizedHandler_HappyPath_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	userID := "test-user-optimized"

	// Seed some progress
	seedCompletedGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025")

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok)
	assert.Len(t, challenges, 2) // winter-challenge and daily-quests

	// Verify completed goal is reflected
	// Note: Optimized handler now uses camelCase (challengeId, goalId) - M3 JSON tag fix
	winterChallenge := findChallengeJSON(challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)
	goals, ok := winterChallenge["goals"].([]interface{})
	require.True(t, ok)

	completedGoal := findGoalJSON(goals, "kill-10-snowmen")
	assert.NotNil(t, completedGoal)
	assert.Equal(t, "completed", completedGoal["status"])
}

// TestOptimizedHandler_MethodNotAllowed_HTTP tests non-GET requests
func TestOptimizedHandler_MethodNotAllowed_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/challenges", nil)
			req.Header.Set("x-mock-user-id", "test-user")
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code,
				"%s should return 405 Method Not Allowed", method)
		})
	}
}

// TestOptimizedHandler_MissingUserID_HTTP tests request without user ID
func TestOptimizedHandler_MissingUserID_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	// No x-mock-user-id header
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Auth is disabled in test mode, so it should use default test user ID
	// and return success
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "challenges")
}

// TestOptimizedHandler_DatabaseError_HTTP tests database failure handling
func TestOptimizedHandler_DatabaseError_HTTP(t *testing.T) {
	h, mockRepo, cleanup := setupOptimizedHandler(t)
	defer cleanup()

	userID := "test-user-db-error"

	// Mock database error
	mockRepo.On("GetUserProgress", mock.Anything, userID, false).
		Return(nil, errors.New("database connection failed"))

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Should return 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockRepo.AssertExpectations(t)
}

// TestOptimizedHandler_EmptyProgress_HTTP tests user with no progress
func TestOptimizedHandler_EmptyProgress_HTTP(t *testing.T) {
	h, mockRepo, cleanup := setupOptimizedHandler(t)
	defer cleanup()

	userID := "test-user-empty"

	// Mock empty progress
	mockRepo.On("GetUserProgress", mock.Anything, userID, false).
		Return([]*commonDomain.UserGoalProgress{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok)
	assert.Len(t, challenges, 2) // Still returns all challenges, just with no progress

	mockRepo.AssertExpectations(t)
}

// TestOptimizedHandler_ActiveOnlyTrue_HTTP tests active_only=true parameter
func TestOptimizedHandler_ActiveOnlyTrue_HTTP(t *testing.T) {
	h, mockRepo, cleanup := setupOptimizedHandler(t)
	defer cleanup()

	userID := "test-user-active-only"

	// Mock progress with only active goals
	mockRepo.On("GetUserProgress", mock.Anything, userID, true).
		Return([]*commonDomain.UserGoalProgress{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=true", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that GetUserProgress was called with activeOnly=true
	mockRepo.AssertCalled(t, "GetUserProgress", mock.Anything, userID, true)
	mockRepo.AssertExpectations(t)
}

// TestOptimizedHandler_ActiveOnlyFalse_HTTP tests active_only=false parameter
func TestOptimizedHandler_ActiveOnlyFalse_HTTP(t *testing.T) {
	h, mockRepo, cleanup := setupOptimizedHandler(t)
	defer cleanup()

	userID := "test-user-all-goals"

	// Mock progress with all goals
	mockRepo.On("GetUserProgress", mock.Anything, userID, false).
		Return([]*commonDomain.UserGoalProgress{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges?active_only=false", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that GetUserProgress was called with activeOnly=false
	mockRepo.AssertCalled(t, "GetUserProgress", mock.Anything, userID, false)
	mockRepo.AssertExpectations(t)
}

// TestOptimizedHandler_ActiveOnlyDefault_HTTP tests omitted active_only parameter
func TestOptimizedHandler_ActiveOnlyDefault_HTTP(t *testing.T) {
	h, mockRepo, cleanup := setupOptimizedHandler(t)
	defer cleanup()

	userID := "test-user-default"

	// Mock progress - should default to activeOnly=false
	mockRepo.On("GetUserProgress", mock.Anything, userID, false).
		Return([]*commonDomain.UserGoalProgress{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify that GetUserProgress was called with activeOnly=false (default)
	mockRepo.AssertCalled(t, "GetUserProgress", mock.Anything, userID, false)
	mockRepo.AssertExpectations(t)
}

// TestOptimizedHandler_WithProgress_HTTP tests with actual progress data
func TestOptimizedHandler_WithProgress_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	userID := "test-user-with-progress"

	// Seed various progress states
	seedInProgressGoal(t, testDB, userID, "kill-10-snowmen", "winter-challenge-2025", 5, 10)
	seedCompletedGoal(t, testDB, userID, "complete-tutorial", "winter-challenge-2025")
	seedClaimedGoal(t, testDB, userID, "login-today", "daily-quests")

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok)
	assert.Len(t, challenges, 2)

	// Verify winter challenge has progress
	// Note: Optimized handler uses snake_case
	winterChallenge := findChallengeJSON(challenges, "winter-challenge-2025")
	assert.NotNil(t, winterChallenge)
	goals, ok := winterChallenge["goals"].([]interface{})
	require.True(t, ok)

	// Check in_progress goal
	inProgressGoal := findGoalJSON(goals, "kill-10-snowmen")
	assert.NotNil(t, inProgressGoal)
	assert.Equal(t, "in_progress", inProgressGoal["status"])
	assert.Equal(t, float64(5), inProgressGoal["progress"])

	// Check completed goal
	completedGoal := findGoalJSON(goals, "complete-tutorial")
	assert.NotNil(t, completedGoal)
	assert.Equal(t, "completed", completedGoal["status"])

	// Verify daily quests has claimed goal
	dailyQuests := findChallengeJSON(challenges, "daily-quests")
	assert.NotNil(t, dailyQuests)
	dailyGoals, ok := dailyQuests["goals"].([]interface{})
	require.True(t, ok)

	claimedGoal := findGoalJSON(dailyGoals, "login-today")
	assert.NotNil(t, claimedGoal)
	assert.Equal(t, "claimed", claimedGoal["status"])
}

// TestOptimizedHandler_ResponseFormat_HTTP tests JSON response structure
func TestOptimizedHandler_ResponseFormat_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	userID := "test-user-format"

	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", userID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify top-level structure
	challenges, ok := resp["challenges"].([]interface{})
	require.True(t, ok, "Response should have 'challenges' array")
	assert.GreaterOrEqual(t, len(challenges), 1, "Should have at least one challenge")

	// Verify challenge structure
	// Note: Optimized handler now uses camelCase (challengeId, goalId) - M3 JSON tag fix
	challenge := challenges[0].(map[string]interface{})
	assert.Contains(t, challenge, "challengeId", "Challenge should have challengeId (camelCase)")
	assert.Contains(t, challenge, "name", "Challenge should have name")
	assert.Contains(t, challenge, "description", "Challenge should have description")
	assert.Contains(t, challenge, "goals", "Challenge should have goals array")

	// Verify goal structure
	goals, ok := challenge["goals"].([]interface{})
	require.True(t, ok, "Goals should be an array")
	if len(goals) > 0 {
		goal := goals[0].(map[string]interface{})
		assert.Contains(t, goal, "goalId", "Goal should have goalId (camelCase)")
		assert.Contains(t, goal, "name", "Goal should have name")
		assert.Contains(t, goal, "description", "Goal should have description")
		assert.Contains(t, goal, "status", "Goal should have status")
	}
}

// TestOptimizedHandler_ConcurrentRequests_HTTP tests thread safety
func TestOptimizedHandler_ConcurrentRequests_HTTP(t *testing.T) {
	h, cleanup := setupOptimizedHandlerWithRealDB(t)
	defer cleanup()

	const numRequests = 10
	results := make(chan int, numRequests)

	// Seed some data
	seedCompletedGoal(t, testDB, "concurrent-user-1", "kill-10-snowmen", "winter-challenge-2025")
	seedCompletedGoal(t, testDB, "concurrent-user-2", "complete-tutorial", "winter-challenge-2025")

	// Make concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			userID := "concurrent-user-" + string(rune('1'+id%2))
			req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
			req.Header.Set("x-mock-user-id", userID)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			results <- w.Code
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		code := <-results
		assert.Equal(t, http.StatusOK, code, "All concurrent requests should succeed")
	}
}
