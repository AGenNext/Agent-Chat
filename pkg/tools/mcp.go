// MCPInvoker is a second loop.Invoker binding: it calls a Model Context
// Protocol (MCP) tool over Streamable HTTP, speaking JSON-RPC 2.0. Each
// capability maps to one MCP server + tool name; a tools/call request is issued
// and the text content of the result is returned tagged OriginTool, so the loop
// screens it as untrusted before it re-enters reasoning — exactly as with
// HTTPInvoker. MCP is the interoperable tool surface at the edge.
//
// Only the tools/call method is used; this is a client of an already-known tool,
// not a discovery/handshake session. A JSON-RPC error, an MCP tool error
// (isError), or an HTTP failure all surface as a Go error, so the loop treats
// the step as unresolved and escalates rather than inventing data.
//
// Pure standard library — net/http + encoding/json; no third-party deps.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agennext/agent-chat/pkg/guard"
	"github.com/agennext/agent-chat/pkg/loop"
)

// MCPServer describes how to reach one MCP tool for a capability.
type MCPServer struct {
	URL      string            // required: the MCP Streamable HTTP endpoint
	Tool     string            // required: the tool name to call
	Headers  map[string]string // optional (e.g. Authorization)
	Timeout  time.Duration     // default 10s
	MaxBytes int64             // response cap; default 1 MiB
}

// MCPInvoker implements loop.Invoker by dispatching each capability to its
// configured MCP tool via JSON-RPC 2.0 tools/call.
type MCPInvoker struct {
	servers map[string]MCPServer
	client  *http.Client
}

// NewMCPInvoker builds an invoker over the given capability→server map.
func NewMCPInvoker(servers map[string]MCPServer) *MCPInvoker {
	cp := make(map[string]MCPServer, len(servers))
	for k, v := range servers {
		cp[k] = v
	}
	return &MCPInvoker{servers: cp, client: &http.Client{}}
}

// Names returns the capabilities this invoker can serve.
func (m *MCPInvoker) Names() []string {
	out := make([]string, 0, len(m.servers))
	for k := range m.servers {
		out = append(out, k)
	}
	return out
}

// rpcRequest is a JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      int        `json:"id"`
	Method  string     `json:"method"`
	Params  callParams `json:"params"`
}

// callParams are the params for the tools/call method.
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// rpcResponse is a JSON-RPC 2.0 response envelope.
type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  *toolResult `json:"result"`
	Error   *rpcError   `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolResult is the result of a tools/call: a list of content blocks and an
// error flag (a tool-level failure, distinct from a transport/JSON-RPC error).
type toolResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Invoke calls the capability's MCP tool and returns the text of its result,
// tagged OriginTool (untrusted). Bounded by timeout and a response cap.
func (m *MCPInvoker) Invoke(ctx context.Context, capName string, input []byte) (loop.Output, error) {
	srv, ok := m.servers[capName]
	if !ok {
		return loop.Output{}, fmt.Errorf("no MCP server configured for capability %q", capName)
	}
	timeout := srv.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxBytes := srv.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  callParams{Name: srv.Tool, Arguments: toArguments(input)},
	})
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: encode request: %w", capName, err)
	}

	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: build request: %w", capName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// MCP Streamable HTTP servers may reply as JSON or SSE; accept both.
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range srv.Headers {
		req.Header.Set(k, v)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: %w", capName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: read response: %w", capName, err)
	}
	if resp.StatusCode >= 400 {
		return loop.Output{}, fmt.Errorf("tool %q: MCP endpoint returned status %d", capName, resp.StatusCode)
	}

	var rpc rpcResponse
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return loop.Output{}, fmt.Errorf("tool %q: decode JSON-RPC response: %w", capName, err)
	}
	if rpc.Error != nil {
		return loop.Output{}, fmt.Errorf("tool %q: JSON-RPC error %d: %s", capName, rpc.Error.Code, rpc.Error.Message)
	}
	if rpc.Result == nil {
		return loop.Output{}, fmt.Errorf("tool %q: MCP response had no result", capName)
	}
	if rpc.Result.IsError {
		return loop.Output{}, fmt.Errorf("tool %q: MCP tool reported an error: %s", capName, textOf(rpc.Result.Content))
	}
	return loop.Output{Data: []byte(textOf(rpc.Result.Content)), Origin: guard.OriginTool}, nil
}

// toArguments turns the loop's raw input into MCP tool arguments. Valid JSON is
// passed through as the arguments object; anything else is wrapped as
// {"input": "<text>"} so a plain-text question is still a well-formed call.
func toArguments(input []byte) json.RawMessage {
	if len(bytes.TrimSpace(input)) == 0 {
		return json.RawMessage("{}")
	}
	if json.Valid(input) {
		return json.RawMessage(input)
	}
	wrapped, _ := json.Marshal(map[string]string{"input": string(input)})
	return wrapped
}

// textOf concatenates the text of the text content blocks with newlines.
func textOf(blocks []contentBlock) string {
	var b bytes.Buffer
	for _, c := range blocks {
		if c.Type == "text" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(c.Text)
		}
	}
	return b.String()
}
