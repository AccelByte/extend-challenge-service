package cleanup

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/repository"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/mock"
)

// mockRepo is a mock that implements repository.GoalRepository for cleanup tests.
// Only DeleteExpiredRows is exercised; other methods are stubs.
type mockRepo struct {
	mock.Mock
	mu      sync.Mutex
	calls   []mockCleanerCall
	results []mockCleanerResult
	callIdx int
}

type mockCleanerCall struct {
	cutoff    time.Time
	batchSize int
}

type mockCleanerResult struct {
	deleted int64
	err     error
}

func (m *mockRepo) DeleteExpiredRows(_ context.Context, _ string, cutoff time.Time, batchSize int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, mockCleanerCall{cutoff: cutoff, batchSize: batchSize})

	if m.callIdx >= len(m.results) {
		return 0, nil
	}
	r := m.results[m.callIdx]
	m.callIdx++
	return r.deleted, r.err
}

func (m *mockRepo) getCalls() []mockCleanerCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]mockCleanerCall, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// Stub implementations for GoalRepository interface (not used by cleanup).
func (m *mockRepo) GetProgress(_ context.Context, _, _ string) (*domain.UserGoalProgress, error) {
	return nil, nil
}
func (m *mockRepo) GetUserProgress(_ context.Context, _ string, _ bool) ([]*domain.UserGoalProgress, error) {
	return nil, nil
}
func (m *mockRepo) GetChallengeProgress(_ context.Context, _, _ string, _ bool) ([]*domain.UserGoalProgress, error) {
	return nil, nil
}
func (m *mockRepo) UpsertProgress(_ context.Context, _ *domain.UserGoalProgress) error { return nil }
func (m *mockRepo) BatchUpsertProgress(_ context.Context, _ []*domain.UserGoalProgress) error {
	return nil
}
func (m *mockRepo) BatchUpsertProgressWithCOPY(_ context.Context, _ []repository.CopyRow) error {
	return nil
}
func (m *mockRepo) MarkAsClaimed(_ context.Context, _, _ string) error { return nil }
func (m *mockRepo) BeginTx(_ context.Context) (repository.TxRepository, error) {
	return nil, nil
}
func (m *mockRepo) GetGoalsByIDs(_ context.Context, _ string, _ []string) ([]*domain.UserGoalProgress, error) {
	return nil, nil
}
func (m *mockRepo) BulkInsert(_ context.Context, _ []*domain.UserGoalProgress) error { return nil }
func (m *mockRepo) BulkInsertWithCOPY(_ context.Context, _ []*domain.UserGoalProgress) error {
	return nil
}
func (m *mockRepo) UpsertGoalActive(_ context.Context, _ *domain.UserGoalProgress) error { return nil }
func (m *mockRepo) BatchUpsertGoalActive(_ context.Context, _ []*domain.UserGoalProgress) error {
	return nil
}
func (m *mockRepo) GetUserGoalCount(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *mockRepo) GetActiveGoals(_ context.Context, _ string) ([]*domain.UserGoalProgress, error) {
	return nil, nil
}
func (m *mockRepo) DeleteUserData(_ context.Context, _, _ string) (int64, error) { return 0, nil }

// resetMetrics creates fresh metric instances for test isolation.
func resetMetrics() {
	cleanupRowsDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "challenge_cleanup_rows_deleted_total",
		Help: "Total number of expired rows deleted by the cleanup goroutine.",
	})
	cleanupCyclesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "challenge_cleanup_cycles_total",
		Help: "Total number of cleanup cycles executed.",
	})
	cleanupErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "challenge_cleanup_errors_total",
		Help: "Total number of errors encountered during cleanup.",
	})
	cleanupDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "challenge_cleanup_duration_seconds",
		Help:    "Duration of each cleanup cycle in seconds.",
		Buckets: prometheus.DefBuckets,
	})
}

func getCounterValue(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	_ = c.(prometheus.Metric).Write(m)
	return m.GetCounter().GetValue()
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestStartCleanupGoroutine_Disabled(t *testing.T) {
	m := &mockRepo{}
	cfg := CleanupConfig{Enabled: false}

	// Should return immediately without calling repo
	StartCleanupGoroutine(context.Background(), m, cfg, "test-ns", nil, testLogger())

	calls := m.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls when disabled, got %d", len(calls))
	}
}

func TestRunCleanupCycle_HappyPath(t *testing.T) {
	resetMetrics()

	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 500, err: nil}, // < batchSize, so only 1 call
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	runCleanupCycle(context.Background(), m, cfg, "test-ns", cfg.MaxBatchesPerCycle, testLogger())

	calls := m.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	if calls[0].batchSize != 1000 {
		t.Errorf("expected batchSize=1000, got %d", calls[0].batchSize)
	}

	// Verify metrics
	if v := getCounterValue(cleanupCyclesTotal); v != 1 {
		t.Errorf("expected cycles_total=1, got %f", v)
	}
	if v := getCounterValue(cleanupRowsDeleted); v != 500 {
		t.Errorf("expected rows_deleted=500, got %f", v)
	}
}

func TestRunCleanupCycle_MultipleBatches(t *testing.T) {
	resetMetrics()

	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 1000, err: nil}, // == batchSize, continue
			{deleted: 1000, err: nil}, // == batchSize, continue
			{deleted: 300, err: nil},  // < batchSize, stop
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	runCleanupCycle(context.Background(), m, cfg, "test-ns", cfg.MaxBatchesPerCycle, testLogger())

	calls := m.getCalls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	if v := getCounterValue(cleanupRowsDeleted); v != 2300 {
		t.Errorf("expected rows_deleted=2300, got %f", v)
	}
}

func TestRunCleanupCycle_Error(t *testing.T) {
	resetMetrics()

	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 1000, err: nil},
			{deleted: 0, err: errors.New("db connection lost")},
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	runCleanupCycle(context.Background(), m, cfg, "test-ns", cfg.MaxBatchesPerCycle, testLogger())

	calls := m.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (abort after error), got %d", len(calls))
	}

	if v := getCounterValue(cleanupErrors); v != 1 {
		t.Errorf("expected errors_total=1, got %f", v)
	}

	// D4: cycles_total is always incremented (via defer), even on error
	if v := getCounterValue(cleanupCyclesTotal); v != 1 {
		t.Errorf("expected cycles_total=1 on error (deferred), got %f", v)
	}
}

func TestRunCleanupCycle_CorrectCutoff(t *testing.T) {
	resetMetrics()

	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 0, err: nil},
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	before := time.Now().UTC().Add(-7 * 24 * time.Hour)
	runCleanupCycle(context.Background(), m, cfg, "test-ns", cfg.MaxBatchesPerCycle, testLogger())
	after := time.Now().UTC().Add(-7 * 24 * time.Hour)

	calls := m.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	cutoff := calls[0].cutoff
	// Cutoff should be approximately NOW() - 7 days (within 2 seconds tolerance)
	if cutoff.Before(before.Add(-2*time.Second)) || cutoff.After(after.Add(2*time.Second)) {
		t.Errorf("cutoff %v not within expected range [%v, %v]", cutoff, before, after)
	}
}

func TestStartCleanupGoroutine_ContextCancel(t *testing.T) {
	resetMetrics()

	m := &mockRepo{}
	cfg := CleanupConfig{
		Enabled:            true,
		Interval:           100 * time.Millisecond,
		MaxBatchesPerCycle: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartCleanupGoroutine(ctx, m, cfg, "test-ns", nil, testLogger())
		close(done)
	}()

	// Let goroutine start then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Goroutine exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit within 2 seconds after cancel")
	}
}

func TestStartCleanupGoroutine_ImmediateExecution(t *testing.T) {
	resetMetrics()

	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 42, err: nil}, // immediate execution result
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		Interval:           10 * time.Second, // Long interval so ticker won't fire
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartCleanupGoroutine(ctx, m, cfg, "test-ns", nil, testLogger())
		close(done)
	}()

	// Wait briefly for immediate execution to complete
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit within 2 seconds after cancel")
	}

	// Verify that cleanup ran immediately (before any ticker fire)
	calls := m.getCalls()
	if len(calls) < 1 {
		t.Fatalf("expected at least 1 call from immediate execution, got %d", len(calls))
	}
}

func TestRunCleanupCycle_MaxBatchesGuard(t *testing.T) {
	resetMetrics()

	// Return full batches every time (would loop forever without max guard)
	m := &mockRepo{
		results: []mockCleanerResult{
			{deleted: 100, err: nil},
			{deleted: 100, err: nil},
			{deleted: 100, err: nil},
			{deleted: 100, err: nil},
			{deleted: 100, err: nil},
		},
	}
	cfg := CleanupConfig{
		Enabled:            true,
		RetentionDays:      7,
		BatchSize:          100,
		MaxBatchesPerCycle: 3, // Limit to 3 batches
	}

	runCleanupCycle(context.Background(), m, cfg, "test-ns", cfg.MaxBatchesPerCycle, testLogger())

	calls := m.getCalls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls (max batches limit), got %d", len(calls))
	}

	if v := getCounterValue(cleanupRowsDeleted); v != 300 {
		t.Errorf("expected rows_deleted=300, got %f", v)
	}

	// Cycle should still be counted as complete
	if v := getCounterValue(cleanupCyclesTotal); v != 1 {
		t.Errorf("expected cycles_total=1, got %f", v)
	}
}

func TestStartCleanupGoroutine_PanicRecovery(t *testing.T) {
	resetMetrics()

	// Create a mock that panics on DeleteExpiredRows
	m := &panicRepo{}
	cfg := CleanupConfig{
		Enabled:            true,
		Interval:           100 * time.Millisecond,
		RetentionDays:      7,
		BatchSize:          1000,
		MaxBatchesPerCycle: 100,
	}

	done := make(chan struct{})
	go func() {
		// Should NOT crash — panic is recovered
		StartCleanupGoroutine(context.Background(), m, cfg, "test-ns", nil, testLogger())
		close(done)
	}()

	select {
	case <-done:
		// Goroutine exited cleanly after panic recovery
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit within 2 seconds after panic")
	}

	// Verify error was counted
	if v := getCounterValue(cleanupErrors); v < 1 {
		t.Errorf("expected errors_total>=1 after panic, got %f", v)
	}
}

func TestCleanupConfig_BatchPauseMs(t *testing.T) {
	cfg := CleanupConfig{BatchPauseMs: 50}
	if cfg.BatchPauseMs != 50 {
		t.Errorf("expected BatchPauseMs=50, got %d", cfg.BatchPauseMs)
	}

	// Verify env parsing with clamping
	t.Setenv("CLEANUP_BATCH_PAUSE_MS", "5")
	t.Setenv("CLEANUP_ENABLED", "true")
	envCfg := NewCleanupConfigFromEnv()
	if envCfg.BatchPauseMs != 10 {
		t.Errorf("expected BatchPauseMs clamped to 10, got %d", envCfg.BatchPauseMs)
	}
}

func TestCleanupConfig_InitialTurbo(t *testing.T) {
	cfg := CleanupConfig{
		MaxBatchesPerCycle: 100,
		InitialMaxBatches:  1000,
		InitialCycles:      3,
	}

	// Cycle 1-3 should use turbo
	if v := cfg.effectiveMaxBatches(1); v != 1000 {
		t.Errorf("cycle 1: expected 1000 (turbo), got %d", v)
	}
	if v := cfg.effectiveMaxBatches(3); v != 1000 {
		t.Errorf("cycle 3: expected 1000 (turbo), got %d", v)
	}

	// Cycle 4+ should use normal
	if v := cfg.effectiveMaxBatches(4); v != 100 {
		t.Errorf("cycle 4: expected 100 (normal), got %d", v)
	}

	// When InitialMaxBatches <= MaxBatchesPerCycle, always use normal
	cfg2 := CleanupConfig{
		MaxBatchesPerCycle: 100,
		InitialMaxBatches:  50,
		InitialCycles:      3,
	}
	if v := cfg2.effectiveMaxBatches(1); v != 100 {
		t.Errorf("cycle 1 (no turbo): expected 100, got %d", v)
	}
}

// panicRepo is a mock that panics on the first call to DeleteExpiredRows.
type panicRepo struct {
	mockRepo
}

func (m *panicRepo) DeleteExpiredRows(_ context.Context, _ string, _ time.Time, _ int) (int64, error) {
	panic("test panic in cleanup")
}
