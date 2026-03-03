package cleanup

import (
	"context"
	"log/slog"
	"time"
)

// Cleaner is the interface for deleting expired rows.
// PostgresGoalRepository satisfies this via Go's structural typing.
type Cleaner interface {
	DeleteExpiredRows(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
}

// StartCleanupGoroutine launches a background goroutine that periodically deletes expired rows.
// It blocks until ctx is cancelled if enabled, or returns immediately if disabled.
func StartCleanupGoroutine(ctx context.Context, cleaner Cleaner, cfg CleanupConfig, logger *slog.Logger) {
	if !cfg.Enabled {
		logger.Info("cleanup goroutine disabled via CLEANUP_ENABLED=false")
		return
	}

	logger.Info("cleanup goroutine started",
		"interval", cfg.Interval,
		"retentionDays", cfg.RetentionDays,
		"batchSize", cfg.BatchSize,
	)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup goroutine stopping due to context cancellation")
			return
		case <-ticker.C:
			runCleanupCycle(ctx, cleaner, cfg, logger)
		}
	}
}

// runCleanupCycle executes one cleanup cycle, deleting expired rows in batches.
func runCleanupCycle(ctx context.Context, cleaner Cleaner, cfg CleanupConfig, logger *slog.Logger) {
	start := time.Now()
	cutoff := time.Now().UTC().Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)

	var totalDeleted int64
	for {
		if ctx.Err() != nil {
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}

		deleted, err := cleaner.DeleteExpiredRows(ctx, cutoff, cfg.BatchSize)
		if err != nil {
			cleanupErrors.Inc()
			logger.Error("cleanup batch failed", "error", err, "totalDeletedSoFar", totalDeleted)
			return
		}

		totalDeleted += deleted

		if deleted < int64(cfg.BatchSize) {
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
		"duration", duration,
		"cutoff", cutoff,
	)
}
