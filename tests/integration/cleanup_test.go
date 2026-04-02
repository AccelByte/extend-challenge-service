package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"extend-challenge-service/pkg/cleanup"

	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
	"github.com/stretchr/testify/require"
)

// TestCleanupGoroutine_DeletesExpiredRows verifies that the cleanup goroutine
// deletes expired rows while preserving permanent and future-expiry rows.
func TestCleanupGoroutine_DeletesExpiredRows(t *testing.T) {
	truncateTables(t, testDB)

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Seed 3 expired rows (10, 20, 30 days ago)
	seedExpiredGoal(t, testDB, "cleanup-user-1", "expired-goal-1", "ch-1", "completed", 10)
	seedExpiredGoal(t, testDB, "cleanup-user-1", "expired-goal-2", "ch-1", "in_progress", 20)
	seedExpiredGoal(t, testDB, "cleanup-user-1", "expired-goal-3", "ch-1", "claimed", 30)

	// Seed 2 permanent rows (NULL expires_at)
	seedPermanentGoal(t, testDB, "cleanup-user-1", "permanent-goal-1", "ch-1", "completed")
	seedPermanentGoal(t, testDB, "cleanup-user-1", "permanent-goal-2", "ch-1", "in_progress")

	// Seed 1 future-expiry row
	seedFutureExpiryGoal(t, testDB, "cleanup-user-1", "future-goal-1", "ch-1", "in_progress", 5)

	require.Equal(t, 6, countAllRows(t, testDB), "should have 6 rows before cleanup")

	// Start cleanup goroutine with fast interval and 0 retention days
	cfg := cleanup.CleanupConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		RetentionDays: 0,
		BatchSize:     1000,
	}

	go cleanup.StartCleanupGoroutine(t.Context(), goalRepo, cfg, "test-namespace", nil, logger)

	// Poll until expired rows are deleted (timeout 5s)
	require.Eventually(t, func() bool {
		return countAllRows(t, testDB) == 3
	}, 5*time.Second, 50*time.Millisecond, "cleanup should delete 3 expired rows, leaving 3")

	// Verify correct rows remain
	require.True(t, goalExists(t, testDB, "cleanup-user-1", "permanent-goal-1"), "permanent goal 1 should remain")
	require.True(t, goalExists(t, testDB, "cleanup-user-1", "permanent-goal-2"), "permanent goal 2 should remain")
	require.True(t, goalExists(t, testDB, "cleanup-user-1", "future-goal-1"), "future expiry goal should remain")

	// Verify expired rows are gone
	require.False(t, goalExists(t, testDB, "cleanup-user-1", "expired-goal-1"), "expired goal 1 should be deleted")
	require.False(t, goalExists(t, testDB, "cleanup-user-1", "expired-goal-2"), "expired goal 2 should be deleted")
	require.False(t, goalExists(t, testDB, "cleanup-user-1", "expired-goal-3"), "expired goal 3 should be deleted")
}

// TestCleanupGoroutine_RespectsRetentionDays verifies that cleanup respects the retention window.
// Only rows expired beyond RetentionDays are deleted.
func TestCleanupGoroutine_RespectsRetentionDays(t *testing.T) {
	truncateTables(t, testDB)

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Seed row expired 10 days ago (should be deleted with 7d retention)
	seedExpiredGoal(t, testDB, "retention-user", "old-goal", "ch-1", "completed", 10)

	// Seed row expired 3 days ago (should NOT be deleted with 7d retention)
	seedExpiredGoal(t, testDB, "retention-user", "recent-goal", "ch-1", "completed", 3)

	require.Equal(t, 2, countAllRows(t, testDB), "should have 2 rows before cleanup")

	cfg := cleanup.CleanupConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		RetentionDays: 7,
		BatchSize:     1000,
	}

	go cleanup.StartCleanupGoroutine(t.Context(), goalRepo, cfg, "test-namespace", nil, logger)

	// Poll until the old row is deleted
	require.Eventually(t, func() bool {
		return countAllRows(t, testDB) == 1
	}, 5*time.Second, 50*time.Millisecond, "cleanup should delete only old-goal (10d > 7d retention)")

	// Verify correct row remains
	require.True(t, goalExists(t, testDB, "retention-user", "recent-goal"), "recent goal (3d) should remain within retention")
	require.False(t, goalExists(t, testDB, "retention-user", "old-goal"), "old goal (10d) should be deleted past retention")
}

// TestCleanupGoroutine_DisabledDoesNothing verifies that when Enabled=false,
// StartCleanupGoroutine returns immediately and no rows are deleted.
func TestCleanupGoroutine_DisabledDoesNothing(t *testing.T) {
	truncateTables(t, testDB)

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Seed expired rows
	seedExpiredGoal(t, testDB, "disabled-user", "expired-goal-1", "ch-1", "completed", 10)
	seedExpiredGoal(t, testDB, "disabled-user", "expired-goal-2", "ch-1", "completed", 20)

	require.Equal(t, 2, countAllRows(t, testDB), "should have 2 rows before cleanup")

	cfg := cleanup.CleanupConfig{
		Enabled:       false,
		Interval:      100 * time.Millisecond,
		RetentionDays: 0,
		BatchSize:     1000,
	}

	// StartCleanupGoroutine should return immediately when disabled
	cleanup.StartCleanupGoroutine(context.Background(), goalRepo, cfg, "test-namespace", nil, logger)

	// Wait a bit to make sure nothing happens
	time.Sleep(300 * time.Millisecond)

	// All rows should remain
	require.Equal(t, 2, countAllRows(t, testDB), "disabled cleanup should not delete any rows")
}

// TestCleanupGoroutine_MultipleBatches verifies that cleanup correctly handles
// more rows than a single batch can delete.
func TestCleanupGoroutine_MultipleBatches(t *testing.T) {
	truncateTables(t, testDB)

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Seed 150 expired rows
	for i := range 150 {
		seedExpiredGoal(t, testDB, "batch-user", goalID(i), "ch-1", "completed", 10)
	}

	require.Equal(t, 150, countAllRows(t, testDB), "should have 150 rows before cleanup")

	// Use BatchSize=100 to force multiple batches
	cfg := cleanup.CleanupConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		RetentionDays: 0,
		BatchSize:     100,
	}

	go cleanup.StartCleanupGoroutine(t.Context(), goalRepo, cfg, "test-namespace", nil, logger)

	// Poll until all 150 rows are deleted
	require.Eventually(t, func() bool {
		return countAllRows(t, testDB) == 0
	}, 5*time.Second, 50*time.Millisecond, "cleanup should delete all 150 expired rows across batches")
}

// TestDeleteUserData_Integration verifies GDPR user data deletion with real database.
func TestDeleteUserData_Integration(t *testing.T) {
	truncateTables(t, testDB)

	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)

	// Seed rows for user-A (mix of statuses)
	seedExpiredGoal(t, testDB, "user-A", "goal-1", "ch-1", "completed", 5)
	seedPermanentGoal(t, testDB, "user-A", "goal-2", "ch-1", "in_progress")
	seedPermanentGoal(t, testDB, "user-A", "goal-3", "ch-2", "not_started")

	// Seed rows for user-B
	seedPermanentGoal(t, testDB, "user-B", "goal-1", "ch-1", "completed")
	seedPermanentGoal(t, testDB, "user-B", "goal-2", "ch-2", "in_progress")

	require.Equal(t, 3, countRowsForUser(t, testDB, "user-A"), "user-A should have 3 rows")
	require.Equal(t, 2, countRowsForUser(t, testDB, "user-B"), "user-B should have 2 rows")

	// Delete user-A data
	deleted, err := goalRepo.DeleteUserData(context.Background(), "test-namespace", "user-A")
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted, "should delete 3 rows for user-A")

	// Verify user-A is gone
	require.Equal(t, 0, countRowsForUser(t, testDB, "user-A"), "user-A should have 0 rows after delete")

	// Verify user-B is intact
	require.Equal(t, 2, countRowsForUser(t, testDB, "user-B"), "user-B should still have 2 rows")
	require.True(t, goalExists(t, testDB, "user-B", "goal-1"), "user-B goal-1 should exist")
	require.True(t, goalExists(t, testDB, "user-B", "goal-2"), "user-B goal-2 should exist")

	// Idempotency: delete user-A again
	deleted, err = goalRepo.DeleteUserData(context.Background(), "test-namespace", "user-A")
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted, "second delete should return 0")
}

// goalID generates a unique goal ID for batch tests.
func goalID(i int) string {
	return fmt.Sprintf("batch-goal-%03d", i)
}
