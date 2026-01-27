// Package observability provides metrics, tracing, and logging for the V2 scheduler.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// MetricsNamespace is the namespace for all scheduler metrics.
	MetricsNamespace = "crawler"

	// MetricsSubsystem is the subsystem for scheduler metrics.
	MetricsSubsystem = "scheduler"
)

// Metrics holds all Prometheus metrics for the V2 scheduler.
type Metrics struct {
	// Job metrics
	JobsScheduledTotal   *prometheus.CounterVec
	JobsExecutedTotal    *prometheus.CounterVec
	JobDurationSeconds   *prometheus.HistogramVec
	JobsCurrentlyRunning prometheus.Gauge

	// Worker pool metrics
	WorkerPoolSize  prometheus.Gauge
	WorkersBusy     prometheus.Gauge
	WorkersIdle     prometheus.Gauge
	WorkerTasksWait *prometheus.HistogramVec

	// Queue metrics
	QueueDepth      *prometheus.GaugeVec
	QueueEnqueued   *prometheus.CounterVec
	QueueDequeued   *prometheus.CounterVec
	QueueProcessLag *prometheus.HistogramVec

	// Trigger metrics
	TriggersFired   *prometheus.CounterVec
	TriggersMatched *prometheus.CounterVec

	// Circuit breaker metrics
	CircuitBreakerState   *prometheus.GaugeVec
	CircuitBreakerTrips   *prometheus.CounterVec
	CircuitBreakerSuccess *prometheus.CounterVec
	CircuitBreakerFailure *prometheus.CounterVec

	// Leader election metrics
	LeaderElectionAttempts prometheus.Counter
	LeaderElectionWins     prometheus.Counter
	LeaderElectionLosses   prometheus.Counter
	IsLeader               prometheus.Gauge
}

// NewMetrics creates and registers all V2 scheduler metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	factory := promauto.With(reg)
	m := &Metrics{}

	m.initJobMetrics(factory)
	m.initWorkerMetrics(factory)
	m.initQueueMetrics(factory)
	m.initTriggerMetrics(factory)
	m.initCircuitBreakerMetrics(factory)
	m.initLeaderMetrics(factory)

	return m
}

// initJobMetrics initializes job-related metrics.
func (m *Metrics) initJobMetrics(factory promauto.Factory) {
	m.JobsScheduledTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "jobs_scheduled_total",
			Help:      "Total number of jobs scheduled",
		},
		[]string{"schedule_type", "priority"},
	)

	m.JobsExecutedTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "jobs_executed_total",
			Help:      "Total number of jobs executed",
		},
		[]string{"status", "source_id"},
	)

	m.JobDurationSeconds = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "job_duration_seconds",
			Help:      "Duration of job execution in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 15), // 0.1s to ~55min
		},
		[]string{"source_id"},
	)

	m.JobsCurrentlyRunning = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "jobs_currently_running",
			Help:      "Number of jobs currently running",
		},
	)
}

// initWorkerMetrics initializes worker pool metrics.
func (m *Metrics) initWorkerMetrics(factory promauto.Factory) {
	m.WorkerPoolSize = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "worker_pool_size",
			Help:      "Total size of the worker pool",
		},
	)

	m.WorkersBusy = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "workers_busy",
			Help:      "Number of busy workers",
		},
	)

	m.WorkersIdle = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "workers_idle",
			Help:      "Number of idle workers",
		},
	)

	m.WorkerTasksWait = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "worker_tasks_wait_seconds",
			Help:      "Time tasks spend waiting for a worker",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 12), // 10ms to ~41s
		},
		[]string{"priority"},
	)
}

// initQueueMetrics initializes queue metrics.
func (m *Metrics) initQueueMetrics(factory promauto.Factory) {
	m.QueueDepth = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "queue_depth",
			Help:      "Current depth of the job queue",
		},
		[]string{"priority"},
	)

	m.QueueEnqueued = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "queue_enqueued_total",
			Help:      "Total number of jobs enqueued",
		},
		[]string{"priority"},
	)

	m.QueueDequeued = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "queue_dequeued_total",
			Help:      "Total number of jobs dequeued",
		},
		[]string{"priority"},
	)

	m.QueueProcessLag = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "queue_process_lag_seconds",
			Help:      "Lag between job scheduled time and actual processing",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12), // 100ms to ~7min
		},
		[]string{"priority"},
	)
}

// initTriggerMetrics initializes trigger metrics.
func (m *Metrics) initTriggerMetrics(factory promauto.Factory) {
	m.TriggersFired = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "triggers_fired_total",
			Help:      "Total number of triggers fired",
		},
		[]string{"type"}, // webhook, pubsub
	)

	m.TriggersMatched = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "triggers_matched_total",
			Help:      "Total number of triggers that matched jobs",
		},
		[]string{"type"},
	)
}

// initCircuitBreakerMetrics initializes circuit breaker metrics.
func (m *Metrics) initCircuitBreakerMetrics(factory promauto.Factory) {
	m.CircuitBreakerState = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "circuit_breaker_state",
			Help:      "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
		},
		[]string{"domain"},
	)

	m.CircuitBreakerTrips = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "circuit_breaker_trips_total",
			Help:      "Total number of circuit breaker trips",
		},
		[]string{"domain"},
	)

	m.CircuitBreakerSuccess = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "circuit_breaker_success_total",
			Help:      "Total number of successful requests through circuit breaker",
		},
		[]string{"domain"},
	)

	m.CircuitBreakerFailure = factory.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "circuit_breaker_failure_total",
			Help:      "Total number of failed requests through circuit breaker",
		},
		[]string{"domain"},
	)
}

// initLeaderMetrics initializes leader election metrics.
func (m *Metrics) initLeaderMetrics(factory promauto.Factory) {
	m.LeaderElectionAttempts = factory.NewCounter(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "leader_election_attempts_total",
			Help:      "Total number of leader election attempts",
		},
	)

	m.LeaderElectionWins = factory.NewCounter(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "leader_election_wins_total",
			Help:      "Total number of leader election wins",
		},
	)

	m.LeaderElectionLosses = factory.NewCounter(
		prometheus.CounterOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "leader_election_losses_total",
			Help:      "Total number of leader election losses",
		},
	)

	m.IsLeader = factory.NewGauge(
		prometheus.GaugeOpts{
			Namespace: MetricsNamespace,
			Subsystem: MetricsSubsystem,
			Name:      "is_leader",
			Help:      "Whether this instance is the current leader (1=yes, 0=no)",
		},
	)
}

// RecordJobScheduled records a job being scheduled.
func (m *Metrics) RecordJobScheduled(scheduleType, priority string) {
	m.JobsScheduledTotal.WithLabelValues(scheduleType, priority).Inc()
}

// RecordJobExecuted records a job execution completion.
func (m *Metrics) RecordJobExecuted(status, sourceID string, durationSeconds float64) {
	m.JobsExecutedTotal.WithLabelValues(status, sourceID).Inc()
	m.JobDurationSeconds.WithLabelValues(sourceID).Observe(durationSeconds)
}

// RecordJobStarted increments the running job count.
func (m *Metrics) RecordJobStarted() {
	m.JobsCurrentlyRunning.Inc()
}

// RecordJobFinished decrements the running job count.
func (m *Metrics) RecordJobFinished() {
	m.JobsCurrentlyRunning.Dec()
}

// SetWorkerPoolMetrics sets the worker pool metrics.
func (m *Metrics) SetWorkerPoolMetrics(total, busy, idle int) {
	m.WorkerPoolSize.Set(float64(total))
	m.WorkersBusy.Set(float64(busy))
	m.WorkersIdle.Set(float64(idle))
}

// RecordQueueDepth records the current queue depth.
func (m *Metrics) RecordQueueDepth(priority string, depth int) {
	m.QueueDepth.WithLabelValues(priority).Set(float64(depth))
}

// RecordEnqueue records a job enqueue operation.
func (m *Metrics) RecordEnqueue(priority string) {
	m.QueueEnqueued.WithLabelValues(priority).Inc()
}

// RecordDequeue records a job dequeue operation.
func (m *Metrics) RecordDequeue(priority string) {
	m.QueueDequeued.WithLabelValues(priority).Inc()
}

// RecordTriggerFired records a trigger being fired.
func (m *Metrics) RecordTriggerFired(triggerType string, matched bool) {
	m.TriggersFired.WithLabelValues(triggerType).Inc()
	if matched {
		m.TriggersMatched.WithLabelValues(triggerType).Inc()
	}
}

// SetCircuitBreakerState sets the circuit breaker state for a domain.
func (m *Metrics) SetCircuitBreakerState(domain string, state int) {
	m.CircuitBreakerState.WithLabelValues(domain).Set(float64(state))
}

// RecordCircuitBreakerTrip records a circuit breaker trip.
func (m *Metrics) RecordCircuitBreakerTrip(domain string) {
	m.CircuitBreakerTrips.WithLabelValues(domain).Inc()
}

// RecordCircuitBreakerResult records a circuit breaker operation result.
func (m *Metrics) RecordCircuitBreakerResult(domain string, success bool) {
	if success {
		m.CircuitBreakerSuccess.WithLabelValues(domain).Inc()
	} else {
		m.CircuitBreakerFailure.WithLabelValues(domain).Inc()
	}
}

// RecordLeaderElectionAttempt records a leader election attempt.
func (m *Metrics) RecordLeaderElectionAttempt(won bool) {
	m.LeaderElectionAttempts.Inc()
	if won {
		m.LeaderElectionWins.Inc()
	} else {
		m.LeaderElectionLosses.Inc()
	}
}

// SetIsLeader sets whether this instance is the leader.
func (m *Metrics) SetIsLeader(isLeader bool) {
	if isLeader {
		m.IsLeader.Set(1)
	} else {
		m.IsLeader.Set(0)
	}
}
