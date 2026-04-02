package cleanup

import (
	"testing"
	"time"
)

func TestNewCleanupConfigFromEnv_Defaults(t *testing.T) {
	cfg := NewCleanupConfigFromEnv()

	if !cfg.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if cfg.Interval != 60*time.Minute {
		t.Errorf("expected Interval=60m, got %v", cfg.Interval)
	}
	if cfg.RetentionDays != 7 {
		t.Errorf("expected RetentionDays=7, got %d", cfg.RetentionDays)
	}
	if cfg.BatchSize != 1000 {
		t.Errorf("expected BatchSize=1000, got %d", cfg.BatchSize)
	}
	if cfg.MaxBatchesPerCycle != 100 {
		t.Errorf("expected MaxBatchesPerCycle=100, got %d", cfg.MaxBatchesPerCycle)
	}
	if cfg.RetryBackoff != 5*time.Second {
		t.Errorf("expected RetryBackoff=5s, got %v", cfg.RetryBackoff)
	}
}

func TestNewCleanupConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("CLEANUP_ENABLED", "false")
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "30")
	t.Setenv("CLEANUP_RETENTION_DAYS", "14")
	t.Setenv("CLEANUP_BATCH_SIZE", "500")
	t.Setenv("CLEANUP_MAX_BATCHES_PER_CYCLE", "50")

	cfg := NewCleanupConfigFromEnv()

	if cfg.Enabled {
		t.Error("expected Enabled=false")
	}
	if cfg.Interval != 30*time.Minute {
		t.Errorf("expected Interval=30m, got %v", cfg.Interval)
	}
	if cfg.RetentionDays != 14 {
		t.Errorf("expected RetentionDays=14, got %d", cfg.RetentionDays)
	}
	if cfg.BatchSize != 500 {
		t.Errorf("expected BatchSize=500, got %d", cfg.BatchSize)
	}
	if cfg.MaxBatchesPerCycle != 50 {
		t.Errorf("expected MaxBatchesPerCycle=50, got %d", cfg.MaxBatchesPerCycle)
	}
}

func TestNewCleanupConfigFromEnv_InvalidFallsBack(t *testing.T) {
	t.Setenv("CLEANUP_ENABLED", "maybe")
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "notanumber")
	t.Setenv("CLEANUP_RETENTION_DAYS", "")
	t.Setenv("CLEANUP_BATCH_SIZE", "abc")
	t.Setenv("CLEANUP_MAX_BATCHES_PER_CYCLE", "xyz")

	cfg := NewCleanupConfigFromEnv()

	if !cfg.Enabled {
		t.Error("expected Enabled=true (fallback for invalid)")
	}
	if cfg.Interval != 60*time.Minute {
		t.Errorf("expected Interval=60m (fallback), got %v", cfg.Interval)
	}
	if cfg.RetentionDays != 7 {
		t.Errorf("expected RetentionDays=7 (fallback), got %d", cfg.RetentionDays)
	}
	if cfg.BatchSize != 1000 {
		t.Errorf("expected BatchSize=1000 (fallback), got %d", cfg.BatchSize)
	}
	if cfg.MaxBatchesPerCycle != 100 {
		t.Errorf("expected MaxBatchesPerCycle=100 (fallback), got %d", cfg.MaxBatchesPerCycle)
	}
}

func TestNewCleanupConfigFromEnv_ZeroValuesClamped(t *testing.T) {
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "0")
	t.Setenv("CLEANUP_RETENTION_DAYS", "0")
	t.Setenv("CLEANUP_BATCH_SIZE", "0")
	t.Setenv("CLEANUP_MAX_BATCHES_PER_CYCLE", "0")

	cfg := NewCleanupConfigFromEnv()

	if cfg.Interval != 1*time.Minute {
		t.Errorf("expected Interval clamped to 1m, got %v", cfg.Interval)
	}
	if cfg.RetentionDays != 0 {
		t.Errorf("expected RetentionDays=0 (valid: delete immediately after expiry), got %d", cfg.RetentionDays)
	}
	if cfg.BatchSize != 1 {
		t.Errorf("expected BatchSize clamped to 1, got %d", cfg.BatchSize)
	}
	if cfg.MaxBatchesPerCycle != 1 {
		t.Errorf("expected MaxBatchesPerCycle clamped to 1, got %d", cfg.MaxBatchesPerCycle)
	}
}

func TestNewCleanupConfigFromEnv_NegativeValuesClamped(t *testing.T) {
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "-5")
	t.Setenv("CLEANUP_RETENTION_DAYS", "-1")
	t.Setenv("CLEANUP_BATCH_SIZE", "-10")
	t.Setenv("CLEANUP_MAX_BATCHES_PER_CYCLE", "-3")

	cfg := NewCleanupConfigFromEnv()

	if cfg.Interval != 1*time.Minute {
		t.Errorf("expected Interval clamped to 1m, got %v", cfg.Interval)
	}
	if cfg.RetentionDays != 0 {
		t.Errorf("expected RetentionDays clamped to 0, got %d", cfg.RetentionDays)
	}
	if cfg.BatchSize != 1 {
		t.Errorf("expected BatchSize clamped to 1, got %d", cfg.BatchSize)
	}
	if cfg.MaxBatchesPerCycle != 1 {
		t.Errorf("expected MaxBatchesPerCycle clamped to 1, got %d", cfg.MaxBatchesPerCycle)
	}
}

func TestNewCleanupConfigFromEnv_RetryBackoff(t *testing.T) {
	// Override
	t.Setenv("CLEANUP_RETRY_BACKOFF_SECONDS", "10")
	cfg := NewCleanupConfigFromEnv()
	if cfg.RetryBackoff != 10*time.Second {
		t.Errorf("expected RetryBackoff=10s, got %v", cfg.RetryBackoff)
	}
}

func TestNewCleanupConfigFromEnv_RetryBackoffClampMin(t *testing.T) {
	t.Setenv("CLEANUP_RETRY_BACKOFF_SECONDS", "0")
	cfg := NewCleanupConfigFromEnv()
	if cfg.RetryBackoff != 1*time.Second {
		t.Errorf("expected RetryBackoff clamped to 1s, got %v", cfg.RetryBackoff)
	}
}

func TestNewCleanupConfigFromEnv_RetryBackoffClampMax(t *testing.T) {
	t.Setenv("CLEANUP_RETRY_BACKOFF_SECONDS", "120")
	cfg := NewCleanupConfigFromEnv()
	if cfg.RetryBackoff != 60*time.Second {
		t.Errorf("expected RetryBackoff clamped to 60s, got %v", cfg.RetryBackoff)
	}
}
