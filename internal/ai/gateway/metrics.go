package gateway

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics collects Prometheus metrics for the gateway.
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	TokensTotal      *prometheus.CounterVec
	ErrorsTotal      *prometheus.CounterVec
	CacheHitsTotal   prometheus.Counter
	CacheMissesTotal prometheus.Counter
	RetriesTotal     *prometheus.CounterVec
	RateLimitTotal   *prometheus.CounterVec
	InFlightRequests *prometheus.GaugeVec
	Registry         *prometheus.Registry
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	// Register default Go and process collectors
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	factory := promauto.With(reg)

	m := &Metrics{
		Registry: reg,

		RequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "requests_total",
			Help:      "Total number of gateway requests by model, provider, protocol, and status.",
		}, []string{"model", "provider", "protocol", "status"}),

		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds by model, provider, and protocol.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"model", "provider", "protocol"}),

		TokensTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "tokens_total",
			Help:      "Total tokens processed by model, provider, and type (prompt/completion).",
		}, []string{"model", "provider", "type"}),

		ErrorsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "errors_total",
			Help:      "Total upstream errors by provider and status code.",
		}, []string{"provider", "status_code"}),

		CacheHitsTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "cache_hits_total",
			Help:      "Total number of response cache hits.",
		}),

		CacheMissesTotal: factory.NewCounter(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "cache_misses_total",
			Help:      "Total number of response cache misses.",
		}),

		RetriesTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "retries_total",
			Help:      "Total retry attempts by provider and reason.",
		}, []string{"provider", "reason"}),

		RateLimitTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "rate_limit_total",
			Help:      "Total rate limit rejections by type (rpm/concurrency).",
		}, []string{"type"}),

		InFlightRequests: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "freerouter",
			Subsystem: "gateway",
			Name:      "in_flight_requests",
			Help:      "Current number of in-flight requests by protocol.",
		}, []string{"protocol"}),
	}

	return m
}

// ObserveRequest records metrics for a completed request.
func (m *Metrics) ObserveRequest(model, providerID, protocol, status string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(model, providerID, protocol, status).Inc()
	m.RequestDuration.WithLabelValues(model, providerID, protocol).Observe(duration.Seconds())
}

// ObserveTokens records token usage.
func (m *Metrics) ObserveTokens(model, providerID string, promptTokens, completionTokens int) {
	if promptTokens > 0 {
		m.TokensTotal.WithLabelValues(model, providerID, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.TokensTotal.WithLabelValues(model, providerID, "completion").Add(float64(completionTokens))
	}
}

// ObserveError records an upstream error.
func (m *Metrics) ObserveError(providerID, statusCode string) {
	m.ErrorsTotal.WithLabelValues(providerID, statusCode).Inc()
}

// ObserveCacheHit records a cache hit.
func (m *Metrics) ObserveCacheHit() {
	m.CacheHitsTotal.Inc()
}

// ObserveCacheMiss records a cache miss.
func (m *Metrics) ObserveCacheMiss() {
	m.CacheMissesTotal.Inc()
}

// ObserveRetry records a retry attempt.
func (m *Metrics) ObserveRetry(providerID, reason string) {
	m.RetriesTotal.WithLabelValues(providerID, reason).Inc()
}

// ObserveRateLimit records a rate limit rejection.
func (m *Metrics) ObserveRateLimit(limitType string) {
	m.RateLimitTotal.WithLabelValues(limitType).Inc()
}
