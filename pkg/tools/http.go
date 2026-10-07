// Package tools provides real (non-stub) implementations of loop.Invoker — the
// ACT step that calls an actual capability. HTTPInvoker calls a configured HTTP
// endpoint per capability and returns its output tagged OriginTool, so the loop
// screens it as untrusted before it re-enters reasoning.
//
// Calling an external tool is the fenced, nondeterministic exception: the core
// loop stays deterministic; the nondeterminism is quarantined here, behind the
// Invoker interface, screened on return, and bounded by a timeout and size cap.
//
// Pure standard library — net/http + encoding/json; no third-party deps.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/loop"
)

// Endpoint describes how to call one capability over HTTP.
type Endpoint struct {
	URL      string            // required
	Method   string            // default POST
	Headers  map[string]string // optional (e.g. auth)
	Timeout  time.Duration     // default 10s
	MaxBytes int64             // response cap; default 1 MiB
}

// HTTPInvoker implements loop.Invoker by dispatching each capability to its
// configured HTTP endpoint. A capability with no endpoint is an error, so the
// loop treats it as an unresolved step and escalates rather than inventing data.
type HTTPInvoker struct {
	endpoints map[string]Endpoint
	client    *http.Client
}

// NewHTTPInvoker builds an invoker over the given capability→endpoint map.
func NewHTTPInvoker(endpoints map[string]Endpoint) *HTTPInvoker {
	cp := make(map[string]Endpoint, len(endpoints))
	for k, v := range endpoints {
		cp[k] = v
	}
	return &HTTPInvoker{endpoints: cp, client: &http.Client{}}
}

// Names returns the capabilities this invoker can serve (sorted order not guaranteed).
func (h *HTTPInvoker) Names() []string {
	out := make([]string, 0, len(h.endpoints))
	for k := range h.endpoints {
		out = append(out, k)
	}
	return out
}

// Invoke calls the capability's endpoint with the given input and returns its
// output tagged OriginTool (untrusted). Bounded by timeout and a response cap.
func (h *HTTPInvoker) Invoke(ctx context.Context, capName string, input []byte) (loop.Output, error) {
	ep, ok := h.endpoints[capName]
	if !ok {
		return loop.Output{}, fmt.Errorf("no tool endpoint configured for capability %q", capName)
	}
	method := ep.Method
	if method == "" {
		method = http.MethodPost
	}
	timeout := ep.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxBytes := ep.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, method, ep.URL, bytes.NewReader(input))
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: build request: %w", capName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: %w", capName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: read response: %w", capName, err)
	}
	if resp.StatusCode >= 400 {
		return loop.Output{}, fmt.Errorf("tool %q: endpoint returned status %d", capName, resp.StatusCode)
	}
	return loop.Output{Data: body, Origin: guard.OriginTool}, nil
}
