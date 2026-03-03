package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/repository"
)

// StartCleanupGoroutine launches a background goroutine that periodically deletes expired rows.
// It runs one cleanup cycle immediately on startup, then enters the ticker loop.
// It blocks until ctx is cancelled if enabled, or returns immediately if disabled.
func StartCleanupGoroutine(ctx context.Context, repo repository.GoalRepository, cfg CleanupConfig, logger *slog.Logger) {
	if !cfg.Enabled {
		logger.Info("cleanup goroutine disabled via CLEANUP_ENABLED=false")
		return
	}

	logger.Info("cleanup goroutine started",
		"interval", cfg.Interval,
		"retentionDays", cfg.RetentionDays,
		"batchSize", cfg.BatchSize,
		"maxBatchesPerCycle", cfg.MaxBatchesPerCycle,
	)

	// Run immediately on startup before entering the ticker loop
	runCleanupCycle(ctx, repo, cfg, logger)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup goroutine stopping due to context cancellation")
			return
		case <-ticker.C:
			runCleanupCycle(ctx, repo, cfg, logger)
		}
	}
}

// runCleanupCycle executes one cleanup cycle, deleting expired rows in batches.
// It stops after MaxBatchesPerCycle batches to prevent sustained DB pressure.
func runCleanupCycle(ctx context.Context, repo repository.GoalRepository, cfg CleanupConfig, logger *slog.Logger) {
	start := time.Now()
	cutoff := time.Now().UTC().Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)

	var totalDeleted int64
	batchCount := 0
	for {
		if ctx.Err() != nil {
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}

		deleted, err := repo.DeleteExpiredRows(ctx, cutoff, cfg.BatchSize)
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

		if batchCount >= cfg.MaxBatchesPerCycle {
			logger.Warn("cleanup cycle hit max batches limit, remaining rows will be cleaned next cycle",
				"maxBatchesPerCycle", cfg.MaxBatchesPerCycle,
				"totalDeletedSoFar", totalDeleted,
			)
			break
		}

		// Pause between batches to avoid I/O starvation
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}
	}

	duration := time.Since(start)
	cleanupCyclesTotal.Inc()
	cleanupRowsDeleted.Add(float64(totalDeleted))
	cleanupDuration.Observe(duration.Seconds())

	logger.Info("cleanup cycle completed",
		"rowsDeleted", totalDeleted,
		"batches", batchCount,
		"duration", duration,
		"cutoff", cutoff,
	)
}
