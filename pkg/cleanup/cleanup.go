package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/repository"
)

const maxRestarts = 3

// StartCleanupGoroutine launches a background goroutine that periodically deletes expired rows.
// If the inner loop panics, it restarts up to maxRestarts times with exponential backoff (1s, 2s, 4s).
// It blocks until ctx is cancelled if enabled, or returns immediately if disabled.
func StartCleanupGoroutine(ctx context.Context, repo repository.GoalRepository, cfg CleanupConfig, namespace string, status *CleanupStatus, logger *slog.Logger) {
	if !cfg.Enabled {
		logger.Info("cleanup goroutine disabled via CLEANUP_ENABLED=false")
		return
	}

	for attempt := 0; attempt <= maxRestarts; attempt++ {
		if attempt > 0 {
			cleanupPanics.Inc()
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			logger.Warn("restarting cleanup goroutine after panic",
				"attempt", attempt,
				"maxRestarts", maxRestarts,
				"backoff", backoff,
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				logger.Info("cleanup goroutine restart cancelled by context")
				return
			}
		}

		if runCleanupLoop(ctx, repo, cfg, namespace, status, logger) {
			return // normal exit (context cancelled)
		}
	}

	logger.Error("cleanup goroutine exhausted all restarts, giving up",
		"maxRestarts", maxRestarts,
	)
}

// runCleanupLoop runs the cleanup ticker loop. It returns true on normal exit (context cancelled)
// and false if it recovered from a panic.
func runCleanupLoop(ctx context.Context, repo repository.GoalRepository, cfg CleanupConfig, namespace string, status *CleanupStatus, logger *slog.Logger) (normalExit bool) {
	defer func() {
		if r := recover(); r != nil {
			cleanupErrors.Inc()
			logger.Error("cleanup goroutine panicked", "panic", fmt.Sprintf("%v", r))
			normalExit = false
		}
	}()

	logger.Info("cleanup goroutine started",
		"interval", cfg.Interval,
		"retentionDays", cfg.RetentionDays,
		"batchSize", cfg.BatchSize,
		"maxBatchesPerCycle", cfg.MaxBatchesPerCycle,
		"batchPauseMs", cfg.BatchPauseMs,
		"initialMaxBatches", cfg.InitialMaxBatches,
		"initialCycles", cfg.InitialCycles,
		"namespace", namespace,
	)

	// D3: Track cycle count for initial turbo mode
	cycleCount := 0

	// Run immediately on startup before entering the ticker loop
	cycleCount++
	runCleanupCycle(ctx, repo, cfg, namespace, cfg.effectiveMaxBatches(cycleCount), logger)
	if status != nil {
		status.RecordHeartbeat()
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup goroutine stopping due to context cancellation")
			return true
		case <-ticker.C:
			cycleCount++
			runCleanupCycle(ctx, repo, cfg, namespace, cfg.effectiveMaxBatches(cycleCount), logger)
			if status != nil {
				status.RecordHeartbeat()
			}
		}
	}
}

// runCleanupCycle executes one cleanup cycle, deleting expired rows in batches.
// It stops after maxBatches batches to prevent sustained DB pressure.
func runCleanupCycle(ctx context.Context, repo repository.GoalRepository, cfg CleanupConfig, namespace string, maxBatches int, logger *slog.Logger) {
	start := time.Now()
	cutoff := time.Now().UTC().Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)

	// D4: Always record cycle metrics, even on error
	defer func() {
		duration := time.Since(start)
		cleanupCyclesTotal.Inc()
		cleanupDuration.Observe(duration.Seconds())
	}()

	// D2: Reusable timer for inter-batch pauses (avoids timer garbage from time.After)
	pauseDuration := time.Duration(cfg.BatchPauseMs) * time.Millisecond
	pauseTimer := time.NewTimer(0)
	if !pauseTimer.Stop() {
		<-pauseTimer.C
	}
	defer pauseTimer.Stop()

	var totalDeleted int64
	batchCount := 0
	for {
		if ctx.Err() != nil {
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}

		deleted, err := repo.DeleteExpiredRows(ctx, namespace, cutoff, cfg.BatchSize)
		if err != nil {
			cleanupErrors.Inc()
			logger.Error("cleanup batch failed", "error", err, "totalDeletedSoFar", totalDeleted)
			return
		}

		totalDeleted += deleted
		batchCount++

		if deleted < int64(cfg.BatchSize) {
			break
		}

		if batchCount >= maxBatches {
			logger.Warn("cleanup cycle hit max batches limit, remaining rows will be cleaned next cycle",
				"maxBatches", maxBatches,
				"totalDeletedSoFar", totalDeleted,
			)
			break
		}

		// Pause between batches to avoid I/O starvation
		pauseTimer.Reset(pauseDuration)
		select {
		case <-pauseTimer.C:
		case <-ctx.Done():
			if !pauseTimer.Stop() {
				<-pauseTimer.C
			}
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}
	}

	cleanupRowsDeleted.Add(float64(totalDeleted))

	logger.Info("cleanup cycle completed",
		"rowsDeleted", totalDeleted,
		"batches", batchCount,
		"duration", time.Since(start),
		"cutoff", cutoff,
	)
}
