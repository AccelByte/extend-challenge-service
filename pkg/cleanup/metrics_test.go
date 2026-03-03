package cleanup

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollectors_ReturnsExactly4(t *testing.T) {
	collectors := Collectors()
	if len(collectors) != 4 {
		t.Errorf("expected 4 collectors, got %d", len(collectors))
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
