package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Recorder struct {
	registry *prometheus.Registry

	eventsTotal *prometheus.CounterVec
	queueDepth  prometheus.Gauge
	syncTotal   *prometheus.CounterVec
	retries     prometheus.Counter
	dropped     prometheus.Counter

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

func NewRecorder() *Recorder {
	r := &Recorder{
		registry: prometheus.NewRegistry(),
		eventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "marketplace_status_controller_events_total",
				Help: "Total Argo Workflow informer events observed by the status controller.",
			},
			[]string{"event_type"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "marketplace_status_controller_queue_depth",
				Help: "Number of workflow keys currently waiting in the status controller workqueue.",
			},
		),
		syncTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "marketplace_status_controller_sync_total",
				Help: "Total workflow status synchronization attempts by result.",
			},
			[]string{"result"},
		),
		retries: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "marketplace_status_controller_sync_retries_total",
				Help: "Total rate-limited retries scheduled after transient synchronization failures.",
			},
		),
		dropped: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "marketplace_status_controller_sync_dropped_total",
				Help: "Total workflow status updates abandoned after permanent errors or retry exhaustion.",
			},
		),
		httpRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "marketplace_status_controller_http_requests_total",
				Help: "Total Publishing Status REST requests by HTTP status class.",
			},
			[]string{"status_class"},
		),
		httpDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "marketplace_status_controller_http_request_duration_seconds",
				Help:    "Publishing Status REST request latency in seconds.",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
			},
			[]string{"operation"},
		),
	}

	r.registry.MustRegister(
		r.eventsTotal,
		r.queueDepth,
		r.syncTotal,
		r.retries,
		r.dropped,
		r.httpRequests,
		r.httpDuration,
	)

	return r
}

func (r *Recorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

func (r *Recorder) Event(eventType string) {
	r.eventsTotal.WithLabelValues(eventType).Inc()
}

func (r *Recorder) SetQueueDepth(depth int) {
	r.queueDepth.Set(float64(depth))
}

func (r *Recorder) Sync(result string) {
	r.syncTotal.WithLabelValues(result).Inc()
}

func (r *Recorder) Retry() {
	r.retries.Inc()
}

func (r *Recorder) Dropped() {
	r.dropped.Inc()
}

func (r *Recorder) ObserveHTTPRequest(statusClass string, duration time.Duration) {
	r.httpRequests.WithLabelValues(statusClass).Inc()
	r.httpDuration.WithLabelValues("update_workflow_status").Observe(duration.Seconds())
}
