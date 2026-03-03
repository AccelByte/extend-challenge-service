package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"extend-challenge-service/pkg/cleanup"

	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
	"github.com/stretchr/testify/require"
)

// TestCleanupGoroutine_ConcurrentWithAPI verifies that the cleanup goroutine
// can run concurrently with API traffic without data races or incorrect results.
// Active user goals are served correctly while expired rows for other users are cleaned up.
func TestCleanupGoroutine_ConcurrentWithAPI(t *testing.T) {
	// Setup HTTP test server (truncates tables internally)
	handler, _, httpCleanup := setupHTTPTestServer(t)
	defer httpCleanup()

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	apiUserID := "api-user-concurrent"

	// Seed active goals for the API user (permanent, should not be touched)
	seedInProgressActiveGoal(t, testDB, apiUserID, "kill-10-snowmen", "winter-challenge-2025", 5, 10)

	// Seed expired rows for OTHER users (should be cleaned up)
	for i := range 20 {
		seedExpiredGoal(t, testDB, fmt.Sprintf("expired-user-%03d", i), "expired-goal", "ch-1", "completed", 10)
	}

	require.Equal(t, 21, countAllRows(t, testDB), "should have 21 rows (1 active + 20 expired)")

	// Start cleanup goroutine
	cfg := cleanup.CleanupConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		RetentionDays: 0,
		BatchSize:     1000,
	}

	go cleanup.StartCleanupGoroutine(t.Context(), goalRepo, cfg, "test-namespace", nil, logger)

	// Make API requests while cleanup is running
	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
		req.Header.Set("x-mock-user-id", apiUserID)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, "API should respond 200 during cleanup (iteration %d)", i)

		var resp map[string]any
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err, "should decode JSON response (iteration %d)", i)

		challenges, ok := resp["challenges"].([]any)
		require.True(t, ok, "response should have challenges array (iteration %d)", i)
		require.NotEmpty(t, challenges, "challenges should not be empty (iteration %d)", i)

		// Small delay between requests
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for cleanup to finish deleting expired rows
	require.Eventually(t, func() bool {
		return countAllRows(t, testDB) == 1
	}, 5*time.Second, 50*time.Millisecond, "cleanup should delete 20 expired rows, leaving 1 active")

	// Final API call should still work
	req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
	req.Header.Set("x-mock-user-id", apiUserID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "API should still work after cleanup completed")

	// Verify the active user's goal is intact
	require.True(t, goalExists(t, testDB, apiUserID, "kill-10-snowmen"), "API user's goal should survive cleanup")
}
