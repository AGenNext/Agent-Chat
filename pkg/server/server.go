// Package server is the headless HTTP transport — the edge surface that wraps
// the core. It terminates HTTP, admits requests at the gate, and runs the chat
// core, returning an inspectable result. Pure standard library (no third-party
// deps); a gRPC transport (proto Chat service) is the gated alternative.
//
// The server is the edge: it imports the gate and the core, but the cores never
// import it. Scope is derived at the gate, never taken from the request body.
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agennext/agent-chat/pkg/chat"
	"github.com/agennext/agent-chat/pkg/edge"
	"github.com/agennext/agent-chat/pkg/event"
	"github.com/agennext/agent-chat/pkg/loop"
)

// Server wires the entry gate to the chat core over HTTP.
type Server struct {
	gate    *edge.Gate
	core    *chat.Core
	metrics *metrics
}

// New builds a Server from an entry gate and a chat core.
func New(g *edge.Gate, c *chat.Core) *Server {
	return &Server{gate: g, core: c, metrics: &metrics{}}
}

// Handler returns the HTTP routes (Go 1.22+ method patterns), wrapped with
// request instrumentation. GET /metrics exposes Prometheus-format counters.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.health)
	mux.HandleFunc("GET /metrics", s.metrics.write)
	mux.HandleFunc("POST /v1/chat", s.chat)
	return s.instrument(mux)
}

// ChatRequest is the JSON body for POST /v1/chat. Scope is intentionally absent:
// authority is derived at the gate from the principal's grants.
type ChatRequest struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Type       string `json:"type"`
	Session    string `json:"session"`
	Tenant     string `json:"tenant"`
	Principal  string `json:"principal"`
	Capability string `json:"capability"`
	Message    string `json:"message"`
}

// ChatResponse mirrors loop.Result for the wire.
type ChatResponse struct {
	Session    string            `json:"session"`
	Answer     string            `json:"answer"`
	Iterations int               `json:"iterations"`
	StoppedBy  string            `json:"stopped_by"`
	Escalated  bool              `json:"escalated"`
	Trace      []loop.TraceEntry `json:"trace"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Type == "" {
		req.Type = "chat.message.v1"
	}
	if req.Source == "" {
		req.Source = "channel/http"
	}
	ev := event.New(req.ID, req.Source, req.Type, req.Session, req.Tenant, req.Principal, req.Capability, []byte(req.Message))

	admitted, err := s.gate.Admit(r.Context(), ev)
	if err != nil {
		// Map to a safe status + generic reason — never echo the raw error
		// (must not bleed internal detail).
		code, reason := gateStatus(err)
		writeErr(w, code, reason)
		return
	}

	res, err := s.core.Run(r.Context(), admitted)
	if err != nil {
		s.metrics.errors.Add(1)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.metrics.turns.Add(1)
	if res.Escalated {
		s.metrics.escalated.Add(1)
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Session:    res.SessionID,
		Answer:     res.Answer,
		Iterations: res.Iterations,
		StoppedBy:  res.StoppedBy,
		Escalated:  res.Escalated,
		Trace:      res.Trace,
	})
}

// gateStatus maps a gate admission error to an HTTP status and a safe, generic
// reason that bleeds no internal detail.
func gateStatus(err error) (int, string) {
	switch {
	case errors.Is(err, edge.ErrUnauthenticated):
		return http.StatusUnauthorized, "unauthenticated"
	case errors.Is(err, edge.ErrUnauthorized), errors.Is(err, edge.ErrOutOfScope):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, edge.ErrBlocked):
		return http.StatusUnprocessableEntity, "request blocked"
	case errors.Is(err, event.ErrInvalidEvent):
		return http.StatusBadRequest, "invalid request"
	default:
		return http.StatusBadRequest, "bad request"
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, reason string) {
	writeJSON(w, code, map[string]string{"error": reason})
}
