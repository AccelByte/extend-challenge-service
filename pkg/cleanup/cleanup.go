package cleanup

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
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

	// Replica jitter: random startup delay in [0, interval/2) to spread cleanup cycles
	// across replicas and reduce thundering herd effects on the database.
	if jitter := randomJitter(cfg.Interval / 2); jitter > 0 {
		logger.Info("cleanup goroutine applying startup jitter", "jitter", jitter)
		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			logger.Info("cleanup goroutine jitter interrupted by context cancellation")
			return true
		}
	}

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
	now := time.Now()
	start := now
	cutoff := now.UTC().Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)

	// D4: Always record cycle metrics, even on error
	defer func() {
		duration := time.Since(start)
		cleanupCyclesTotal.Inc()
		cleanupDuration.Observe(duration.Seconds())
	}()

	pauseDuration := time.Duration(cfg.BatchPauseMs) * time.Millisecond

	var totalDeleted int64
	defer func() {
		if totalDeleted > 0 {
			cleanupRowsDeleted.Add(float64(totalDeleted))
		}
	}()

	batchCount := 0
	for {
		if ctx.Err() != nil {
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}

		const maxRetries = 3
		var deleted int64
		var err error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			deleted, err = repo.DeleteExpiredRows(ctx, namespace, cutoff, cfg.BatchSize)
			if err == nil {
				break
			}
			cleanupErrors.Inc()
			logger.Error("cleanup batch failed", "error", err, "attempt", attempt, "maxRetries", maxRetries, "totalDeletedSoFar", totalDeleted)
			if attempt == maxRetries {
				return
			}
			select {
			case <-time.After(cfg.RetryBackoff):
			case <-ctx.Done():
				return
			}
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
		select {
		case <-time.After(pauseDuration):
		case <-ctx.Done():
			logger.Info("cleanup cycle interrupted by context cancellation")
			return
		}
	}

	logger.Info("cleanup cycle completed",
		"rowsDeleted", totalDeleted,
		"batches", batchCount,
		"duration", time.Since(start),
		"cutoff", cutoff,
	)
}

// randomJitter returns a random duration in [0, maxJitter) using crypto/rand.
// Returns 0 if maxJitter <= 0.
func randomJitter(maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxJitter)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}
