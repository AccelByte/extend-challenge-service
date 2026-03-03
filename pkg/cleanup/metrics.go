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
)

// Collectors returns all Prometheus collectors for cleanup metrics.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		cleanupRowsDeleted,
		cleanupCyclesTotal,
		cleanupErrors,
		cleanupDuration,
	}
}
