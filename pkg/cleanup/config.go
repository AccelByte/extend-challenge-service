package cleanup

import (
	"time"

	"extend-challenge-service/pkg/common"
)

// CleanupConfig holds configuration for the expired row cleanup goroutine.
type CleanupConfig struct {
	Enabled       bool
	Interval      time.Duration
	RetentionDays int
	BatchSize     int
}

// NewCleanupConfigFromEnv creates a CleanupConfig from environment variables.
func NewCleanupConfigFromEnv() CleanupConfig {
	return CleanupConfig{
		Enabled:       common.GetEnvBool("CLEANUP_ENABLED", true),
		Interval:      time.Duration(common.GetEnvInt("CLEANUP_INTERVAL_MINUTES", 60)) * time.Minute,
		RetentionDays: common.GetEnvInt("CLEANUP_RETENTION_DAYS", 7),
		BatchSize:     common.GetEnvInt("CLEANUP_BATCH_SIZE", 1000),
	}
}
