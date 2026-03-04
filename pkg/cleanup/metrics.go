package cleanup

import "github.com/prometheus/client_golang/prometheus"

var (
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

	cleanupPanics = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "challenge_cleanup_panics_total",
		Help: "Total number of panic-recovery restarts in the cleanup goroutine.",
	})
)

// Collectors returns all Prometheus collectors for cleanup metrics.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		cleanupRowsDeleted,
		cleanupCyclesTotal,
		cleanupErrors,
		cleanupDuration,
		cleanupPanics,
	}
}

// NewHeartbeatGauge creates a GaugeFunc that reports the last cleanup heartbeat
// as a Unix timestamp in seconds. This enables staleness detection via PromQL:
//
//	time() - challenge_cleanup_last_heartbeat_seconds > threshold
func NewHeartbeatGauge(status *CleanupStatus) prometheus.GaugeFunc {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "challenge_cleanup_last_heartbeat_seconds",
		Help: "Unix timestamp (seconds) of the last cleanup cycle heartbeat.",
	}, status.LastHeartbeatUnixSeconds)
}
