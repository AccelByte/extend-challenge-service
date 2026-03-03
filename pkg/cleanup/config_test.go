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
}

func TestNewCleanupConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("CLEANUP_ENABLED", "false")
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "30")
	t.Setenv("CLEANUP_RETENTION_DAYS", "14")
	t.Setenv("CLEANUP_BATCH_SIZE", "500")

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
}

func TestNewCleanupConfigFromEnv_InvalidFallsBack(t *testing.T) {
	t.Setenv("CLEANUP_ENABLED", "maybe")
	t.Setenv("CLEANUP_INTERVAL_MINUTES", "notanumber")
	t.Setenv("CLEANUP_RETENTION_DAYS", "")
	t.Setenv("CLEANUP_BATCH_SIZE", "abc")

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
}
