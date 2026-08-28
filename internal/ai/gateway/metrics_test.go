package gateway

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func getCounterValue(counter prometheus.Counter) float64 {
	var m dto.Metric
	if err := counter.Write(&m); err != nil {
		return 0
	}
	return m.GetCounter().GetValue()
}

func getCounterVecValue(cv *prometheus.CounterVec, labels ...string) float64 {
	counter, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	return getCounterValue(counter)
}

func getGaugeVecValue(gv *prometheus.GaugeVec, labels ...string) float64 {
	gauge, err := gv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	var m dto.Metric
	if err := gauge.Write(&m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

// We use a fresh registry for each test to avoid cross-test pollution.
// Since promauto registers to the default registry, we test behavior directly.

func TestMetrics_ObserveRequest(t *testing.T) {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_requests_total",
		}, []string{"model", "provider", "protocol", "status"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "test_request_duration",
			Buckets: []float64{0.1, 0.5, 1},
		}, []string{"model", "provider", "protocol"}),
	}

	m.ObserveRequest("gpt-4o", "openai", "openai", "ok", 250*time.Millisecond)
	m.ObserveRequest("gpt-4o", "openai", "openai", "ok", 750*time.Millisecond)
	m.ObserveRequest("gpt-4o", "openai", "openai", "error", 100*time.Millisecond)

	okCount := getCounterVecValue(m.RequestsTotal, "gpt-4o", "openai", "openai", "ok")
	if okCount != 2 {
		t.Errorf("expected 2 ok requests, got %f", okCount)
	}

	errCount := getCounterVecValue(m.RequestsTotal, "gpt-4o", "openai", "openai", "error")
	if errCount != 1 {
		t.Errorf("expected 1 error request, got %f", errCount)
	}
}

func TestMetrics_ObserveTokens(t *testing.T) {
	m := &Metrics{
		TokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_tokens_total",
		}, []string{"model", "provider", "type"}),
	}

	m.ObserveTokens("gpt-4o", "openai", 100, 50)
	m.ObserveTokens("gpt-4o", "openai", 200, 0)

	prompt := getCounterVecValue(m.TokensTotal, "gpt-4o", "openai", "prompt")
	if prompt != 300 {
		t.Errorf("expected 300 prompt tokens, got %f", prompt)
	}

	completion := getCounterVecValue(m.TokensTotal, "gpt-4o", "openai", "completion")
	if completion != 50 {
		t.Errorf("expected 50 completion tokens, got %f", completion)
	}
}

func TestMetrics_ObserveError(t *testing.T) {
	m := &Metrics{
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_errors_total",
		}, []string{"provider", "status_code"}),
	}

	m.ObserveError("openai", "500")
	m.ObserveError("openai", "500")
	m.ObserveError("openai", "429")

	e500 := getCounterVecValue(m.ErrorsTotal, "openai", "500")
	if e500 != 2 {
		t.Errorf("expected 2 500 errors, got %f", e500)
	}

	e429 := getCounterVecValue(m.ErrorsTotal, "openai", "429")
	if e429 != 1 {
		t.Errorf("expected 1 429 error, got %f", e429)
	}
}

func TestMetrics_CacheHitsMisses(t *testing.T) {
	m := &Metrics{
		CacheHitsTotal:   prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_hits"}),
		CacheMissesTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_cache_misses"}),
	}

	m.ObserveCacheHit()
	m.ObserveCacheHit()
	m.ObserveCacheMiss()

	hits := getCounterValue(m.CacheHitsTotal)
	if hits != 2 {
		t.Errorf("expected 2 cache hits, got %f", hits)
	}

	misses := getCounterValue(m.CacheMissesTotal)
	if misses != 1 {
		t.Errorf("expected 1 cache miss, got %f", misses)
	}
}

func TestMetrics_ObserveRetry(t *testing.T) {
	m := &Metrics{
		RetriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_retries_total",
		}, []string{"provider", "reason"}),
	}

	m.ObserveRetry("openai", "http_500")
	m.ObserveRetry("openai", "http_429")
	m.ObserveRetry("openai", "http_500")

	r500 := getCounterVecValue(m.RetriesTotal, "openai", "http_500")
	if r500 != 2 {
		t.Errorf("expected 2 retries for 500, got %f", r500)
	}

	r429 := getCounterVecValue(m.RetriesTotal, "openai", "http_429")
	if r429 != 1 {
		t.Errorf("expected 1 retry for 429, got %f", r429)
	}
}

func TestMetrics_ObserveRateLimit(t *testing.T) {
	m := &Metrics{
		RateLimitTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_rate_limit_total",
		}, []string{"type"}),
	}

	m.ObserveRateLimit("rpm")
	m.ObserveRateLimit("concurrency")
	m.ObserveRateLimit("rpm")

	rpm := getCounterVecValue(m.RateLimitTotal, "rpm")
	if rpm != 2 {
		t.Errorf("expected 2 RPM rate limits, got %f", rpm)
	}

	conc := getCounterVecValue(m.RateLimitTotal, "concurrency")
	if conc != 1 {
		t.Errorf("expected 1 concurrency rate limit, got %f", conc)
	}
}

func TestMetrics_InFlightRequests(t *testing.T) {
	m := &Metrics{
		InFlightRequests: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "test_in_flight",
		}, []string{"protocol"}),
	}

	m.InFlightRequests.WithLabelValues("openai").Inc()
	m.InFlightRequests.WithLabelValues("openai").Inc()

	val := getGaugeVecValue(m.InFlightRequests, "openai")
	if val != 2 {
		t.Errorf("expected 2 in-flight, got %f", val)
	}

	m.InFlightRequests.WithLabelValues("openai").Dec()
	val = getGaugeVecValue(m.InFlightRequests, "openai")
	if val != 1 {
		t.Errorf("expected 1 in-flight after dec, got %f", val)
	}
}

func TestMetrics_ZeroTokensNotRecorded(t *testing.T) {
	m := &Metrics{
		TokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_tokens_zero",
		}, []string{"model", "provider", "type"}),
	}

	m.ObserveTokens("gpt-4o", "openai", 0, 0)

	// With zero tokens, nothing should be recorded
	prompt := getCounterVecValue(m.TokensTotal, "gpt-4o", "openai", "prompt")
	if prompt != 0 {
		t.Errorf("expected 0 prompt tokens, got %f", prompt)
	}
}
