package cleanup

import (
	"sync/atomic"
	"time"
)

// CleanupStatus tracks cleanup goroutine liveness via atomic heartbeat timestamps.
type CleanupStatus struct {
	lastHeartbeat atomic.Int64 // Unix nanoseconds of last heartbeat
}

// NewCleanupStatus creates a new CleanupStatus with no initial heartbeat.
func NewCleanupStatus() *CleanupStatus {
	return &CleanupStatus{}
}

// RecordHeartbeat updates the heartbeat timestamp to now.
func (s *CleanupStatus) RecordHeartbeat() {
	s.lastHeartbeat.Store(time.Now().UnixNano())
}

// IsAlive returns true if a heartbeat was recorded within the given threshold.
// Returns false if no heartbeat has ever been recorded.
func (s *CleanupStatus) IsAlive(threshold time.Duration) bool {
	last := s.lastHeartbeat.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < threshold
}
