package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/metrics"
)

func TestHandleMetrics_AlwaysIncludesClampViolationCounter(t *testing.T) {
	s := &Server{metrics: metrics.New()}
	rec := httptest.NewRecorder()

	s.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "# TYPE clamp_violations_total counter") {
		t.Errorf("missing clamp_violations_total TYPE line:\n%s", body)
	}
	if !strings.Contains(body, "clamp_violations_total ") {
		t.Errorf("missing clamp_violations_total value:\n%s", body)
	}
}

func TestHandleMetrics_IncludesRegistryCounters(t *testing.T) {
	reg := metrics.New()
	reg.Counter("hint_requests_total", "test help").Inc(map[string]string{"intent": "hint"})
	reg.Counter("hint_requests_total", "test help").Inc(map[string]string{"intent": "stuck"})

	s := &Server{metrics: reg}
	rec := httptest.NewRecorder()
	s.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `hint_requests_total{intent="hint"} 1`) {
		t.Errorf("missing hint counter:\n%s", body)
	}
	if !strings.Contains(body, `hint_requests_total{intent="stuck"} 1`) {
		t.Errorf("missing stuck counter:\n%s", body)
	}
}

func TestHandleMetrics_PrometheusContentType(t *testing.T) {
	s := &Server{metrics: metrics.New()}
	rec := httptest.NewRecorder()
	s.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics", nil))

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain*", got)
	}
}
