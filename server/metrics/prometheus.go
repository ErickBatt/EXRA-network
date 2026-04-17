package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ActiveNodes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "Exra_nodes_active",
		Help: "The total number of active WebSocket nodes currently connected",
	})

	TotalSessions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_sessions_total",
		Help: "The total number of proxy sessions established",
	})

	TotalBytesProxied = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_bytes_proxied_total",
		Help: "The total number of bytes tunneled through the proxy",
	})

	// --- Compute Marketplace ---
	ComputeTasksSubmitted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_compute_tasks_submitted_total",
		Help: "Total number of compute tasks submitted by buyers",
	})

	ComputeTasksCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_compute_tasks_completed_total",
		Help: "Total number of compute tasks successfully completed by workers",
	})

	ComputeTasksFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_compute_tasks_failed_total",
		Help: "Total number of compute tasks that failed or timed out",
	})

	ComputeTaskDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "Exra_compute_task_duration_seconds",
		Help:    "Time taken to complete compute tasks",
		Buckets: []float64{1, 5, 10, 30, 60, 300, 600},
	})

	// --- Hub & WebSocket ---
	HubMessagesOut = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_hub_messages_out_total",
		Help: "Total messages sent from Hub out to nodes via WebSocket",
	})

	HubMessagesIn = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_hub_messages_in_total",
		Help: "Total messages received by Hub from nodes via WebSocket",
	})

	RedisPubs = promauto.NewCounter(prometheus.CounterOpts{
		Name: "Exra_redis_pub_total",
		Help: "Total messages published to Redis Pub/Sub for scale-out",
	})
)

