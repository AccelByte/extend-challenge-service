package cleanup

import (
	"log/slog"
	"time"

	"extend-challenge-service/pkg/common"
)

// CleanupConfig holds configuration for the expired row cleanup goroutine.
type CleanupConfig struct {
	Enabled            bool
	Interval           time.Duration
	RetentionDays      int
	BatchSize          int
	MaxBatchesPerCycle int
	BatchPauseMs       int // Pause between batches in milliseconds (default 50, min 10, max 5000)
	InitialMaxBatches  int // Max batches per cycle during initial turbo mode (default 1000)
	InitialCycles      int // Number of initial cycles to use turbo mode (default 3)
}

// effectiveMaxBatches returns the max batches limit for a given cycle number.
// During the first InitialCycles cycles, it uses InitialMaxBatches for faster backlog clearing.
func (c CleanupConfig) effectiveMaxBatches(cycleCount int) int {
	if cycleCount <= c.InitialCycles && c.InitialMaxBatches > c.MaxBatchesPerCycle {
		return c.InitialMaxBatches
	}
	return c.MaxBatchesPerCycle
}

// NewCleanupConfigFromEnv creates a CleanupConfig from environment variables.
// Dangerous values are clamped to safe minimums to prevent panics and infinite loops.
func NewCleanupConfigFromEnv() CleanupConfig {
	logger := slog.Default()

	interval := common.GetEnvInt("CLEANUP_INTERVAL_MINUTES", 60)
	if interval < 1 {
		logger.Warn("CLEANUP_INTERVAL_MINUTES clamped to minimum 1", "original", interval)
		interval = 1
	}

	retentionDays := common.GetEnvInt("CLEANUP_RETENTION_DAYS", 7)
	if retentionDays < 0 {
		logger.Warn("CLEANUP_RETENTION_DAYS clamped to minimum 0", "original", retentionDays)
		retentionDays = 0
	}

	batchSize := common.GetEnvInt("CLEANUP_BATCH_SIZE", 1000)
	if batchSize < 1 {
		logger.Warn("CLEANUP_BATCH_SIZE clamped to minimum 1", "original", batchSize)
		batchSize = 1
	}

	maxBatches := common.GetEnvInt("CLEANUP_MAX_BATCHES_PER_CYCLE", 100)
	if maxBatches < 1 {
		logger.Warn("CLEANUP_MAX_BATCHES_PER_CYCLE clamped to minimum 1", "original", maxBatches)
		maxBatches = 1
	}

	batchPauseMs := common.GetEnvInt("CLEANUP_BATCH_PAUSE_MS", 50)
	if batchPauseMs < 10 {
		logger.Warn("CLEANUP_BATCH_PAUSE_MS clamped to minimum 10", "original", batchPauseMs)
		batchPauseMs = 10
	}
	if batchPauseMs > 5000 {
		logger.Warn("CLEANUP_BATCH_PAUSE_MS clamped to maximum 5000", "original", batchPauseMs)
		batchPauseMs = 5000
	}

	initialMaxBatches := common.GetEnvInt("CLEANUP_INITIAL_MAX_BATCHES", 1000)
	if initialMaxBatches < 1 {
		logger.Warn("CLEANUP_INITIAL_MAX_BATCHES clamped to minimum 1", "original", initialMaxBatches)
		initialMaxBatches = 1
	}

	initialCycles := common.GetEnvInt("CLEANUP_INITIAL_CYCLES", 3)
	if initialCycles < 0 {
		logger.Warn("CLEANUP_INITIAL_CYCLES clamped to minimum 0", "original", initialCycles)
		initialCycles = 0
	}

	return CleanupConfig{
		Enabled:            common.GetEnvBool("CLEANUP_ENABLED", true),
		Interval:           time.Duration(interval) * time.Minute,
		RetentionDays:      retentionDays,
		BatchSize:          batchSize,
		MaxBatchesPerCycle: maxBatches,
		BatchPauseMs:       batchPauseMs,
		InitialMaxBatches:  initialMaxBatches,
		InitialCycles:      initialCycles,
	}
}
