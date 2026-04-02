package cleanup

import (
	"sync/atomic"
	"time"
)

// CleanupStatus tracks cleanup goroutine liveness via atomic heartbeat timestamps.
type CleanupStatus struct {
	lastHeartbeat atomic.Int64 // Unix nanoseconds of last heartbeat
	createdAt     time.Time
}

// NewCleanupStatus creates a new CleanupStatus with no initial heartbeat.
func NewCleanupStatus() *CleanupStatus {
	return &CleanupStatus{createdAt: time.Now()}
}

// NewCleanupStatusWithCreatedAt creates a CleanupStatus with a custom creation time (for testing).
func NewCleanupStatusWithCreatedAt(t time.Time) *CleanupStatus {
	return &CleanupStatus{createdAt: t}
}

// RecordHeartbeat updates the heartbeat timestamp to now.
func (s *CleanupStatus) RecordHeartbeat() {
	s.lastHeartbeat.Store(time.Now().UnixNano())
}

// LastHeartbeatUnixSeconds returns the last heartbeat timestamp as Unix seconds (float64).
// Returns 0 if no heartbeat has been recorded.
func (s *CleanupStatus) LastHeartbeatUnixSeconds() float64 {
	nanos := s.lastHeartbeat.Load()
	if nanos == 0 {
		return 0
	}
	return float64(nanos) / 1e9
}

// IsAlive returns true if a heartbeat was recorded within the given threshold.
// If no heartbeat has been recorded yet, returns true during a startup grace period
// (2 * threshold since creation) to avoid false-positive unhealthy reports on boot.
func (s *CleanupStatus) IsAlive(threshold time.Duration) bool {
	last := s.lastHeartbeat.Load()
	if last == 0 {
		return time.Since(s.createdAt) < 2*threshold
	}
	return time.Since(time.Unix(0, last)) < threshold
}
