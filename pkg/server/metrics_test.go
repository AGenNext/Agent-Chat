package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsEndpoint verifies GET /metrics serves Prometheus text format and
// that the request instrumentation counts requests. It uses no gate/core since
// /metrics and /healthz never touch them.
func TestMetricsEndpoint(t *testing.T) {
	t.Parallel()
	h := New(nil, nil).Handler()

	// Make one request so the counter is non-zero.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics code = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# TYPE agennext_http_requests_total counter",
		"# TYPE agennext_http_in_flight gauge",
		"agennext_chat_turns_total",
		"agennext_chat_escalated_total",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in /metrics output:\n%s", want, body)
		}
	}
	// requests_total should be >= 2 (healthz + metrics, both instrumented).
	if !strings.Contains(body, "agennext_http_requests_total 2") &&
		!strings.Contains(body, "agennext_http_requests_total 3") {
		t.Fatalf("unexpected request count in:\n%s", body)
	}
}
