package cleanup

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// mockCleaner is a test double for the Cleaner interface.
type mockCleaner struct {
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

func (m *mockCleaner) DeleteExpiredRows(_ context.Context, cutoff time.Time, batchSize int) (int64, error) {
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

func (m *mockCleaner) getCalls() []mockCleanerCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]mockCleanerCall, len(m.calls))
	copy(cp, m.calls)
	return cp
}

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
	mock := &mockCleaner{}
	cfg := CleanupConfig{Enabled: false}

	// Should return immediately without calling cleaner
	StartCleanupGoroutine(context.Background(), mock, cfg, testLogger())

	calls := mock.getCalls()
	if len(calls) != 0 {
		t.Errorf("expected 0 calls when disabled, got %d", len(calls))
	}
}

func TestRunCleanupCycle_HappyPath(t *testing.T) {
	resetMetrics()

	mock := &mockCleaner{
		results: []mockCleanerResult{
			{deleted: 500, err: nil}, // < batchSize, so only 1 call
		},
	}
	cfg := CleanupConfig{
		Enabled:       true,
		RetentionDays: 7,
		BatchSize:     1000,
	}

	runCleanupCycle(context.Background(), mock, cfg, testLogger())

	calls := mock.getCalls()
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

	mock := &mockCleaner{
		results: []mockCleanerResult{
			{deleted: 1000, err: nil}, // == batchSize, continue
			{deleted: 1000, err: nil}, // == batchSize, continue
			{deleted: 300, err: nil},  // < batchSize, stop
		},
	}
	cfg := CleanupConfig{
		Enabled:       true,
		RetentionDays: 7,
		BatchSize:     1000,
	}

	runCleanupCycle(context.Background(), mock, cfg, testLogger())

	calls := mock.getCalls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	if v := getCounterValue(cleanupRowsDeleted); v != 2300 {
		t.Errorf("expected rows_deleted=2300, got %f", v)
	}
}

func TestRunCleanupCycle_Error(t *testing.T) {
	resetMetrics()

	mock := &mockCleaner{
		results: []mockCleanerResult{
			{deleted: 1000, err: nil},
			{deleted: 0, err: errors.New("db connection lost")},
		},
	}
	cfg := CleanupConfig{
		Enabled:       true,
		RetentionDays: 7,
		BatchSize:     1000,
	}

	runCleanupCycle(context.Background(), mock, cfg, testLogger())

	calls := mock.getCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (abort after error), got %d", len(calls))
	}

	if v := getCounterValue(cleanupErrors); v != 1 {
		t.Errorf("expected errors_total=1, got %f", v)
	}

	// cycles_total should NOT be incremented on error
	if v := getCounterValue(cleanupCyclesTotal); v != 0 {
		t.Errorf("expected cycles_total=0 on error, got %f", v)
	}
}

func TestRunCleanupCycle_CorrectCutoff(t *testing.T) {
	resetMetrics()

	mock := &mockCleaner{
		results: []mockCleanerResult{
			{deleted: 0, err: nil},
		},
	}
	cfg := CleanupConfig{
		Enabled:       true,
		RetentionDays: 7,
		BatchSize:     1000,
	}

	before := time.Now().UTC().Add(-7 * 24 * time.Hour)
	runCleanupCycle(context.Background(), mock, cfg, testLogger())
	after := time.Now().UTC().Add(-7 * 24 * time.Hour)

	calls := mock.getCalls()
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
	mock := &mockCleaner{}
	cfg := CleanupConfig{
		Enabled:  true,
		Interval: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartCleanupGoroutine(ctx, mock, cfg, testLogger())
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
