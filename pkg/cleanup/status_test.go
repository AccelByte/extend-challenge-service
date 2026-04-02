package cleanup

import (
	"sync"
	"testing"
	"time"
)

func TestCleanupStatus_NewIsAliveWithinGrace(t *testing.T) {
	s := NewCleanupStatus()
	// A freshly created status should be alive within the startup grace period (2 * threshold)
	if !s.IsAlive(time.Hour) {
		t.Error("new status should be alive within startup grace period")
	}
}

func TestCleanupStatus_NewIsNotAliveAfterGrace(t *testing.T) {
	// Simulate old creation time (startup grace expired)
	s := NewCleanupStatusWithCreatedAt(time.Now().Add(-24 * time.Hour))
	if s.IsAlive(time.Hour) {
		t.Error("status with expired startup grace and no heartbeat should not be alive")
	}
}

func TestCleanupStatus_RecordHeartbeat(t *testing.T) {
	s := NewCleanupStatus()
	s.RecordHeartbeat()

	if !s.IsAlive(time.Second) {
		t.Error("should be alive immediately after heartbeat")
	}
}

func TestCleanupStatus_IsAlive_Expired(t *testing.T) {
	s := NewCleanupStatus()
	s.RecordHeartbeat()

	// Wait briefly then check with very tight threshold
	time.Sleep(50 * time.Millisecond)

	if s.IsAlive(10 * time.Millisecond) {
		t.Error("should not be alive after threshold expires")
	}
	if !s.IsAlive(time.Second) {
		t.Error("should be alive with generous threshold")
	}
}

func TestCleanupStatus_LastHeartbeatUnixSeconds_NoHeartbeat(t *testing.T) {
	s := NewCleanupStatus()
	if v := s.LastHeartbeatUnixSeconds(); v != 0 {
		t.Errorf("expected 0 for no heartbeat, got %f", v)
	}
}

func TestCleanupStatus_LastHeartbeatUnixSeconds_AfterHeartbeat(t *testing.T) {
	s := NewCleanupStatus()
	before := float64(time.Now().Unix())
	s.RecordHeartbeat()
	after := float64(time.Now().Unix()) + 1

	v := s.LastHeartbeatUnixSeconds()
	if v < before || v > after {
		t.Errorf("heartbeat seconds %f not in expected range [%f, %f]", v, before, after)
	}
}

func TestCleanupStatus_MultipleHeartbeats(t *testing.T) {
	s := NewCleanupStatus()

	for range 5 {
		s.RecordHeartbeat()
		time.Sleep(10 * time.Millisecond)
	}

	if !s.IsAlive(time.Second) {
		t.Error("should be alive after multiple heartbeats")
	}
}

func TestCleanupStatus_IsAlive_GraceBoundary(t *testing.T) {
	threshold := 100 * time.Millisecond
	grace := 2 * threshold // 200ms

	// Just inside grace boundary: created at now - (grace - 10ms) → should be alive
	insideGrace := NewCleanupStatusWithCreatedAt(time.Now().Add(-(grace - 10*time.Millisecond)))
	if !insideGrace.IsAlive(threshold) {
		t.Error("should be alive just inside 2*threshold boundary")
	}

	// Just outside grace boundary: created at now - (grace + 50ms) → should NOT be alive
	outsideGrace := NewCleanupStatusWithCreatedAt(time.Now().Add(-(grace + 50*time.Millisecond)))
	if outsideGrace.IsAlive(threshold) {
		t.Error("should not be alive just outside 2*threshold boundary")
	}
}

func TestCleanupStatus_ConcurrentHeartbeatAndIsAlive(t *testing.T) {
	s := NewCleanupStatus()
	var wg sync.WaitGroup

	// 10 goroutines calling RecordHeartbeat
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				s.RecordHeartbeat()
			}
		}()
	}

	// 10 goroutines calling IsAlive
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				s.IsAlive(time.Hour)
			}
		}()
	}

	wg.Wait()

	// After all heartbeats, should be alive
	if !s.IsAlive(time.Second) {
		t.Error("should be alive after concurrent heartbeats")
	}
}

func TestCleanupStatus_StartupGracePeriod(t *testing.T) {
	// With a 1-second threshold, startup grace is 2 seconds
	s := NewCleanupStatus()
	if !s.IsAlive(time.Second) {
		t.Error("should be alive within startup grace period (2 * 1s = 2s)")
	}

	// Simulate old creation: grace expired
	old := NewCleanupStatusWithCreatedAt(time.Now().Add(-5 * time.Second))
	if old.IsAlive(time.Second) {
		t.Error("should not be alive after startup grace expires")
	}

	// After recording heartbeat on old status, it should be alive
	old.RecordHeartbeat()
	if !old.IsAlive(time.Second) {
		t.Error("should be alive after heartbeat even with old creation time")
	}
}
