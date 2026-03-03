package cleanup

import (
	"testing"
	"time"
)

func TestCleanupStatus_NewIsNotAlive(t *testing.T) {
	s := NewCleanupStatus()
	if s.IsAlive(time.Hour) {
		t.Error("new status should not be alive (no heartbeat recorded)")
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
