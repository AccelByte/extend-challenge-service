package cleanup

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollectors_ReturnsExactly5(t *testing.T) {
	collectors := Collectors()
	if len(collectors) != 5 {
		t.Errorf("expected 5 collectors, got %d", len(collectors))
	}
}

func TestNewHeartbeatGauge_Registerable(t *testing.T) {
	status := NewCleanupStatus()
	gauge := NewHeartbeatGauge(status)

	reg := prometheus.NewRegistry()
	if err := reg.Register(gauge); err != nil {
		t.Fatalf("failed to register heartbeat gauge: %v", err)
	}

	// Before heartbeat: value should be 0
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	found := false
	for _, f := range families {
		if f.GetName() == "challenge_cleanup_last_heartbeat_seconds" {
			found = true
			val := f.GetMetric()[0].GetGauge().GetValue()
			if val != 0 {
				t.Errorf("expected 0 before heartbeat, got %f", val)
			}
		}
	}
	if !found {
		t.Error("heartbeat gauge metric not found")
	}

	// After heartbeat: value should be > 0
	status.RecordHeartbeat()
	families, err = reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather after heartbeat: %v", err)
	}
	for _, f := range families {
		if f.GetName() == "challenge_cleanup_last_heartbeat_seconds" {
			val := f.GetMetric()[0].GetGauge().GetValue()
			if val <= 0 {
				t.Errorf("expected > 0 after heartbeat, got %f", val)
			}
		}
	}
}

func TestCollectors_RegisterableWithPrometheus(t *testing.T) {
	reg := prometheus.NewRegistry()
	for _, c := range Collectors() {
		if err := reg.Register(c); err != nil {
			t.Errorf("failed to register collector: %v", err)
		}
	}

	// Gather and verify metric families exist
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expectedNames := map[string]bool{
		"challenge_cleanup_rows_deleted_total": false,
		"challenge_cleanup_cycles_total":       false,
		"challenge_cleanup_errors_total":       false,
		"challenge_cleanup_duration_seconds":   false,
		"challenge_cleanup_panics_total":       false,
	}

	for _, f := range families {
		if _, ok := expectedNames[f.GetName()]; ok {
			expectedNames[f.GetName()] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("metric %q not found in gathered families", name)
		}
	}
}
