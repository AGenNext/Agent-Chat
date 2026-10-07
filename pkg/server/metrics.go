// Metrics: process-level counters exposed in Prometheus text exposition format
// at GET /metrics. Pure standard library — sync/atomic + net/http, no client
// library and no third-party deps (the headless, zero-dependency promise holds).
package server

import (
	"net/http"
	"strconv"
	"sync/atomic"
)

// metrics holds the counters the server exports. All access is via atomics, so
// it is safe under concurrent requests.
type metrics struct {
	requests  atomic.Int64 // total HTTP requests served
	turns     atomic.Int64 // chat turns run to completion
	escalated atomic.Int64 // turns that escalated to a human (the recovery path)
	errors    atomic.Int64 // turns that errored
	inflight  atomic.Int64 // in-flight HTTP requests
}

type metricRow struct {
	name, help, typ string
	val             int64
}

// write renders the counters in Prometheus text exposition format (v0.0.4).
func (m *metrics) write(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	rows := []metricRow{
		{"agennext_http_requests_total", "Total HTTP requests served.", "counter", m.requests.Load()},
		{"agennext_http_in_flight", "In-flight HTTP requests.", "gauge", m.inflight.Load()},
		{"agennext_chat_turns_total", "Total chat turns run to completion.", "counter", m.turns.Load()},
		{"agennext_chat_escalated_total", "Chat turns that escalated to a human.", "counter", m.escalated.Load()},
		{"agennext_chat_errors_total", "Chat turns that errored.", "counter", m.errors.Load()},
	}
	for _, r := range rows {
		_, _ = w.Write([]byte("# HELP " + r.name + " " + r.help + "\n"))
		_, _ = w.Write([]byte("# TYPE " + r.name + " " + r.typ + "\n"))
		_, _ = w.Write([]byte(r.name + " " + strconv.FormatInt(r.val, 10) + "\n"))
	}
}

// instrument counts every request and tracks in-flight depth.
func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.metrics.requests.Add(1)
		s.metrics.inflight.Add(1)
		defer s.metrics.inflight.Add(-1)
		next.ServeHTTP(w, r)
	})
}
